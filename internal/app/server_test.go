package app

import (
	"bytes"
	"context"
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
