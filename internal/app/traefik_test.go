package app

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildTraefikConfigHTTPAndTCPUDP(t *testing.T) {
	resources := []Resource{
		{ID: "http01", Name: "Panel", Mode: ModeHTTP, Domain: "app.example.com", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 8081, TLS: true, Enabled: true},
		{ID: "agent1", Name: "Casa", Mode: ModeHTTP, Domain: "casa.example.com", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 3000, AgentID: "agent01", TLS: true, Enabled: true},
		{ID: "tcp001", Name: "SSH", Mode: ModeTCP, PublicPort: 2222, BackendHost: "10.0.0.10", BackendPort: 22, Enabled: true},
		{ID: "udp001", Name: "Game", Mode: ModeUDP, PublicPort: 25565, BackendHost: "10.0.0.11", BackendPort: 25565, Enabled: true},
	}
	cfg := BuildTraefikConfig(resources)
	if cfg.HTTP == nil || len(cfg.HTTP.Routers) != 4 || len(cfg.HTTP.Services) != 2 {
		t.Fatalf("http config inesperada: %#v", cfg.HTTP)
	}
	if cfg.TCP == nil || len(cfg.TCP.Routers) != 1 || len(cfg.TCP.Services) != 1 {
		t.Fatalf("tcp config inesperada: %#v", cfg.TCP)
	}
	if cfg.UDP == nil || len(cfg.UDP.Routers) != 1 || len(cfg.UDP.Services) != 1 {
		t.Fatalf("udp config inesperada: %#v", cfg.UDP)
	}
	b, err := json.Marshal(cfg)
	if err != nil || len(b) == 0 {
		t.Fatalf("json invalido: %v", err)
	}
}

func TestResourceValidation(t *testing.T) {
	bad := Resource{Name: "x", Mode: ModeHTTP, Domain: "localhost", BackendScheme: "ftp", BackendHost: "127.0.0.1", BackendPort: 80}
	bad.Normalize(time.Now())
	if err := bad.Validate(); err == nil {
		t.Fatal("se esperaba error de validacion")
	}
	remoteTCP := Resource{ProjectID: "project01", Name: "x", Mode: ModeTCP, PublicPort: 2222, BackendHost: "127.0.0.1", BackendPort: 22, OriginType: OriginAgent, AgentID: "agent01"}
	remoteTCP.Normalize(time.Now())
	if err := remoteTCP.Validate(); err != nil {
		t.Fatalf("TCP remoto con cliente NAT debe ser valido: %v", err)
	}
	good := Resource{ProjectID: "project01", Name: "x", Mode: ModeTCP, PublicPort: 2222, BackendHost: "127.0.0.1", BackendPort: 22}
	good.Normalize(time.Now())
	if err := good.Validate(); err != nil {
		t.Fatalf("no se esperaba error: %v", err)
	}
}

func TestACMEEnabledSkipsPlaceholderValues(t *testing.T) {
	if ACMEEnabled(Config{LetsEncryptEmail: "admin@example.com"}) {
		t.Fatal("no debe activar ACME con correo example.com")
	}
	if !ACMEEnabled(Config{LetsEncryptEmail: "admin@example.mx"}) {
		t.Fatal("debe activar ACME con correo real aunque el panel no tenga dominio")
	}
}

func TestBuildTraefikConfigDisabledHTTPRoutesToPanel(t *testing.T) {
	resources := []Resource{
		{ID: "disabled01", Name: "Cliente", Mode: ModeHTTP, Domain: "cliente.example.com", PathPrefix: "/", BackendScheme: "http", BackendHost: "10.0.0.20", BackendPort: 8080, TLS: true, Enabled: false, DisabledResponseMode: DisabledResponse403, DisabledStatusCode: 403},
	}
	cfg := BuildTraefikConfig(resources)
	if cfg.HTTP == nil || len(cfg.HTTP.Services) != 1 {
		t.Fatalf("http config inesperada: %#v", cfg.HTTP)
	}
	for _, svc := range cfg.HTTP.Services {
		if got := svc.LoadBalancer.Servers[0].URL; got != "http://127.0.0.1:2424" {
			t.Fatalf("servicio suspendido debe enrutar a Pangolite, got %s", got)
		}
	}
}

func TestTraefikHelpListsCommand(t *testing.T) {
	withoutCheck := `Commands:
    healthcheck    Calls Traefik /ping endpoint.
    version        Shows the current Traefik version.`
	if traefikHelpListsCommand(withoutCheck, "check") {
		t.Fatal("no debe detectar check cuando no aparece en la lista de comandos")
	}
	withCheck := `Commands:
    check          Checks the Traefik configuration.
    healthcheck    Calls Traefik /ping endpoint.`
	if !traefikHelpListsCommand(withCheck, "check") {
		t.Fatal("debe detectar check cuando aparece en la lista de comandos")
	}
}

func TestDashboardHostRuleMultipleDomains(t *testing.T) {
	domains := NormalizePanelDomains("panel.example.mx", []string{"old.example.mx", "panel.example.mx"})
	if len(domains) != 2 {
		t.Fatalf("dominios normalizados inesperados: %#v", domains)
	}
	rule := DashboardHostRule(domains)
	if rule != "Host(`panel.example.mx`) || Host(`old.example.mx`)" {
		t.Fatalf("regla inesperada: %s", rule)
	}
}

func TestBuildTraefikConfigRedirectHTTPRoutesToPanel(t *testing.T) {
	resources := []Resource{
		{ID: "redir01", Name: "Dominio viejo", Mode: ModeHTTP, Domain: "old.example.com", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 8080, TLS: true, RedirectEnabled: true, RedirectTarget: "https://new.example.com", RedirectStatusCode: RedirectStatusPermanent, Enabled: true},
	}
	cfg := BuildTraefikConfig(resources)
	if cfg.HTTP == nil || len(cfg.HTTP.Services) != 1 {
		t.Fatalf("http config inesperada: %#v", cfg.HTTP)
	}
	for _, svc := range cfg.HTTP.Services {
		if got := svc.LoadBalancer.Servers[0].URL; got != "http://127.0.0.1:2424" {
			t.Fatalf("recurso con redirect debe enrutar a Pangolite, got %s", got)
		}
	}
}

func TestBuildTraefikConfigHiddenUnavailableRoutesToPanel(t *testing.T) {
	resources := []Resource{
		{ID: "hide01", Name: "Backend oculto", Mode: ModeHTTP, Domain: "app.example.com", PathPrefix: "/", BackendScheme: "http", BackendHost: "127.0.0.1", BackendPort: 8181, TLS: false, HideWhenUnavailable: true, Enabled: true},
	}
	cfg := BuildTraefikConfig(resources)
	if cfg.HTTP == nil || len(cfg.HTTP.Services) != 1 {
		t.Fatalf("http config inesperada: %#v", cfg.HTTP)
	}
	for _, svc := range cfg.HTTP.Services {
		if got := svc.LoadBalancer.Servers[0].URL; got != "http://127.0.0.1:2424" {
			t.Fatalf("recurso con caída oculta debe enrutar a Pangolite, got %s", got)
		}
	}
}
