package app

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProjectIDFromRequestPreservesTerminalContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	project, err := store.AddProject(Project{Name: "Proyecto Terminal"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "cliente-01"})
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{store: store}

	byProject := httptest.NewRequest("GET", "/terminal?projectId="+project.ID, nil)
	if got := s.projectIDFromRequest(byProject); got != project.ID {
		t.Fatalf("projectId desde query = %q, want %q", got, project.ID)
	}

	byAgent := httptest.NewRequest("GET", "/terminal?agentId="+agent.ID, nil)
	if got := s.projectIDFromRequest(byAgent); got != project.ID {
		t.Fatalf("projectId desde agentId = %q, want %q", got, project.ID)
	}
}

func TestPanelPageForSSHConnections(t *testing.T) {
	var page panelPage
	found := false
	for _, route := range panelRouteDefinitions {
		if route.Pattern == "GET /ssh" {
			page = route.Page
			found = true
			break
		}
	}
	if !found {
		t.Fatal("la ruta GET /ssh no está registrada en Go")
	}
	if page.Key != "ssh_connections" {
		t.Fatalf("page.Key = %q, want ssh_connections", page.Key)
	}
	if page.Template != "ssh_connections.html" {
		t.Fatalf("page.Template = %q, want ssh_connections.html", page.Template)
	}
}

func TestAvailableConnectionsOnlyCountsUsableTerminals(t *testing.T) {
	agents := []AgentPublic{
		{Enabled: true, Online: true, OS: "linux"},
		{Enabled: true, Online: false, OS: "linux"},
		{Enabled: true, Online: true, OS: "windows"},
		{Enabled: false, Online: true, OS: "linux"},
	}
	if got := availableConnections("linux", agents); got != 2 {
		t.Fatalf("availableConnections(linux) = %d, want 2", got)
	}
	if got := availableConnections("windows", agents); got != 1 {
		t.Fatalf("availableConnections(windows) = %d, want 1", got)
	}
}

func TestRenderSSHConnectionsPage(t *testing.T) {
	project := Project{ID: "project1", Name: "Proyecto Demo", Slug: "demo", Enabled: true}
	agent := AgentPublic{ProjectID: project.ID, ID: "agent1", Name: "Cliente Demo", Enabled: true, Online: true, OS: "linux", Arch: "amd64", Hostname: "demo-host", PrivateIP: "10.0.0.2", LastSeen: time.Now().UTC()}
	data := uiPageData{
		Title: "Pangolite - Conexiones SSH", Path: "/ssh", PageKey: "ssh_connections", Crumb: "Acceso remoto", PageHeading: "Conexiones SSH", Username: "admin",
		Projects: []Project{project}, Agents: []AgentPublic{agent}, Stats: map[string]map[string]int{}, ServerHostname: "pangolite-host", ServerOS: "linux", ServerArch: "amd64",
		Settings: AppSettings{DashboardDomain: "panel.example.test"}, Network: NetworkInfo{PublicIP: "203.0.113.10"}, BootstrapJSON: template.JS(`{"pageKey":"ssh_connections"}`),
	}
	recorder := httptest.NewRecorder()
	renderUIPage(recorder, "ssh_connections.html", data)
	body := recorder.Body.String()
	for _, expected := range []string{"Conexiones SSH", "Servidor Pangolite", "Cliente Demo", "Proyecto Demo", "/assets/app/ssh-connections.js"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("render no contiene %q", expected)
		}
	}
}

func TestFrontendDoesNotOwnPanelRouting(t *testing.T) {
	agentsScript, err := assetsFS.ReadFile("assets/app/agents.js")
	if err != nil {
		t.Fatal(err)
	}
	pageInit, err := assetsFS.ReadFile("assets/app/page-init.js")
	if err != nil {
		t.Fatal(err)
	}
	combined := string(agentsScript) + "\n" + string(pageInit)
	for _, forbidden := range []string{"function route(", "async function route(", "location.pathname"} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("el frontend todavía contiene lógica de routing prohibida: %q", forbidden)
		}
	}
	if !strings.Contains(string(pageInit), "serverPageKey()") || !strings.Contains(string(pageInit), "appBoot.pageKey") {
		t.Fatal("la hidratación debe usar el PageKey emitido por Go")
	}
}

func TestPanelRouteDefinitionsIncludeTerminalAndSSH(t *testing.T) {
	keys := map[string]string{}
	for _, route := range panelRouteDefinitions {
		keys[route.Pattern] = route.Page.Key
	}
	if keys["GET /ssh"] != "ssh_connections" {
		t.Fatal("GET /ssh debe estar administrado por Go")
	}
	if keys["GET /terminal"] != "terminal" {
		t.Fatal("GET /terminal debe estar administrado por Go")
	}
}

func panelSessionCookie(t *testing.T, store *Store) *http.Cookie {
	t.Helper()
	_, temporaryPassword, err := store.BootstrapAdmin("admin", filepath.Join(t.TempDir(), "admin-password.txt"))
	if err != nil {
		t.Fatal(err)
	}
	user, ok := store.AuthenticateUser("admin", temporaryPassword)
	if !ok {
		t.Fatal("no se pudo autenticar el administrador temporal")
	}
	const password = "clave-segura-panel-2026"
	if err := store.ChangePassword(user.ID, temporaryPassword, password, "admin@example.test", false); err != nil {
		t.Fatal(err)
	}
	user, ok = store.AuthenticateUser("admin", password)
	if !ok {
		t.Fatal("no se pudo autenticar el administrador definitivo")
	}
	rawID, _, err := store.CreateSession(user.ID, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: sessionCookieName, Value: rawID, Path: "/"}
}

func TestPanelRoutesAreResolvedByGo(t *testing.T) {
	server, store := testServerWithStore(t)
	cookie := panelSessionCookie(t, store)

	req := httptest.NewRequest(http.MethodGet, "https://panel.example.test/ssh", nil)
	req.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /ssh = %d, want 200; body=%q", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "data-page-key=\"ssh_connections\"") {
		t.Fatal("Go no marcó la página ssh_connections en el HTML")
	}
}

func TestUnknownPanelRouteReturnsNotFound(t *testing.T) {
	server, _ := testServerWithStore(t)
	req := httptest.NewRequest(http.MethodGet, "https://panel.example.test/ruta-inexistente", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("ruta desconocida = %d, want 404", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "" {
		t.Fatalf("una ruta desconocida no debe redirigir, Location=%q", location)
	}
}

func TestUnknownProjectRouteReturnsNotFoundInGo(t *testing.T) {
	server, store := testServerWithStore(t)
	cookie := panelSessionCookie(t, store)
	req := httptest.NewRequest(http.MethodGet, "https://panel.example.test/projects/no-existe/resources", nil)
	req.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("proyecto desconocido = %d, want 404", recorder.Code)
	}
}

func TestTerminalRouteRejectsUnknownAgentInGo(t *testing.T) {
	server, store := testServerWithStore(t)
	cookie := panelSessionCookie(t, store)
	req := httptest.NewRequest(http.MethodGet, "https://panel.example.test/terminal?agentId=no-existe&autoconnect=1", nil)
	req.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("cliente desconocido en terminal = %d, want 404", recorder.Code)
	}
}
