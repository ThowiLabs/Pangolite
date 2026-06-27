package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"
)

type TraefikConfig struct {
	HTTP *HTTPConfig `json:"http,omitempty"`
	TCP  *TCPConfig  `json:"tcp,omitempty"`
	UDP  *UDPConfig  `json:"udp,omitempty"`
}

type HTTPConfig struct {
	Routers     map[string]HTTPRouter     `json:"routers,omitempty"`
	Services    map[string]HTTPService    `json:"services,omitempty"`
	Middlewares map[string]HTTPMiddleware `json:"middlewares,omitempty"`
}

type HTTPRouter struct {
	Rule        string     `json:"rule"`
	Service     string     `json:"service"`
	EntryPoints []string   `json:"entryPoints"`
	Middlewares []string   `json:"middlewares,omitempty"`
	TLS         *TLSConfig `json:"tls,omitempty"`
	Priority    int        `json:"priority,omitempty"`
}

type TLSConfig struct {
	CertResolver string      `json:"certResolver,omitempty"`
	Domains      []TLSDomain `json:"domains,omitempty"`
}

type TLSDomain struct {
	Main string `json:"main"`
}

type HTTPService struct {
	LoadBalancer HTTPLoadBalancer `json:"loadBalancer"`
}

type HTTPLoadBalancer struct {
	Servers []HTTPServer `json:"servers"`
}

type HTTPServer struct {
	URL string `json:"url"`
}

type HTTPMiddleware struct {
	RedirectScheme *RedirectScheme `json:"redirectScheme,omitempty"`
}

type RedirectScheme struct {
	Scheme string `json:"scheme"`
}

type TCPConfig struct {
	Routers  map[string]TCPRouter  `json:"routers,omitempty"`
	Services map[string]TCPService `json:"services,omitempty"`
}

type TCPRouter struct {
	Rule        string   `json:"rule"`
	EntryPoints []string `json:"entryPoints"`
	Service     string   `json:"service"`
}

type TCPService struct {
	LoadBalancer TCPUDPLoadBalancer `json:"loadBalancer"`
}

type UDPConfig struct {
	Routers  map[string]UDPRouter  `json:"routers,omitempty"`
	Services map[string]UDPService `json:"services,omitempty"`
}

type UDPRouter struct {
	EntryPoints []string `json:"entryPoints"`
	Service     string   `json:"service"`
}

type UDPService struct {
	LoadBalancer TCPUDPLoadBalancer `json:"loadBalancer"`
}

type TCPUDPLoadBalancer struct {
	Servers []TCPUDPServer `json:"servers"`
}

type TCPUDPServer struct {
	Address string `json:"address"`
}

