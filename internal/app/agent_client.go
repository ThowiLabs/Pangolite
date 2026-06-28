package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

type AgentClientConfig struct {
	ServerURL      string
	FallbackURL    string
	ConfigPath     string
	AgentID        string
	Token          string
	PollInterval   time.Duration
	RequestTimeout time.Duration
}

func (c AgentClientConfig) Validate() error {
	if strings.TrimSpace(c.ServerURL) == "" {
		return errors.New("server-url requerido")
	}
	if strings.TrimSpace(c.AgentID) == "" {
		return errors.New("agent-id requerido")
	}
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("token requerido")
	}
	if err := validateAgentBaseURL(c.ServerURL); err != nil {
		return fmt.Errorf("server-url invalido: %w", err)
	}
	if strings.TrimSpace(c.FallbackURL) != "" {
		if err := validateAgentBaseURL(c.FallbackURL); err != nil {
			return fmt.Errorf("fallback-url invalido: %w", err)
		}
	}
	return nil
}

func validateAgentBaseURL(raw string) error {
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme no soportado: %s", u.Scheme)
	}
	if u.Host == "" {
		return errors.New("host requerido")
	}
	return nil
}

func newAgentHTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 90 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          32,
		MaxIdleConnsPerHost:   8,
	}}
}

type agentEndpointManager struct {
	mu       sync.Mutex
	base     string
	fallback string
	cfg      AgentClientConfig
	client   *http.Client
	logger   *slog.Logger
	lastTry  time.Time
}

func newAgentEndpointManager(cfg AgentClientConfig, client *http.Client, logger *slog.Logger) *agentEndpointManager {
	return &agentEndpointManager{base: strings.TrimRight(cfg.ServerURL, "/"), fallback: strings.TrimRight(strings.TrimSpace(cfg.FallbackURL), "/"), cfg: cfg, client: client, logger: logger}
}

func (m *agentEndpointManager) Base() string {
	base, _ := m.Snapshot()
	return base
}

func (m *agentEndpointManager) Fallback() string {
	_, fallback := m.Snapshot()
	return fallback
}

func (m *agentEndpointManager) Snapshot() (string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.base, m.fallback
}

func (m *agentEndpointManager) ConfiguredServerURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return strings.TrimRight(strings.TrimSpace(m.cfg.ServerURL), "/")
}

func (m *agentEndpointManager) ReportFailure(ctx context.Context, cause error) bool {
	m.mu.Lock()
	if m.fallback == "" || m.fallback == m.base || time.Since(m.lastTry) < 10*time.Second {
		m.mu.Unlock()
		return false
	}
	fallback := m.fallback
	m.lastTry = time.Now()
	m.mu.Unlock()

	discoveryCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	discovery, err := discoverAgentEndpoint(discoveryCtx, m.client, fallback, m.cfg)
	if err != nil {
		if m.logger != nil {
			m.logger.Warn("descubrimiento por IP fallo", "fallback", fallback, "error", err.Error(), "cause", cause.Error())
		}
		return false
	}
	return m.applyDiscovery(ctx, discovery, true)
}

func (m *agentEndpointManager) ApplyHint(ctx context.Context, discovery AgentDiscovery) bool {
	return m.applyDiscovery(ctx, discovery, false)
}

