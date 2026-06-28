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
	project, err := store.AddProject(Project{Name: "Proyecto Smoke"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.AddResource(Resource{ProjectID: project.ID, Name: "SSH", Mode: ModeTCP, PublicPort: 2222, BackendHost: "127.0.0.1", BackendPort: 22, Enabled: true})
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
	project, err := store.AddProject(Project{Name: "Proyecto Web"})
	if err != nil {
		t.Fatal(err)
	}
	created, err := store.AddResource(Resource{ProjectID: project.ID, Name: "App", Mode: ModeHTTP, Domain: "app.example.com", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 8080, TLS: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := store.UpdateResourceControl(created.ID, false, DisabledResponseHTML, 403, "<h1>Suspendido</h1>", "")
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
	hidden, err := store.UpdateResourceControl(created.ID, false, DisabledResponseHidden, 404, "<h1>No debe guardarse</h1>", "payment")
	if err != nil {
		t.Fatal(err)
	}
	if hidden.DisabledResponseMode != DisabledResponseHidden || hidden.DisabledStatusCode != 404 || hidden.DisabledHTML != "" || hidden.DisabledTemplateID != "" {
		t.Fatalf("suspension oculta inesperada: %#v", hidden)
	}
}

func TestProjectsSeparateResourcesAndAgents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Proyecto Norte", Notes: "smoke"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "nat-01"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddResource(Resource{ProjectID: project.ID, Name: "App", Mode: ModeHTTP, Domain: "proyecto.example.com", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 8080, AgentID: agent.ID, TLS: true, Enabled: true}); err != nil {
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
	if _, err := store.AddProject(Project{Name: "Proyecto ACME"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddProject(Project{Name: "proyecto acme"}); err == nil {
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

func TestFreshStoreDoesNotCreateDefaultProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if got := len(store.ListProjects()); got != 0 {
		t.Fatalf("se esperaba base sin proyectos por defecto, got %d", got)
	}
}

func TestManagedDomainLifecycleBlocksUsedDomain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Proyecto Dominio"})
	if err != nil {
		t.Fatal(err)
	}
	oldDomain, err := store.AddManagedDomain("old.example.mx")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetPrimaryManagedDomain(oldDomain.Domain, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "cliente-01", ServerURL: "https://old.example.mx", DomainID: oldDomain.ID}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteManagedDomain(oldDomain.ID); err == nil {
		t.Fatal("se esperaba bloquear eliminacion de dominio usado por cliente")
	}
	newDomain, err := store.AddManagedDomain("new.example.mx")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetPrimaryManagedDomain(newDomain.Domain, oldDomain.Domain); err != nil {
		t.Fatal(err)
	}
	legacy, err := store.ManagedDomainByID(oldDomain.ID)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Status != DomainStatusLegacy || legacy.Enabled || legacy.Primary {
		t.Fatalf("dominio anterior debe quedar heredado: %#v", legacy)
	}
	domains := store.PanelDomainsForTraefik(newDomain.Domain)
	if len(domains) != 2 || domains[0] != newDomain.Domain || domains[1] != oldDomain.Domain {
		t.Fatalf("dominios de panel inesperados: %#v", domains)
	}
}

func TestLegacyPanelDomainCanDeleteWhenNoDirectUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Proyecto Legado"})
	if err != nil {
		t.Fatal(err)
	}
	oldDomain, err := store.AddManagedDomain("legacy.example.mx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "cliente-legado", ServerURL: "https://otro.example.mx", FallbackURL: "http://203.0.113.10:2424"}); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPrimaryManagedDomain("newlegacy.example.mx", oldDomain.Domain); err != nil {
		t.Fatal(err)
	}
	legacy, err := store.ManagedDomainByID(oldDomain.ID)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Status != DomainStatusLegacy || legacy.DeleteLocked {
		t.Fatalf("dominio heredado sin uso directo debe poder eliminarse: %#v", legacy)
	}
	if err := store.DeleteManagedDomain(oldDomain.ID); err != nil {
		t.Fatal(err)
	}
}

func TestManagedDomainWithUnconfirmedFallbackStaysLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Proyecto Fallback Pendiente"})
	if err != nil {
		t.Fatal(err)
	}
	domain, err := store.AddManagedDomain("pending.example.mx")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "cliente-pendiente", ServerURL: "https://pending.example.mx", FallbackURL: "http://203.0.113.10:2424", DomainID: domain.ID}); err != nil {
		t.Fatal(err)
	}
	domain, err = store.ManagedDomainByID(domain.ID)
	if err != nil {
		t.Fatal(err)
	}
	if domain.AgentCount != 1 || !domain.DeleteLocked {
		t.Fatalf("dominio con fallback no confirmado debe seguir bloqueado: %#v", domain)
	}
}

