package app

import (
	"html/template"
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
	page := panelPageForPath("/ssh")
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