func (m *agentEndpointManager) applyDiscovery(ctx context.Context, discovery AgentDiscovery, allowFallbackAsBase bool) bool {
	serverURL := strings.TrimRight(strings.TrimSpace(discovery.ServerURL), "/")
	fallbackURL := strings.TrimRight(strings.TrimSpace(discovery.FallbackURL), "/")
	if serverURL == "" && fallbackURL == "" {
		return false
	}

	m.mu.Lock()
	currentBase := m.base
	currentFallback := m.fallback
	currentConfiguredServer := strings.TrimRight(strings.TrimSpace(m.cfg.ServerURL), "/")
	currentConfiguredFallback := strings.TrimRight(strings.TrimSpace(m.cfg.FallbackURL), "/")
	m.mu.Unlock()

	nextBase := currentBase
	if serverURL != "" && serverURL != currentBase {
		if agentEndpointReachable(ctx, m.client, serverURL) {
			nextBase = serverURL
		} else if m.logger != nil {
			m.logger.Warn("dominio principal sugerido aun no responde", "server", serverURL)
		}
	}
	if allowFallbackAsBase && nextBase == currentBase {
		for _, candidate := range uniqueNonEmpty(fallbackURL, currentFallback) {
			if candidate != currentBase && agentEndpointReachable(ctx, m.client, candidate) {
				nextBase = candidate
				break
			}
		}
	}
	nextFallback := currentFallback
	if fallbackURL != "" {
		nextFallback = fallbackURL
	}

	// La URL configurada/persistida debe ser el dominio principal sugerido por el
	// servidor cuando exista. La IP fallback solo debe quedar como URL activa de
	// rescate, no como PANGOLITE_SERVER_URL, para que el cliente vuelva a intentar
	// el dominio nuevo después de reinicios o propagación DNS.
	nextConfiguredServer := currentConfiguredServer
	if serverURL != "" {
		nextConfiguredServer = serverURL
	} else if nextConfiguredServer == "" {
		nextConfiguredServer = nextBase
	}
	nextConfiguredFallback := currentConfiguredFallback
	if fallbackURL != "" {
		nextConfiguredFallback = fallbackURL
	}

	changed := nextBase != currentBase || nextFallback != currentFallback || nextConfiguredServer != currentConfiguredServer || nextConfiguredFallback != currentConfiguredFallback
	if !changed {
		return false
	}
	m.mu.Lock()
	m.base = nextBase
	m.fallback = nextFallback
	m.cfg.ServerURL = nextConfiguredServer
	m.cfg.FallbackURL = nextConfiguredFallback
	configPath := m.cfg.ConfigPath
	m.mu.Unlock()
	if err := persistAgentEndpointConfig(configPath, nextConfiguredServer, nextConfiguredFallback); err != nil && m.logger != nil {
		m.logger.Warn("no se pudo actualizar env del cliente", "path", configPath, "error", err.Error())
	}
	if m.logger != nil {
		m.logger.Info("endpoint de cliente actualizado", "active", nextBase, "server", nextConfiguredServer, "fallback", nextConfiguredFallback)
	}
	return true
}

func discoverAgentEndpoint(ctx context.Context, client *http.Client, fallback string, cfg AgentClientConfig) (AgentDiscovery, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(fallback, "/")+"/api/agent/discover", nil)
	if err != nil {
		return AgentDiscovery{}, err
	}
	setAgentAuthWithEndpoint(req, cfg, strings.TrimRight(cfg.ServerURL, "/"), fallback)
	res, err := client.Do(req)
	if err != nil {
		return AgentDiscovery{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return AgentDiscovery{}, fmt.Errorf("estado %s: %s", res.Status, strings.TrimSpace(string(b)))
	}
	var out AgentDiscovery
	if err := json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&out); err != nil {
		return AgentDiscovery{}, err
	}
	return out, nil
}

func agentEndpointReachable(ctx context.Context, client *http.Client, base string) bool {
	if base == "" {
		return false
	}
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(checkCtx, http.MethodGet, strings.TrimRight(base, "/")+"/healthz", nil)
	if err != nil {
		return false
	}
	res, err := client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode >= 200 && res.StatusCode < 500
}

func discoveryFromHeaders(h http.Header) AgentDiscovery {
	return AgentDiscovery{
		ServerURL:   strings.TrimRight(strings.TrimSpace(h.Get("X-Pangolite-Server-URL")), "/"),
		FallbackURL: strings.TrimRight(strings.TrimSpace(h.Get("X-Pangolite-Fallback-URL")), "/"),
		Domain:      strings.TrimSpace(h.Get("X-Pangolite-Domain")),
		PublicIP:    strings.TrimSpace(h.Get("X-Pangolite-Public-IP")),
	}
}

func persistAgentEndpointConfig(path, serverURL, fallbackURL string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	updates := map[string]string{
		"PANGOLITE_SERVER_URL":   strings.TrimRight(strings.TrimSpace(serverURL), "/"),
		"PANGOLITE_FALLBACK_URL": strings.TrimRight(strings.TrimSpace(fallbackURL), "/"),
	}
	out, seen := rewriteEnvLines(string(b), updates)
	for key, value := range updates {
		if !seen[key] && strings.TrimSpace(value) != "" {
			out = append(out, key+"="+value)
		}
	}
	data := []byte(strings.Join(out, "\n") + "\n")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".pangolite-client.env-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	return os.Rename(tmpName, path)
}