func BuildTraefikConfig(resources []Resource) TraefikConfig {
	cfg := TraefikConfig{
		HTTP: &HTTPConfig{
			Routers:  map[string]HTTPRouter{},
			Services: map[string]HTTPService{},
			Middlewares: map[string]HTTPMiddleware{
				"redirect-to-https": {RedirectScheme: &RedirectScheme{Scheme: "https"}},
			},
		},
	}

	for _, r := range resources {
		if !r.Enabled && r.Mode != ModeHTTP {
			continue
		}
		key := safeName(r.ID + "-" + r.Name)
		svc := key + "-service"
		router := key + "-router"

		switch r.Mode {
		case ModeHTTP:
			entry := "web"
			var tls *TLSConfig
			if r.TLS {
				entry = "websecure"
				tls = &TLSConfig{CertResolver: "letsencrypt"}
			}
			rule := fmt.Sprintf("Host(`%s`)", r.Domain)
			if r.PathPrefix != "" && r.PathPrefix != "/" {
				rule += fmt.Sprintf(" && PathPrefix(`%s`)", r.PathPrefix)
			}
			cfg.HTTP.Routers[router] = HTTPRouter{
				Rule:        rule,
				Service:     svc,
				EntryPoints: []string{entry},
				TLS:         tls,
				Priority:    100,
			}
			if r.TLS {
				cfg.HTTP.Routers[router+"-redirect"] = HTTPRouter{
					Rule:        rule,
					Service:     svc,
					EntryPoints: []string{"web"},
					Middlewares: []string{"redirect-to-https"},
					Priority:    100,
				}
			}
			serviceURL := r.ServiceURL()
			if r.UsesAgent() || !r.Enabled || r.ProtectionMode != ProtectionNone {
				serviceURL = "http://127.0.0.1:2424"
			}
			cfg.HTTP.Services[svc] = HTTPService{LoadBalancer: HTTPLoadBalancer{Servers: []HTTPServer{{URL: serviceURL}}}}
		case ModeTCP:
			if cfg.TCP == nil {
				cfg.TCP = &TCPConfig{Routers: map[string]TCPRouter{}, Services: map[string]TCPService{}}
			}
			ep := fmt.Sprintf("tcp-%d", r.PublicPort)
			cfg.TCP.Routers[router] = TCPRouter{Rule: "HostSNI(`*`)", EntryPoints: []string{ep}, Service: svc}
			address := r.ServiceAddress()
			if r.UsesAgent() {
				address = r.BridgeAddress()
			}
			cfg.TCP.Services[svc] = TCPService{LoadBalancer: TCPUDPLoadBalancer{Servers: []TCPUDPServer{{Address: address}}}}
		case ModeUDP:
			if cfg.UDP == nil {
				cfg.UDP = &UDPConfig{Routers: map[string]UDPRouter{}, Services: map[string]UDPService{}}
			}
			ep := fmt.Sprintf("udp-%d", r.PublicPort)
			cfg.UDP.Routers[router] = UDPRouter{EntryPoints: []string{ep}, Service: svc}
			address := r.ServiceAddress()
			if r.UsesAgent() {
				address = r.BridgeAddress()
			}
			cfg.UDP.Services[svc] = UDPService{LoadBalancer: TCPUDPLoadBalancer{Servers: []TCPUDPServer{{Address: address}}}}
		}
	}

	if len(cfg.HTTP.Routers) == 0 && len(cfg.HTTP.Services) == 0 {
		cfg.HTTP = nil
	}
	return cfg
}

func EncodeTraefikJSON(resources []Resource) ([]byte, error) {
	return json.MarshalIndent(BuildTraefikConfig(resources), "", "  ")
}

type StaticTraefikData struct {
	DashboardDomain  string
	LetsEncryptEmail string
	ControlURL       string
	PanelURL         string
	PanelEnabled     bool
	ACMEEnabled      bool
	TCPPorts         []int
	UDPPorts         []int
	DynamicDir       string
}

func RenderStaticTraefik(c Config, resources []Resource) error {
	if err := c.ValidateForRender(); err != nil {
		return err
	}
	if err := os.MkdirAll(c.TraefikDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(c.TraefikDir, "dynamic"), 0o755); err != nil {
		return err
	}
	staticPath := filepath.Join(c.TraefikDir, "traefik.yml")
	dynamicPath := filepath.Join(c.TraefikDir, "dynamic", "pangolite-dashboard.yml")
	backups, err := backupTraefikFiles(staticPath, dynamicPath)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if committed {
			cleanupTraefikBackups(backups)
		} else {
			restoreTraefikBackups(backups)
		}
	}()

	data := StaticTraefikData{
		DashboardDomain:  c.DashboardDomain,
		LetsEncryptEmail: c.LetsEncryptEmail,
		ControlURL:       "http://127.0.0.1:2424/api/v1/traefik-config",
		PanelURL:         "http://127.0.0.1:2424",
		PanelEnabled:     strings.TrimSpace(c.DashboardDomain) != "",
		ACMEEnabled:      ACMEEnabled(c),
		DynamicDir:       filepath.Join(c.TraefikDir, "dynamic"),
		TCPPorts:         uniquePorts(resources, ModeTCP),
		UDPPorts:         uniquePorts(resources, ModeUDP),
	}
	if port := ListenPortFromAddr(c.Addr); port > 0 {
		data.ControlURL = fmt.Sprintf("http://127.0.0.1:%d/api/v1/traefik-config", port)
		data.PanelURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	if err := renderFile(staticPath, traefikYAMLTemplate, data, 0o644); err != nil {
		return err
	}
	if err := renderDynamicTraefikFile(c); err != nil {
		return err
	}
	acme := filepath.Join(c.TraefikDir, "acme.json")
	if _, err := os.Stat(acme); os.IsNotExist(err) {
		if err := os.WriteFile(acme, []byte("{}\n"), 0o600); err != nil {
			return err
		}
	}
	if err := os.Chmod(acme, 0o600); err != nil {
		return err
	}
	if err := ValidateTraefikConfig(c); err != nil {
		return err
	}
	committed = true
	return nil
}

