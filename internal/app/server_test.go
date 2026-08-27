package app

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testServerWithStore(t *testing.T) (*Server, *Store) {
	t.Helper()
	base := t.TempDir()
	store, err := NewStore(filepath.Join(base, "pangolite.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := Config{Addr: "127.0.0.1:2424", DataPath: filepath.Join(base, "pangolite.db"), SessionDays: 30, AutoTraefik: false}
	cfg.ResolveBootstrapPaths()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewServer(cfg, store, logger), store
}

func TestPublicResourceGatewayInterceptsPanelPaths(t *testing.T) {
	server, store := testServerWithStore(t)
	project, err := store.AddProject(Project{Name: "Proyecto Web"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.AddResource(Resource{
		ProjectID:            project.ID,
		Name:                 "App suspendida",
		Mode:                 ModeHTTP,
		Domain:               "app.example.com",
		PathPrefix:           "/",
		BackendScheme:        "http",
		BackendHost:          "127.0.0.1",
		BackendPort:          8181,
		TLS:                  true,
		Enabled:              false,
		DisabledResponseMode: DisabledResponse403,
		DisabledStatusCode:   http.StatusForbidden,
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "https://app.example.com/login", nil)
	req.Host = "app.example.com"
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("/login de un host de recurso debe resolverse como recurso publico, got status %d body %q", rr.Code, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "Pangolite - Iniciar") {
		t.Fatal("/login del recurso publico no debe mostrar el login administrativo")
	}
}

func TestPanelLoginStillWorksOnPanelHost(t *testing.T) {
	server, _ := testServerWithStore(t)
	req := httptest.NewRequest(http.MethodGet, "https://panel.example.com/login", nil)
	req.Host = "panel.example.com"
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("login del panel debe seguir disponible, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Pangolite") {
		t.Fatal("login del panel no se renderizo correctamente")
	}
}

func TestPublicAgentResourceDoesNotReceivePanelCSP(t *testing.T) {
	server, store := testServerWithStore(t)
	project, err := store.AddProject(Project{Name: "Proyecto Web"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "Cliente JYV"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.AddResource(Resource{
		ProjectID:     project.ID,
		Name:          "JYV login",
		Mode:          ModeHTTP,
		Domain:        "jyv.example.com",
		PathPrefix:    "/",
		BackendScheme: "http",
		BackendHost:   "127.0.0.1",
		BackendPort:   8181,
		OriginType:    OriginAgent,
		AgentID:       agent.ID,
		TLS:           true,
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		job, ok, err := server.hub.Poll(ctx, agent.ID)
		if err != nil {
			errCh <- err
			return
		}
		if !ok {
			errCh <- context.Canceled
			return
		}
		if job.Path != "/login" || job.TargetHost != "127.0.0.1" || job.TargetPort != 8181 {
			errCh <- errUnexpectedAgentJob(job)
			return
		}
		header := http.Header{}
		header.Set("Content-Type", "text/html; charset=utf-8")
		header.Set("X-App-Header", "remote")
		server.hub.Complete(job.ID, AgentResponse{
			JobID:      job.ID,
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       []byte(`<script src="https://cdn.tailwindcss.com/"></script>`),
		})
		errCh <- nil
	}()

	req := httptest.NewRequest(http.MethodGet, "https://jyv.example.com/login", nil).WithContext(ctx)
	req.Host = "jyv.example.com"
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el agente de prueba no recibio el job HTTP")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("respuesta del recurso remoto esperada 200, got %d body %q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Security-Policy"); got != "" {
		t.Fatalf("los recursos publicados no deben heredar la CSP del panel, got %q", got)
	}
	if got := rr.Header().Get("X-Frame-Options"); got != "" {
		t.Fatalf("los recursos publicados no deben heredar X-Frame-Options del panel, got %q", got)
	}
	if got := rr.Header().Get("X-App-Header"); got != "remote" {
		t.Fatalf("no se preservo header del recurso remoto, got %q", got)
	}
	if !strings.Contains(rr.Body.String(), "cdn.tailwindcss.com") {
		t.Fatal("no se devolvio el HTML del recurso remoto")
	}
}

func TestPublicAgentResourceForwardsPOSTCookiesCSRFAndRedirect(t *testing.T) {
	server, store := testServerWithStore(t)
	project, err := store.AddProject(Project{Name: "Proyecto Web"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "Cliente JYV"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.AddResource(Resource{
		ProjectID:     project.ID,
		Name:          "JYV login",
		Mode:          ModeHTTP,
		Domain:        "jyv.example.com",
		PathPrefix:    "/",
		BackendScheme: "http",
		BackendHost:   "127.0.0.1",
		BackendPort:   8181,
		OriginType:    OriginAgent,
		AgentID:       agent.ID,
		TLS:           true,
		Enabled:       true,
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		job, ok, err := server.hub.Poll(ctx, agent.ID)
		if err != nil {
			errCh <- err
			return
		}
		if !ok {
			errCh <- context.Canceled
			return
		}
		if job.Method != http.MethodPost || job.Path != "/login" || string(job.Body) != "email=a&password=b" {
			errCh <- errUnexpectedAgentJob(job)
			return
		}
		if job.TargetHost != "127.0.0.1" || job.TargetPort != 8181 || job.TargetScheme != "http" {
			errCh <- errUnexpectedAgentJob(job)
			return
		}
		if job.PublicHost != "jyv.example.com" || job.PublicScheme != "https" {
			errCh <- errUnexpectedAgentJob(job)
			return
		}
		if job.Header.Get("X-CSRF-Token") != "csrf" {
			errCh <- errUnexpectedAgentJob(job)
			return
		}
		if job.Header.Get("X-Forwarded-Host") != "jyv.example.com" || job.Header.Get("X-Forwarded-Proto") != "https" || job.Header.Get("X-Forwarded-Port") != "443" {
			errCh <- errUnexpectedAgentJob(job)
			return
		}
		if got := job.Header.Get("Cookie"); got != "XSRF-TOKEN=token; laravel_session=session" {
			errCh <- errUnexpectedAgentJob(job)
			return
		}
		header := http.Header{}
		header.Set("Location", "http://127.0.0.1:8181/dashboard")
		header.Add("Set-Cookie", "laravel_session=new; Domain=127.0.0.1; Path=/; HttpOnly")
		server.hub.Complete(job.ID, AgentResponse{JobID: job.ID, StatusCode: http.StatusFound, Header: header})
		errCh <- nil
	}()

	req := httptest.NewRequest(http.MethodPost, "https://jyv.example.com/login", bytes.NewBufferString("email=a&password=b")).WithContext(ctx)
	req.Host = "jyv.example.com"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-CSRF-Token", "csrf")
	req.Header.Set("Cookie", "XSRF-TOKEN=token; laravel_session=session; pangolite_resource_secret=internal")
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el agente de prueba no recibio el POST HTTP")
	}
	if rr.Code != http.StatusFound {
		t.Fatalf("respuesta esperada 302, got %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "https://jyv.example.com/dashboard" {
		t.Fatalf("Location no fue reescrita al dominio publico: %q", got)
	}
	if got := rr.Header().Values("Set-Cookie"); len(got) != 1 || strings.Contains(strings.ToLower(got[0]), "domain=127.0.0.1") {
		t.Fatalf("Set-Cookie interno no fue normalizado: %#v", got)
	}
}

func TestPublicAgentResourceForwardsDeleteMethod(t *testing.T) {
	server, store := testServerWithStore(t)
	project, err := store.AddProject(Project{Name: "Proyecto Web"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "Cliente API"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.AddResource(Resource{ProjectID: project.ID, Name: "API", Mode: ModeHTTP, Domain: "api.example.com", PathPrefix: "/", BackendHost: "127.0.0.1", BackendPort: 8181, OriginType: OriginAgent, AgentID: agent.ID, TLS: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		job, ok, err := server.hub.Poll(ctx, agent.ID)
		if err != nil {
			errCh <- err
			return
		}
		if !ok || job.Method != http.MethodDelete || job.Path != "/api/items/7" || job.RawQuery != "force=1" {
			errCh <- errUnexpectedAgentJob(job)
			return
		}
		server.hub.Complete(job.ID, AgentResponse{JobID: job.ID, StatusCode: http.StatusNoContent})
		errCh <- nil
	}()

	req := httptest.NewRequest(http.MethodDelete, "https://api.example.com/api/items/7?force=1", nil).WithContext(ctx)
	req.Host = "api.example.com"
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el agente de prueba no recibio el DELETE HTTP")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("DELETE no fue proxyado correctamente, got %d", rr.Code)
	}
}

func errUnexpectedAgentJob(job AgentJob) error {
	return &unexpectedAgentJobError{job: job}
}

type unexpectedAgentJobError struct {
	job AgentJob
}

func (e *unexpectedAgentJobError) Error() string {
	return "job HTTP inesperado para agente"
}

func TestPermanentResourceRedirectPreservesPathAndQuery(t *testing.T) {
	server, store := testServerWithStore(t)
	project, err := store.AddProject(Project{Name: "Proyecto Redirect"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.AddResource(Resource{
		ProjectID:          project.ID,
		Name:               "Dominio viejo",
		Mode:               ModeHTTP,
		Domain:             "old.example.com",
		PathPrefix:         "/",
		BackendScheme:      "http",
		BackendHost:        "127.0.0.1",
		BackendPort:        8080,
		TLS:                true,
		Enabled:            true,
		RedirectEnabled:    true,
		RedirectTarget:     "https://new.example.com",
		RedirectStatusCode: RedirectStatusPermanent,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "https://old.example.com/login?next=%2Fhome", nil)
	req.Host = "old.example.com"
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != RedirectStatusPermanent {
		t.Fatalf("status inesperado: %d", rr.Code)
	}
	if got := rr.Header().Get("Location"); got != "https://new.example.com/login?next=%2Fhome" {
		t.Fatalf("location inesperado: %s", got)
	}
}

func TestHiddenUnavailableLocalResourceReturns404(t *testing.T) {
	server, store := testServerWithStore(t)
	project, err := store.AddProject(Project{Name: "Proyecto Oculto"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.AddResource(Resource{
		ProjectID:           project.ID,
		Name:                "Backend caido",
		Mode:                ModeHTTP,
		Domain:              "hidden.example.com",
		PathPrefix:          "/",
		BackendScheme:       "http",
		BackendHost:         "127.0.0.1",
		BackendPort:         1,
		OriginType:          OriginLocal,
		TLS:                 false,
		Enabled:             true,
		HideWhenUnavailable: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "http://hidden.example.com/", nil)
	req.Host = "hidden.example.com"
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("backend caido oculto debe responder 404, got %d", rr.Code)
	}
	if strings.TrimSpace(rr.Body.String()) != "" {
		t.Fatalf("404 oculto no debe exponer detalle, body=%q", rr.Body.String())
	}
}

func TestClientIPTrustsForwardedOnlyFromConfiguredProxy(t *testing.T) {
	server, _ := testServerWithStore(t)
	server.trustedProxyNetworks = parseCIDRs("127.0.0.1/32")

	req := httptest.NewRequest(http.MethodGet, "http://panel.example.com/login", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.10, 127.0.0.1")
	if got := server.clientIP(req); got != "198.51.100.10" {
		t.Fatalf("clientIP proxy confiable = %q, want 198.51.100.10", got)
	}

	req.RemoteAddr = "203.0.113.20:54321"
	req.Header.Set("X-Forwarded-For", "198.51.100.99")
	if got := server.clientIP(req); got != "203.0.113.20" {
		t.Fatalf("clientIP directo no debe confiar XFF: got %q", got)
	}
}

func TestAdminLoginLearnModeAllowsPasswordFromNewIP(t *testing.T) {
	server, store := testServerWithStore(t)
	server.config.AdminAccessMode = "learn"
	created, password, err := store.BootstrapAdmin("admin", filepath.Join(t.TempDir(), "admin-password.txt"))
	if err != nil || !created {
		t.Fatalf("crear administrador: created=%v err=%v", created, err)
	}

	login := func(ip string) *http.Cookie {
		t.Helper()
		body := bytes.NewBufferString(`{"username":"admin","password":"` + password + `"}`)
		req := httptest.NewRequest(http.MethodPost, "http://panel.example.com/api/login", body)
		req.RemoteAddr = ip + ":4242"
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		server.Handler().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("login desde %s = %d, body=%q", ip, rr.Code, rr.Body.String())
		}
		for _, cookie := range rr.Result().Cookies() {
			if cookie.Name == sessionCookieName {
				return cookie
			}
		}
		t.Fatalf("login desde %s no entrego cookie de sesion", ip)
		return nil
	}

	_ = login("198.51.100.25")
	cookie := login("203.0.113.10")
	if sess, _, ok := store.SessionWithUser(cookie.Value); !ok || sess.ClientIP != "203.0.113.10" {
		t.Fatalf("sesion nueva no quedo ligada a la IP reautenticada: %#v", sess)
	}
}

func TestSessionRequiresReauthenticationWhenClientIPChanges(t *testing.T) {
	server, store := testServerWithStore(t)
	server.config.AdminAccessMode = "learn"
	created, password, err := store.BootstrapAdmin("admin", filepath.Join(t.TempDir(), "admin-password.txt"))
	if err != nil || !created {
		t.Fatalf("crear administrador: created=%v err=%v", created, err)
	}

	body := bytes.NewBufferString(`{"username":"admin","password":"` + password + `"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "http://panel.example.com/api/login", body)
	loginReq.RemoteAddr = "198.51.100.25:4242"
	loginReq.Header.Set("Content-Type", "application/json")
	loginRR := httptest.NewRecorder()
	server.Handler().ServeHTTP(loginRR, loginReq)
	if loginRR.Code != http.StatusOK {
		t.Fatalf("login inicial = %d, body=%q", loginRR.Code, loginRR.Body.String())
	}
	var cookie *http.Cookie
	for _, candidate := range loginRR.Result().Cookies() {
		if candidate.Name == sessionCookieName {
			cookie = candidate
			break
		}
	}
	if cookie == nil {
		t.Fatal("login inicial no entrego cookie de sesion")
	}

	panelReq := httptest.NewRequest(http.MethodGet, "http://panel.example.com/", nil)
	panelReq.RemoteAddr = "198.51.100.99:4242"
	panelReq.AddCookie(cookie)
	panelRR := httptest.NewRecorder()
	server.Handler().ServeHTTP(panelRR, panelReq)
	if panelRR.Code != http.StatusFound || panelRR.Header().Get("Location") != "/login" {
		t.Fatalf("cambio de IP debe pedir login: status=%d location=%q", panelRR.Code, panelRR.Header().Get("Location"))
	}

	body = bytes.NewBufferString(`{"username":"admin","password":"` + password + `"}`)
	reloginReq := httptest.NewRequest(http.MethodPost, "http://panel.example.com/api/login", body)
	reloginReq.RemoteAddr = "198.51.100.99:4242"
	reloginReq.Header.Set("Content-Type", "application/json")
	reloginRR := httptest.NewRecorder()
	server.Handler().ServeHTTP(reloginRR, reloginReq)
	if reloginRR.Code != http.StatusOK {
		t.Fatalf("relogin tras cambio de IP = %d, body=%q", reloginRR.Code, reloginRR.Body.String())
	}
	var newCookie *http.Cookie
	for _, candidate := range reloginRR.Result().Cookies() {
		if candidate.Name == sessionCookieName {
			newCookie = candidate
			break
		}
	}
	if newCookie == nil || newCookie.Value == cookie.Value {
		t.Fatal("relogin debe emitir una sesion nueva")
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "http://panel.example.com/api/session", nil)
	sessionReq.RemoteAddr = "198.51.100.99:4242"
	sessionReq.AddCookie(newCookie)
	sessionRR := httptest.NewRecorder()
	server.Handler().ServeHTTP(sessionRR, sessionReq)
	if sessionRR.Code != http.StatusOK || !strings.Contains(sessionRR.Body.String(), `"authenticated":true`) {
		t.Fatalf("sesion reautenticada no quedo activa: status=%d body=%q", sessionRR.Code, sessionRR.Body.String())
	}
}

func TestFixedWindowLimiterBoundsRequests(t *testing.T) {
	limiter := newFixedWindowLimiter()
	for i := 0; i < 3; i++ {
		if _, ok := limiter.Allow("login:ip", 3, time.Minute); !ok {
			t.Fatalf("solicitud %d debio permitirse", i+1)
		}
	}
	if _, ok := limiter.Allow("login:ip", 3, time.Minute); ok {
		t.Fatal("cuarta solicitud debio bloquearse")
	}
}

type sizedByteReader struct {
	remaining int64
	value     byte
}

func (r *sizedByteReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	for i := range p {
		p[i] = r.value
	}
	r.remaining -= int64(len(p))
	return len(p), nil
}

type countingResponseWriter struct {
	header http.Header
	status int
	bytes  int64
}

func (w *countingResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *countingResponseWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}

func (w *countingResponseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	w.bytes += int64(len(p))
	return len(p), nil
}

func runFakeHTTPStreamAgent(ctx context.Context, hub *TunnelHub, agentID string, handle func(AgentStreamJob, *http.Request) *http.Response) error {
	job, ok, err := hub.PollStream(ctx, agentID)
	if err != nil {
		return err
	}
	if !ok {
		return context.Canceled
	}
	if job.Mode != AgentStreamModeHTTP {
		return &unexpectedStreamJobError{job: job}
	}
	sess, ok := hub.AttachStream(job.ID, agentID)
	if !ok {
		return context.Canceled
	}
	defer hub.CompleteStream(job.ID)
	defer sess.ClientConn.Close()

	req, err := http.ReadRequest(bufio.NewReader(sess.ClientConn))
	if err != nil {
		return err
	}
	defer req.Body.Close()
	resp := handle(job, req)
	if resp == nil {
		return context.Canceled
	}
	resp.Request = req
	return resp.Write(sess.ClientConn)
}

type unexpectedStreamJobError struct {
	job AgentStreamJob
}

func (e *unexpectedStreamJobError) Error() string {
	return "stream HTTP inesperado para agente"
}

func TestPublicAgentHTTPStreamAcceptsUploadLargerThanLegacyLimit(t *testing.T) {
	server, _ := testServerWithStore(t)
	agentID := "agent-stream-upload"
	server.hub.UpdateAgentCapabilities(agentID, AgentCapabilityHTTPStreamV1)
	resource := Resource{
		ID:            "resource-upload",
		Mode:          ModeHTTP,
		Domain:        "upload.example.com",
		BackendScheme: "http",
		BackendHost:   "127.0.0.1",
		BackendPort:   8080,
		OriginType:    OriginAgent,
		AgentID:       agentID,
		Enabled:       true,
		TLS:           true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	const extra = int64(2 << 20)
	bodySize := MaxAgentHTTPBodyBytes + extra
	go func() {
		errCh <- runFakeHTTPStreamAgent(ctx, server.hub, agentID, func(job AgentStreamJob, req *http.Request) *http.Response {
			if job.TargetScheme != "http" || job.TargetHost != "127.0.0.1" || job.TargetPort != 8080 {
				return nil
			}
			if req.Host != "upload.example.com" || req.Header.Get("X-Forwarded-Proto") != "https" {
				return nil
			}
			n, err := io.Copy(io.Discard, req.Body)
			if err != nil || n != bodySize {
				return nil
			}
			return &http.Response{
				StatusCode:    http.StatusCreated,
				Status:        "201 Created",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
				Body:          io.NopCloser(strings.NewReader("ok")),
				ContentLength: 2,
				Close:         true,
			}
		})
	}()

	body := &sizedByteReader{remaining: bodySize, value: 'x'}
	req := httptest.NewRequest(http.MethodPost, "https://upload.example.com/files", body).WithContext(ctx)
	req.Host = "upload.example.com"
	req.ContentLength = bodySize
	rr := httptest.NewRecorder()
	server.proxyViaAgent(rr, req, resource)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el agente streaming no termino el upload")
	}
	if rr.Code != http.StatusCreated || rr.Body.String() != "ok" {
		t.Fatalf("upload streaming inesperado: status=%d body=%q", rr.Code, rr.Body.String())
	}
}

func TestPublicAgentHTTPStreamDownloadsWithoutBodyLimit(t *testing.T) {
	server, _ := testServerWithStore(t)
	agentID := "agent-stream-download"
	server.hub.UpdateAgentCapabilities(agentID, AgentCapabilityHTTPStreamV1)
	resource := Resource{
		ID:            "resource-download",
		Mode:          ModeHTTP,
		Domain:        "download.example.com",
		BackendScheme: "http",
		BackendHost:   "127.0.0.1",
		BackendPort:   8080,
		OriginType:    OriginAgent,
		AgentID:       agentID,
		Enabled:       true,
		TLS:           true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	bodySize := MaxAgentHTTPBodyBytes + int64(2<<20)
	go func() {
		errCh <- runFakeHTTPStreamAgent(ctx, server.hub, agentID, func(_ AgentStreamJob, req *http.Request) *http.Response {
			_, _ = io.Copy(io.Discard, req.Body)
			return &http.Response{
				StatusCode:    http.StatusOK,
				Status:        "200 OK",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header{"Content-Type": []string{"application/octet-stream"}},
				Body:          io.NopCloser(&sizedByteReader{remaining: bodySize, value: 'z'}),
				ContentLength: bodySize,
				Close:         true,
			}
		})
	}()

	req := httptest.NewRequest(http.MethodGet, "https://download.example.com/archive.bin", nil).WithContext(ctx)
	req.Host = "download.example.com"
	cw := &countingResponseWriter{}
	server.proxyViaAgent(cw, req, resource)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("el agente streaming no termino el download")
	}
	if cw.status != http.StatusOK || cw.bytes != bodySize {
		t.Fatalf("download streaming inesperado: status=%d bytes=%d want=%d", cw.status, cw.bytes, bodySize)
	}
}

func TestHeaderLimitReaderRejectsOversizedBackendHeaders(t *testing.T) {
	payload := strings.Repeat("X", maxAgentHTTPResponseHeaderBytes+1)
	r := &headerLimitReader{r: strings.NewReader(payload), limit: maxAgentHTTPResponseHeaderBytes}
	_, err := io.ReadAll(r)
	if !errors.Is(err, errAgentHTTPResponseHeadersTooLarge) {
		t.Fatalf("error=%v, want errAgentHTTPResponseHeadersTooLarge", err)
	}
}

func TestPublicAgentHTTPStreamAllowsEarlyBackendResponse(t *testing.T) {
	server, _ := testServerWithStore(t)
	agentID := "agent-stream-early-response"
	server.hub.UpdateAgentCapabilities(agentID, AgentCapabilityHTTPStreamV1)
	resource := Resource{
		ID:            "resource-early-response",
		Mode:          ModeHTTP,
		Domain:        "upload.example.com",
		BackendScheme: "http",
		BackendHost:   "127.0.0.1",
		BackendPort:   8080,
		OriginType:    OriginAgent,
		AgentID:       agentID,
		Enabled:       true,
		TLS:           true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- runFakeHTTPStreamAgent(ctx, server.hub, agentID, func(_ AgentStreamJob, req *http.Request) *http.Response {
			// Responde sin consumir el upload completo para validar full-duplex.
			return &http.Response{
				StatusCode:    http.StatusRequestEntityTooLarge,
				Status:        "413 Request Entity Too Large",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        http.Header{"Content-Type": []string{"text/plain"}},
				Body:          io.NopCloser(strings.NewReader("rechazado")),
				ContentLength: int64(len("rechazado")),
				Close:         true,
			}
		})
	}()

	bodySize := MaxAgentHTTPBodyBytes + int64(8<<20)
	body := &sizedByteReader{remaining: bodySize, value: 'x'}
	req := httptest.NewRequest(http.MethodPost, "https://upload.example.com/files", body).WithContext(ctx)
	req.Host = "upload.example.com"
	req.ContentLength = bodySize
	rr := httptest.NewRecorder()
	server.proxyViaAgent(rr, req, resource)

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("la respuesta temprana del backend quedo bloqueada por el upload")
	}
	if rr.Code != http.StatusRequestEntityTooLarge || rr.Body.String() != "rechazado" {
		t.Fatalf("respuesta temprana inesperada: status=%d body=%q", rr.Code, rr.Body.String())
	}
}

func TestAdminLoginSessionCreationFailureReturnsDiagnosableJSON(t *testing.T) {
	server, store := testServerWithStore(t)
	created, password, err := store.BootstrapAdmin("admin", filepath.Join(t.TempDir(), "admin-password.txt"))
	if err != nil || !created {
		t.Fatalf("crear administrador: created=%v err=%v", created, err)
	}
	if _, err := store.db.Exec(`DROP TABLE sessions`); err != nil {
		t.Fatal(err)
	}

	body := bytes.NewBufferString(`{"username":"admin","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "http://panel.example.com/api/login", body)
	req.RemoteAddr = "198.51.100.25:4242"
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	server.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("fallo de sesion = %d, body=%q", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("fallo de sesion debe conservar JSON, content-type=%q", got)
	}
	if !strings.Contains(rr.Body.String(), "revisa los logs del servidor") {
		t.Fatalf("respuesta no orienta al diagnostico: %q", rr.Body.String())
	}
}