func rewriteEnvLines(content string, updates map[string]string) ([]string, map[string]bool) {
	seen := map[string]bool{}
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			out = append(out, line)
			continue
		}
		key, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			out = append(out, line)
			continue
		}
		key = strings.TrimSpace(key)
		value, shouldUpdate := updates[key]
		if !shouldUpdate {
			out = append(out, line)
			continue
		}
		seen[key] = true
		out = append(out, key+"="+value)
	}
	return out, seen
}

func uniqueNonEmpty(values ...string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range values {
		v = strings.TrimRight(strings.TrimSpace(v), "/")
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func RunAgent(ctx context.Context, cfg AgentClientConfig, logger *slog.Logger) error {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	client := newAgentHTTPClient()
	endpoints := newAgentEndpointManager(cfg, client, logger)
	logger.Info("cliente NAT iniciado", "server", endpoints.Base(), "fallback", endpoints.Fallback(), "agent", cfg.AgentID)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		runHTTPJobLoop(ctx, client, endpoints, cfg, logger)
	}()
	go func() {
		defer wg.Done()
		runStreamJobLoop(ctx, client, endpoints, cfg, logger)
	}()
	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func runHTTPJobLoop(ctx context.Context, client *http.Client, endpoints *agentEndpointManager, cfg AgentClientConfig, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		base, fallback := endpoints.Snapshot()
		configuredServer := endpoints.ConfiguredServerURL()
		job, ok, hint, err := pollAgentJob(ctx, client, base, configuredServer, fallback, cfg)
		if hint.ServerURL != "" || hint.FallbackURL != "" {
			endpoints.ApplyHint(ctx, hint)
		}
		if err != nil {
			logger.Warn("poll HTTP/UDP fallo", "server", base, "error", err.Error())
			endpoints.ReportFailure(ctx, err)
			sleepContext(ctx, 3*time.Second)
			continue
		}
		if !ok {
			sleepContext(ctx, cfg.PollInterval)
			continue
		}
		go func(job AgentJob) {
			logger.Info("job recibido", "job", job.ID, "kind", job.Kind, "target", fmt.Sprintf("%s:%d", job.TargetHost, job.TargetPort))
			resp := executeAgentJob(ctx, client, job, cfg.RequestTimeout)
			if err := postAgentResponse(ctx, client, endpoints.Base(), endpoints.ConfiguredServerURL(), endpoints.Fallback(), cfg, job.ID, resp); err != nil {
				logger.Warn("respuesta de job no enviada", "job", job.ID, "error", err.Error())
			}
		}(job)
	}
}

func runStreamJobLoop(ctx context.Context, client *http.Client, endpoints *agentEndpointManager, cfg AgentClientConfig, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		base, fallback := endpoints.Snapshot()
		configuredServer := endpoints.ConfiguredServerURL()
		job, ok, hint, err := pollAgentStream(ctx, client, base, configuredServer, fallback, cfg)
		if hint.ServerURL != "" || hint.FallbackURL != "" {
			endpoints.ApplyHint(ctx, hint)
		}
		if err != nil {
			logger.Warn("poll stream fallo", "server", base, "error", err.Error())
			endpoints.ReportFailure(ctx, err)
			sleepContext(ctx, 3*time.Second)
			continue
		}
		if !ok {
			sleepContext(ctx, cfg.PollInterval)
			continue
		}
		go func(job AgentStreamJob) {
			base, fallback := endpoints.Snapshot()
			handleAgentStream(ctx, base, endpoints.ConfiguredServerURL(), fallback, cfg, job, logger)
		}(job)
	}
}