func TestManagedDomainWithConfirmedFallbackAgentsCanBeDeleted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Proyecto Fallback"})
	if err != nil {
		t.Fatal(err)
	}
	domain, err := store.AddManagedDomain("fallback.example.mx")
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "cliente-fallback", ServerURL: "https://fallback.example.mx", FallbackURL: "http://203.0.113.10:2424", DomainID: domain.ID})
	if err != nil {
		t.Fatal(err)
	}
	store.TouchAgent(agent.ID, AgentHeartbeat{ServerURL: "https://fallback.example.mx", FallbackURL: "http://203.0.113.10:2424"})
	domain, err = store.ManagedDomainByID(domain.ID)
	if err != nil {
		t.Fatal(err)
	}
	if domain.AgentCount != 1 || domain.DeleteLocked {
		t.Fatalf("dominio con cliente y fallback confirmado no debe bloquear eliminacion: %#v", domain)
	}
	if err := store.DeleteManagedDomain(domain.ID); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateAgentInstallEndpointsPersistsFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Proyecto Token"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "cliente-token", ServerURL: "https://old.example.mx"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateAgentInstallEndpoints(agent.ID, "https://new.example.mx/", "http://203.0.113.10:2424/", ""); err != nil {
		t.Fatal(err)
	}
	updated, err := store.AgentByID(agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ServerURL != "https://new.example.mx" || updated.FallbackURL != "http://203.0.113.10:2424" {
		t.Fatalf("endpoints inesperados: server=%q fallback=%q", updated.ServerURL, updated.FallbackURL)
	}
	if updated.FallbackReady {
		t.Fatal("la URL fallback generada no debe contarse como confirmada hasta que el cliente reporte su configuracion")
	}
	store.TouchAgent(agent.ID, AgentHeartbeat{ServerURL: "https://new.example.mx", FallbackURL: "http://203.0.113.10:2424"})
	confirmed, err := store.AgentByID(agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed.FallbackReady {
		t.Fatal("se esperaba fallback confirmado despues del heartbeat del cliente")
	}
}

func TestProjectAgentListIncludesLinkedLegacyAgents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Proyecto Clientes"})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := store.AddProject(Project{Name: "Proyecto Legacy"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: legacy.ID, Name: "cliente-usado"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.Exec(`INSERT INTO resources(id,project_id,name,mode,domain,path_prefix,public_port,tunnel_port,backend_scheme,backend_host,backend_port,origin_type,agent_id,tls,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, "reslegacy1", project.ID, "App Legacy", ModeHTTP, "legacy.example.com", "/", nil, nil, "http", "127.0.0.1", 8080, OriginAgent, agent.ID, 1, 1, formatTime(agent.CreatedAt), formatTime(agent.UpdatedAt)); err != nil {
		t.Fatal(err)
	}
	agents := store.ListAgentsByProject(project.ID)
	if len(agents) != 1 || agents[0].ID != agent.ID {
		t.Fatalf("se esperaba listar cliente ligado por recurso legacy: %#v", agents)
	}
	stats := store.ProjectStats()
	if stats[project.ID]["agents"] != 1 {
		t.Fatalf("contador de clientes inconsistente: %#v", stats[project.ID])
	}
}

func TestProjectIdentifierAcceptsSlugForAgents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Pruebas"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "cliente-pruebas"})
	if err != nil {
		t.Fatal(err)
	}
	agents := store.ListAgentsByProject(project.Slug)
	if len(agents) != 1 || agents[0].ID != agent.ID {
		t.Fatalf("se esperaba resolver proyecto por slug: %#v", agents)
	}
}

func TestEffectiveConfigUsesActiveManagedDomainWhenDashboardSettingIsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.AddManagedDomain("hircoir.duckdns.org"); err != nil {
		t.Fatal(err)
	}
	effective := store.EffectiveConfig(Config{})
	if effective.DashboardDomain != "hircoir.duckdns.org" {
		t.Fatalf("se esperaba usar dominio activo como principal efectivo, got %q", effective.DashboardDomain)
	}
}

