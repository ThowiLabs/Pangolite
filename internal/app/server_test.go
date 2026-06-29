package app

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
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