func pollAgentJob(ctx context.Context, client *http.Client, base, configuredServer, fallback string, cfg AgentClientConfig) (AgentJob, bool, AgentDiscovery, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/agent/poll", nil)
	if err != nil {
		return AgentJob{}, false, AgentDiscovery{}, err
	}
	setAgentAuthWithEndpoint(req, cfg, configuredServer, fallback)
	res, err := client.Do(req)
	if err != nil {
		return AgentJob{}, false, AgentDiscovery{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return AgentJob{}, false, discoveryFromHeaders(res.Header), nil
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return AgentJob{}, false, discoveryFromHeaders(res.Header), fmt.Errorf("estado %s: %s", res.Status, strings.TrimSpace(string(b)))
	}
	var job AgentJob
	if err := json.NewDecoder(res.Body).Decode(&job); err != nil {
		return AgentJob{}, false, AgentDiscovery{}, err
	}
	if job.ID == "" {
		return AgentJob{}, false, AgentDiscovery{}, errors.New("job sin id")
	}
	return job, true, discoveryFromHeaders(res.Header), nil
}

func pollAgentStream(ctx context.Context, client *http.Client, base, configuredServer, fallback string, cfg AgentClientConfig) (AgentStreamJob, bool, AgentDiscovery, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/agent/stream-poll", nil)
	if err != nil {
		return AgentStreamJob{}, false, AgentDiscovery{}, err
	}
	setAgentAuthWithEndpoint(req, cfg, configuredServer, fallback)
	res, err := client.Do(req)
	if err != nil {
		return AgentStreamJob{}, false, AgentDiscovery{}, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return AgentStreamJob{}, false, discoveryFromHeaders(res.Header), nil
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return AgentStreamJob{}, false, discoveryFromHeaders(res.Header), fmt.Errorf("estado %s: %s", res.Status, strings.TrimSpace(string(b)))
	}
	var job AgentStreamJob
	if err := json.NewDecoder(res.Body).Decode(&job); err != nil {
		return AgentStreamJob{}, false, AgentDiscovery{}, err
	}
	if job.ID == "" {
		return AgentStreamJob{}, false, AgentDiscovery{}, errors.New("stream sin id")
	}
	return job, true, discoveryFromHeaders(res.Header), nil
}

func executeAgentJob(ctx context.Context, client *http.Client, job AgentJob, requestTimeout time.Duration) AgentResponse {
	if job.Kind == ModeUDP {
		return runUDPAgentJob(ctx, job)
	}
	if job.TargetScheme != "http" && job.TargetScheme != "https" {
		return AgentResponse{JobID: job.ID, StatusCode: http.StatusBadGateway, Error: "scheme de backend no soportado"}
	}
	target := url.URL{Scheme: job.TargetScheme, Host: net.JoinHostPort(job.TargetHost, strconv.Itoa(job.TargetPort)), Path: job.Path, RawQuery: job.RawQuery}
	if requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, requestTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, job.Method, target.String(), bytes.NewReader(job.Body))
	if err != nil {
		return AgentResponse{JobID: job.ID, StatusCode: http.StatusBadGateway, Error: err.Error()}
	}
	copySafeHeader(req.Header, job.Header)
	res, err := client.Do(req)
	if err != nil {
		return AgentResponse{JobID: job.ID, StatusCode: http.StatusBadGateway, Error: err.Error()}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return AgentResponse{JobID: job.ID, StatusCode: http.StatusBadGateway, Error: err.Error()}
	}
	return AgentResponse{JobID: job.ID, StatusCode: res.StatusCode, Header: cloneSafeHeader(res.Header), Body: body}
}

func handleAgentStream(ctx context.Context, base, configuredServer, fallback string, cfg AgentClientConfig, job AgentStreamJob, logger *slog.Logger) {
	if job.Mode == AgentStreamModeTerminal {
		handleAgentTerminalStream(ctx, base, configuredServer, fallback, cfg, job, logger)
		return
	}
	if job.Mode != ModeTCP {
		logger.Warn("stream no soportado", "stream", job.ID, "mode", job.Mode)
		return
	}
	addr := net.JoinHostPort(job.TargetHost, strconv.Itoa(job.TargetPort))
	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	backend, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		logger.Warn("backend TCP no disponible", "stream", job.ID, "target", addr, "error", err.Error())
		return
	}
	defer backend.Close()
	wsURL, err := agentWebSocketURL(base, "/api/agent/streams/"+url.PathEscape(job.ID))
	if err != nil {
		logger.Warn("url de stream invalida", "stream", job.ID, "error", err.Error())
		return
	}
	header := http.Header{}
	setAgentAuthHeaderWithEndpoint(header, cfg, configuredServer, fallback)
	ws, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		logger.Warn("websocket de stream fallo", "stream", job.ID, "error", err.Error())
		return
	}
	logger.Info("stream TCP conectado", "stream", job.ID, "target", addr)
	if err := bridgeWebSocketNetConn(ctx, ws, backend); err != nil {
		logger.Debug("stream TCP cerrado", "stream", job.ID, "error", err.Error())
	}
}

