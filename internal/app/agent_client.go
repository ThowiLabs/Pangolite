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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

type AgentClientConfig struct {
	ServerURL      string
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
	if _, err := url.ParseRequestURI(c.ServerURL); err != nil {
		return fmt.Errorf("server-url invalido: %w", err)
	}
	return nil
}

func RunAgent(ctx context.Context, cfg AgentClientConfig, logger *slog.Logger) error {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	base := strings.TrimRight(cfg.ServerURL, "/")
	client := &http.Client{}
	logger.Info("cliente NAT iniciado", "server", base, "agent", cfg.AgentID)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		runHTTPJobLoop(ctx, client, base, cfg, logger)
	}()
	go func() {
		defer wg.Done()
		runStreamJobLoop(ctx, client, base, cfg, logger)
	}()
	<-ctx.Done()
	wg.Wait()
	return ctx.Err()
}

func runHTTPJobLoop(ctx context.Context, client *http.Client, base string, cfg AgentClientConfig, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		job, ok, err := pollAgentJob(ctx, client, base, cfg)
		if err != nil {
			logger.Warn("poll HTTP/UDP fallo", "error", err.Error())
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
			if err := postAgentResponse(ctx, client, base, cfg, job.ID, resp); err != nil {
				logger.Warn("respuesta de job no enviada", "job", job.ID, "error", err.Error())
			}
		}(job)
	}
}

func runStreamJobLoop(ctx context.Context, client *http.Client, base string, cfg AgentClientConfig, logger *slog.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		job, ok, err := pollAgentStream(ctx, client, base, cfg)
		if err != nil {
			logger.Warn("poll stream fallo", "error", err.Error())
			sleepContext(ctx, 3*time.Second)
			continue
		}
		if !ok {
			sleepContext(ctx, cfg.PollInterval)
			continue
		}
		go handleAgentStream(ctx, base, cfg, job, logger)
	}
}

func pollAgentJob(ctx context.Context, client *http.Client, base string, cfg AgentClientConfig) (AgentJob, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/agent/poll", nil)
	if err != nil {
		return AgentJob{}, false, err
	}
	setAgentAuth(req, cfg)
	res, err := client.Do(req)
	if err != nil {
		return AgentJob{}, false, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return AgentJob{}, false, nil
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return AgentJob{}, false, fmt.Errorf("estado %s: %s", res.Status, strings.TrimSpace(string(b)))
	}
	var job AgentJob
	if err := json.NewDecoder(res.Body).Decode(&job); err != nil {
		return AgentJob{}, false, err
	}
	if job.ID == "" {
		return AgentJob{}, false, errors.New("job sin id")
	}
	return job, true, nil
}

func pollAgentStream(ctx context.Context, client *http.Client, base string, cfg AgentClientConfig) (AgentStreamJob, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/agent/stream-poll", nil)
	if err != nil {
		return AgentStreamJob{}, false, err
	}
	setAgentAuth(req, cfg)
	res, err := client.Do(req)
	if err != nil {
		return AgentStreamJob{}, false, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return AgentStreamJob{}, false, nil
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return AgentStreamJob{}, false, fmt.Errorf("estado %s: %s", res.Status, strings.TrimSpace(string(b)))
	}
	var job AgentStreamJob
	if err := json.NewDecoder(res.Body).Decode(&job); err != nil {
		return AgentStreamJob{}, false, err
	}
	if job.ID == "" {
		return AgentStreamJob{}, false, errors.New("stream sin id")
	}
	return job, true, nil
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

func handleAgentStream(ctx context.Context, base string, cfg AgentClientConfig, job AgentStreamJob, logger *slog.Logger) {
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
	setAgentAuthHeader(header, cfg)
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

func postAgentResponse(ctx context.Context, client *http.Client, base string, cfg AgentClientConfig, jobID string, resp AgentResponse) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/agent/jobs/"+url.PathEscape(jobID)+"/response", bytes.NewReader(b))
	if err != nil {
		return err
	}
	setAgentAuth(req, cfg)
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
	setAgentAuthHeader(req.Header, cfg)
}

func setAgentAuthHeader(h http.Header, cfg AgentClientConfig) {
	h.Set("Authorization", "Bearer "+cfg.Token)
	h.Set("X-Pangolite-Agent", cfg.AgentID)
	h.Set("User-Agent", "pangolite-client/0.5")
	h.Set("X-Pangolite-Client-Version", Version)
	h.Set("X-Pangolite-Client-OS", runtime.GOOS)
	h.Set("X-Pangolite-Client-Arch", runtime.GOARCH)
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