func TestSuspendAgentWebResourcesKeepsTCPAndRestoresOnlyAffected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Proyecto Mantenimiento"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "cliente-mantenimiento"})
	if err != nil {
		t.Fatal(err)
	}
	webActive, err := store.AddResource(Resource{ProjectID: project.ID, Name: "Web activa", Mode: ModeHTTP, Domain: "web.example.mx", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 8080, OriginType: OriginAgent, AgentID: agent.ID, TLS: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	webOff, err := store.AddResource(Resource{ProjectID: project.ID, Name: "Web apagada", Mode: ModeHTTP, Domain: "off.example.mx", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 8081, OriginType: OriginAgent, AgentID: agent.ID, TLS: true, Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	tcp, err := store.AddResource(Resource{ProjectID: project.ID, Name: "SSH", Mode: ModeTCP, PublicPort: 2222, BackendHost: "127.0.0.1", BackendPort: 22, OriginType: OriginAgent, AgentID: agent.ID, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	updated, resources, err := store.SuspendAgentWebResources(agent.ID, AgentWebMaintenanceOptions{ResponseMode: DisabledResponseHTML, StatusCode: 200, HTML: "<h1>Mantenimiento</h1>"})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.WebMaintenanceActive || updated.WebSuspendedCount != 1 {
		t.Fatalf("mantenimiento inesperado: %#v", updated)
	}
	if len(resources) != 3 {
		t.Fatalf("recursos inesperados: %#v", resources)
	}
	activeAfter, _ := store.ResourceByID(webActive.ID)
	if activeAfter.Enabled || activeAfter.DisabledResponseMode != DisabledResponseHTML {
		t.Fatalf("web activa debe quedar suspendida: %#v", activeAfter)
	}
	tcpAfter, _ := store.ResourceByID(tcp.ID)
	if !tcpAfter.Enabled {
		t.Fatalf("tcp no debe suspenderse: %#v", tcpAfter)
	}
	_, _, err = store.ResumeAgentWebResources(agent.ID)
	if err != nil {
		t.Fatal(err)
	}
	activeRestored, _ := store.ResourceByID(webActive.ID)
	if !activeRestored.Enabled {
		t.Fatalf("web afectada debe reactivarse: %#v", activeRestored)
	}
	offStillOff, _ := store.ResourceByID(webOff.ID)
	if offStillOff.Enabled {
		t.Fatalf("web que ya estaba apagada no debe activarse: %#v", offStillOff)
	}
	tcpRestored, _ := store.ResourceByID(tcp.ID)
	if !tcpRestored.Enabled {
		t.Fatalf("tcp debe seguir activo: %#v", tcpRestored)
	}
}

func TestSuspendAgentMaintenanceCanTargetTCPUDP(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pangolite.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	project, err := store.AddProject(Project{Name: "Proyecto TCP UDP"})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := store.AddAgent(Agent{ProjectID: project.ID, Name: "cliente-tcpudp"})
	if err != nil {
		t.Fatal(err)
	}
	web, err := store.AddResource(Resource{ProjectID: project.ID, Name: "Web", Mode: ModeHTTP, Domain: "web.example.mx", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 8080, OriginType: OriginAgent, AgentID: agent.ID, TLS: true, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	tcp, err := store.AddResource(Resource{ProjectID: project.ID, Name: "SSH", Mode: ModeTCP, PublicPort: 2222, BackendHost: "127.0.0.1", BackendPort: 22, OriginType: OriginAgent, AgentID: agent.ID, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	udp, err := store.AddResource(Resource{ProjectID: project.ID, Name: "DNS", Mode: ModeUDP, PublicPort: 5353, BackendHost: "127.0.0.1", BackendPort: 53, OriginType: OriginAgent, AgentID: agent.ID, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	updated, _, err := store.SuspendAgentResources(agent.ID, AgentMaintenanceOptions{TCP: true, UDP: true})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.MaintenanceActive || updated.TCPSuspendedCount != 1 || updated.UDPSuspendedCount != 1 || updated.WebSuspendedCount != 0 {
		t.Fatalf("mantenimiento tcp/udp inesperado: %#v", updated)
	}
	webAfter, _ := store.ResourceByID(web.ID)
	if !webAfter.Enabled {
		t.Fatalf("web no debe suspenderse: %#v", webAfter)
	}
	tcpAfter, _ := store.ResourceByID(tcp.ID)
	udpAfter, _ := store.ResourceByID(udp.ID)
	if tcpAfter.Enabled || udpAfter.Enabled {
		t.Fatalf("tcp/udp deben quedar suspendidos: tcp=%#v udp=%#v", tcpAfter, udpAfter)
	}
	updated, _, err = store.ResumeAgentResources(agent.ID, false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if updated.TCPSuspendedCount != 0 || updated.UDPSuspendedCount != 1 || !updated.MaintenanceActive {
		t.Fatalf("debe quedar solo udp suspendido: %#v", updated)
	}
	tcpRestored, _ := store.ResourceByID(tcp.ID)
	udpStillOff, _ := store.ResourceByID(udp.ID)
	if !tcpRestored.Enabled || udpStillOff.Enabled {
		t.Fatalf("reactivacion parcial inesperada: tcp=%#v udp=%#v", tcpRestored, udpStillOff)
	}
}