func handleAgentTerminalStream(ctx context.Context, base, configuredServer, fallback string, cfg AgentClientConfig, job AgentStreamJob, logger *slog.Logger) {
	wsURL, err := agentWebSocketURL(base, "/api/agent/streams/"+url.PathEscape(job.ID))
	if err != nil {
		logger.Warn("url de terminal invalida", "stream", job.ID, "error", err.Error())
		return
	}
	header := http.Header{}
	setAgentAuthHeaderWithEndpoint(header, cfg, configuredServer, fallback)
	ws, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		logger.Warn("websocket de terminal fallo", "stream", job.ID, "error", err.Error())
		return
	}

	term, err := startTerminalProcess(ctx, terminalStartOptions{Shell: job.Shell, Cols: job.Cols, Rows: job.Rows})
	if err != nil {
		logger.Warn("terminal remota no disponible", "stream", job.ID, "error", err.Error())
		_ = ws.Write(ctx, websocket.MessageText, []byte("No se pudo iniciar la terminal remota en el cliente: "+err.Error()+"\r\n"))
		_ = ws.Close(websocket.StatusInternalError, "terminal no disponible")
		return
	}
	defer term.Close()

	logger.Info("terminal remota conectada", "stream", job.ID, "os", runtime.GOOS)
	if err := bridgeWebSocketTerminalProcess(ctx, ws, term, true); err != nil {
		logger.Debug("terminal remota cerrada", "stream", job.ID, "error", err.Error())
	}
}

func postAgentResponse(ctx context.Context, client *http.Client, base, configuredServer, fallback string, cfg AgentClientConfig, jobID string, resp AgentResponse) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/agent/jobs/"+url.PathEscape(jobID)+"/response", bytes.NewReader(b))
	if err != nil {
		return err
	}
	setAgentAuthWithEndpoint(req, cfg, configuredServer, fallback)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("estado %s: %s", res.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

func agentWebSocketURL(base, path string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("scheme no soportado: %s", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = ""
	return u.String(), nil
}

func setAgentAuth(req *http.Request, cfg AgentClientConfig) {
	setAgentAuthWithEndpoint(req, cfg, cfg.ServerURL, cfg.FallbackURL)
}

func setAgentAuthWithEndpoint(req *http.Request, cfg AgentClientConfig, serverURL, fallbackURL string) {
	setAgentAuthHeaderWithEndpoint(req.Header, cfg, serverURL, fallbackURL)
}

func setAgentAuthHeader(h http.Header, cfg AgentClientConfig) {
	setAgentAuthHeaderWithEndpoint(h, cfg, cfg.ServerURL, cfg.FallbackURL)
}

func setAgentAuthHeaderWithEndpoint(h http.Header, cfg AgentClientConfig, serverURL, fallbackURL string) {
	h.Set("Authorization", "Bearer "+cfg.Token)
	h.Set("X-Pangolite-Agent", cfg.AgentID)
	h.Set("User-Agent", "pangolite-client/0.5")
	h.Set("X-Pangolite-Client-Version", Version)
	h.Set("X-Pangolite-Client-OS", runtime.GOOS)
	h.Set("X-Pangolite-Client-Arch", runtime.GOARCH)
	if strings.TrimSpace(serverURL) != "" {
		h.Set("X-Pangolite-Client-Server-URL", strings.TrimRight(strings.TrimSpace(serverURL), "/"))
	}
	if strings.TrimSpace(fallbackURL) != "" {
		h.Set("X-Pangolite-Client-Fallback-URL", strings.TrimRight(strings.TrimSpace(fallbackURL), "/"))
	}
	if hn, err := os.Hostname(); err == nil {
		h.Set("X-Pangolite-Client-Hostname", hn)
	}
	if ip := firstPrivateIP(); ip != "" {
		h.Set("X-Pangolite-Client-Private-IP", ip)
	}
}

func firstPrivateIP() string {
	ifs, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifs {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			if v4 := ip.To4(); v4 != nil {
				return v4.String()
			}
		}
	}
	return ""
}

func sleepContext(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
