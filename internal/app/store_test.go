package app

import (
	"path/filepath"
	"testing"
)

func TestStoreAddDeletePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	created, err := store.AddResource(Resource{Name: "SSH", Mode: ModeTCP, PublicPort: 2222, BackendHost: "127.0.0.1", BackendPort: 22, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.ListResources()) != 1 {
		t.Fatal("se esperaba un recurso")
	}
	reloaded, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Close()
	if len(reloaded.ListResources()) != 1 {
		t.Fatal("se esperaba persistencia")
	}
	if err := reloaded.DeleteResource(created.ID); err != nil {
		t.Fatal(err)
	}
	if len(reloaded.ListResources()) != 0 {
		t.Fatal("se esperaba borrar recurso")
	}
}

func TestBootstrapAdminAndSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	passFile := filepath.Join(t.TempDir(), "admin-password.txt")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	created, temp, err := store.BootstrapAdmin("admin", passFile)
	if err != nil {
		t.Fatal(err)
	}
	if !created || temp == "" {
		t.Fatal("se esperaba admin temporal")
	}
	user, ok := store.AuthenticateUser("admin", temp)
	if !ok {
		t.Fatal("credenciales temporales no autenticaron")
	}
	raw, sess, err := store.CreateSession(user.ID, sessionDuration(Config{SessionDays: 30}))
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || sess.CSRFToken == "" {
		t.Fatal("sesion incompleta")
	}
	_, sessionUser, ok := store.SessionWithUser(raw)
	if !ok || sessionUser.Username != "admin" {
		t.Fatal("sesion no encontrada")
	}
}

func TestStoreUpdateResourceControl(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	created, err := store.AddResource(Resource{Name: "App", Mode: ModeHTTP, Domain: "app.example.com", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 8080, TLS: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdateResourceControl(created.ID, false, DisabledResponseHTML, 403, "<h1>Suspendido</h1>")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Enabled || updated.DisabledResponseMode != DisabledResponseHTML || updated.DisabledHTML == "" {
		t.Fatalf("control inesperado: %#v", updated)
	}
	found, ok := store.FindHTTPPanelResource("app.example.com", "/")
	if !ok || found.ID != created.ID || found.Enabled {
		t.Fatalf("se esperaba recurso suspendido enrutable por Pangolite: %#v", found)
	}
}

func TestProjectsSeparateResourcesAndAgents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Cliente Norte", Notes: "smoke"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "nat-01"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddResource(Resource{ProjectID: project.ID, Name: "App", Mode: ModeHTTP, Domain: "cliente.example.com", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 8080, AgentID: agent.ID, TLS: true, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if len(store.ListResourcesByProject(project.ID)) != 1 {
		t.Fatal("se esperaba recurso del proyecto")
	}
	if len(store.ListAgentsByProject(project.ID)) != 1 {
		t.Fatal("se esperaba agente del proyecto")
	}
}

func TestProjectNameMustBeUnique(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.AddProject(Project{Name: "Cliente ACME"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddProject(Project{Name: "cliente acme"}); err == nil {
		t.Fatal("se esperaba rechazar nombre duplicado")
	}
}

func TestManagedDomainsCRUD(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	domain, err := store.AddManagedDomain("example.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(store.ListManagedDomains()) != 1 {
		t.Fatal("se esperaba dominio administrado")
	}
	if _, err := store.AddManagedDomain("EXAMPLE.com"); err == nil {
		t.Fatal("se esperaba rechazar dominio duplicado")
	}
	if err := store.DeleteManagedDomain(domain.ID); err != nil {
		t.Fatal(err)
	}
	if len(store.ListManagedDomains()) != 0 {
		t.Fatal("se esperaba dominio eliminado")
	}
}
