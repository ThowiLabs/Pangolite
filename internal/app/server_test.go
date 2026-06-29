package app

import (
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

func errUnexpectedAgentJob(job AgentJob) error {
	return &unexpectedAgentJobError{job: job}
}

type unexpectedAgentJobError struct {
	job AgentJob
}

func (e *unexpectedAgentJobError) Error() string {
	return "job HTTP inesperado para agente"
}