func RenderDynamicTraefik(c Config) error {
	if err := c.ValidateForRender(); err != nil {
		return err
	}
	dynamicPath := filepath.Join(c.TraefikDir, "dynamic", "pangolite-dashboard.yml")
	backups, err := backupTraefikFiles(dynamicPath)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if committed {
			cleanupTraefikBackups(backups)
		} else {
			restoreTraefikBackups(backups)
		}
	}()
	if err := renderDynamicTraefikFile(c); err != nil {
		return err
	}
	if err := ValidateTraefikConfig(c); err != nil {
		return err
	}
	committed = true
	return nil
}

func renderDynamicTraefikFile(c Config) error {
	dynamicDir := filepath.Join(c.TraefikDir, "dynamic")
	if err := os.MkdirAll(dynamicDir, 0o755); err != nil {
		return err
	}
	data := StaticTraefikData{
		DashboardDomain:  c.DashboardDomain,
		LetsEncryptEmail: c.LetsEncryptEmail,
		PanelURL:         "http://127.0.0.1:2424",
		PanelEnabled:     strings.TrimSpace(c.DashboardDomain) != "",
		ACMEEnabled:      ACMEEnabled(c),
		DynamicDir:       filepath.Join(c.TraefikDir, "dynamic"),
	}
	if port := ListenPortFromAddr(c.Addr); port > 0 {
		data.PanelURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	return renderFile(filepath.Join(dynamicDir, "pangolite-dashboard.yml"), dynamicDashboardYAMLTemplate, data, 0o644)
}

func TraefikPortSignature(resources []Resource) string {
	var parts []string
	for _, port := range uniquePorts(resources, ModeTCP) {
		parts = append(parts, fmt.Sprintf("tcp:%d", port))
	}
	for _, port := range uniquePorts(resources, ModeUDP) {
		parts = append(parts, fmt.Sprintf("udp:%d", port))
	}
	return strings.Join(parts, ",")
}

type traefikFileBackup struct {
	Path    string
	Backup  string
	Existed bool
}

func backupTraefikFiles(paths ...string) ([]traefikFileBackup, error) {
	stamp := time.Now().UTC().Format("20060102150405")
	out := make([]traefikFileBackup, 0, len(paths))
	for _, path := range paths {
		item := traefikFileBackup{Path: path, Backup: path + ".bak-" + stamp}
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("respaldar %s: %w", path, err)
			}
			if err := os.WriteFile(item.Backup, data, 0o600); err != nil {
				return nil, fmt.Errorf("escribir respaldo %s: %w", item.Backup, err)
			}
			item.Existed = true
		}
		out = append(out, item)
	}
	return out, nil
}

func restoreTraefikBackups(backups []traefikFileBackup) {
	for _, item := range backups {
		if item.Existed {
			if data, err := os.ReadFile(item.Backup); err == nil {
				_ = os.WriteFile(item.Path, data, 0o644)
			}
			_ = os.Remove(item.Backup)
		} else {
			_ = os.Remove(item.Path)
		}
	}
}

func cleanupTraefikBackups(backups []traefikFileBackup) {
	for _, item := range backups {
		if item.Backup != "" {
			_ = os.Remove(item.Backup)
		}
	}
}

func ValidateTraefikConfig(c Config) error {
	if os.Getenv("PANGOLITE_SKIP_TRAEFIK_CHECK") == "1" {
		return nil
	}
	path := filepath.Join(c.TraefikDir, "traefik.yml")
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	traefikBin, err := exec.LookPath("traefik")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, traefikBin, "check", "--configFile", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("validacion Traefik fallo; se restauro la configuracion anterior: %s", msg)
	}
	return nil
}

func renderFile(path, tpl string, data any, perm os.FileMode) error {
	parsed, err := template.New(filepath.Base(path)).Parse(tpl)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if err := parsed.Execute(f, data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func uniquePorts(resources []Resource, mode string) []int {
	seen := map[int]bool{}
	for _, r := range resources {
		if r.Enabled && r.Mode == mode && r.PublicPort > 0 {
			seen[r.PublicPort] = true
		}
	}
	ports := make([]int, 0, len(seen))
	for port := range seen {
		ports = append(ports, port)
	}
	sort.Ints(ports)
	return ports
}

func ACMEEnabled(c Config) bool {
	email := strings.ToLower(strings.TrimSpace(c.LetsEncryptEmail))
	if email == "" {
		return false
	}
	if strings.HasSuffix(email, "@example.com") || strings.HasSuffix(email, "@example.org") || strings.HasSuffix(email, "@example.net") {
		return false
	}
	return true
}

var nameCleaner = regexp.MustCompile(`[^a-zA-Z0-9-]+`)

func safeName(s string) string {
	s = strings.ToLower(s)
	s = nameCleaner.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "resource"
	}
	return s
}

const traefikYAMLTemplate = `# managed by Pangolite - do not edit manually
api:
  dashboard: false

providers:
  http:
    endpoint: {{ printf "%q" .ControlURL }}
    pollInterval: 5s
  file:
    directory: {{ printf "%q" .DynamicDir }}
    watch: true

log:
  level: INFO

accessLog:
  fields:
    headers:
      defaultMode: drop
      names:
        User-Agent: keep
        Authorization: redact
        Cookie: redact

{{- if .ACMEEnabled }}
certificatesResolvers:
  letsencrypt:
    acme:
      email: {{ printf "%q" .LetsEncryptEmail }}
      storage: /etc/traefik/acme.json
      httpChallenge:
        entryPoint: web
{{- end }}

entryPoints:
  web:
    address: ":80"
  websecure:
    address: ":443"
{{- range .TCPPorts }}
  tcp-{{ . }}:
    address: ":{{ . }}/tcp"
{{- end }}
{{- range .UDPPorts }}
  udp-{{ . }}:
    address: ":{{ . }}/udp"
{{- end }}

ping:
  entryPoint: web
`

const dynamicDashboardYAMLTemplate = `# managed by Pangolite - base dynamic config
http:
  middlewares:
    redirect-to-https:
      redirectScheme:
        scheme: https
{{- if .PanelEnabled }}

  routers:
{{- if .ACMEEnabled }}
    pangolite-panel-redirect:
      rule: "Host(` + "`" + `{{ .DashboardDomain }}` + "`" + `)"
      service: pangolite-panel
      entryPoints:
        - web
      middlewares:
        - redirect-to-https

    pangolite-panel:
      rule: "Host(` + "`" + `{{ .DashboardDomain }}` + "`" + `)"
      service: pangolite-panel
      entryPoints:
        - websecure
      tls:
        certResolver: letsencrypt
        domains:
          - main: {{ printf "%q" .DashboardDomain }}
{{- else }}
    pangolite-panel:
      rule: "Host(` + "`" + `{{ .DashboardDomain }}` + "`" + `)"
      service: pangolite-panel
      entryPoints:
        - web
{{- end }}

  services:
    pangolite-panel:
      loadBalancer:
        servers:
          - url: "{{ .PanelURL }}"
{{- end }}
`
