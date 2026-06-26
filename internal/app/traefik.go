package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
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
	CertResolver string `json:"certResolver,omitempty"`
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
			if r.UsesAgent() || !r.Enabled {
				serviceURL = "http://127.0.0.1:2424"
			}
			cfg.HTTP.Services[svc] = HTTPService{LoadBalancer: HTTPLoadBalancer{Servers: []HTTPServer{{URL: serviceURL}}}}
		case ModeTCP:
			if cfg.TCP == nil {
				cfg.TCP = &TCPConfig{Routers: map[string]TCPRouter{}, Services: map[string]TCPService{}}
			}
			ep := fmt.Sprintf("tcp-%d", r.PublicPort)
			cfg.TCP.Routers[router] = TCPRouter{Rule: "HostSNI(`*`)", EntryPoints: []string{ep}, Service: svc}
			cfg.TCP.Services[svc] = TCPService{LoadBalancer: TCPUDPLoadBalancer{Servers: []TCPUDPServer{{Address: r.ServiceAddress()}}}}
		case ModeUDP:
			if cfg.UDP == nil {
				cfg.UDP = &UDPConfig{Routers: map[string]UDPRouter{}, Services: map[string]UDPService{}}
			}
			ep := fmt.Sprintf("udp-%d", r.PublicPort)
			cfg.UDP.Routers[router] = UDPRouter{EntryPoints: []string{ep}, Service: svc}
			cfg.UDP.Services[svc] = UDPService{LoadBalancer: TCPUDPLoadBalancer{Servers: []TCPUDPServer{{Address: r.ServiceAddress()}}}}
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
}

func RenderStaticTraefik(c Config, resources []Resource) error {
	if err := c.ValidateForRender(); err != nil {
		return err
	}
	if err := os.MkdirAll(c.TraefikDir, 0o755); err != nil {
		return err
	}
	data := StaticTraefikData{
		DashboardDomain:  c.DashboardDomain,
		LetsEncryptEmail: c.LetsEncryptEmail,
		ControlURL:       "http://127.0.0.1:2424/api/v1/traefik-config",
		PanelURL:         "http://127.0.0.1:2424",
		PanelEnabled:     strings.TrimSpace(c.DashboardDomain) != "",
		ACMEEnabled:      ACMEEnabled(c),
		TCPPorts:         uniquePorts(resources, ModeTCP),
		UDPPorts:         uniquePorts(resources, ModeUDP),
	}
	if port := ListenPortFromAddr(c.Addr); port > 0 {
		data.ControlURL = fmt.Sprintf("http://127.0.0.1:%d/api/v1/traefik-config", port)
		data.PanelURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	if err := renderFile(filepath.Join(c.TraefikDir, "traefik.yml"), traefikYAMLTemplate, data, 0o644); err != nil {
		return err
	}
	if err := renderFile(filepath.Join(c.TraefikDir, "pangolite-dynamic-base.yml"), dynamicBaseYAMLTemplate, data, 0o644); err != nil {
		return err
	}
	acme := filepath.Join(c.TraefikDir, "acme.json")
	if _, err := os.Stat(acme); os.IsNotExist(err) {
		if err := os.WriteFile(acme, []byte("{}\n"), 0o600); err != nil {
			return err
		}
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
	domain := strings.ToLower(strings.TrimSpace(c.DashboardDomain))
	email := strings.ToLower(strings.TrimSpace(c.LetsEncryptEmail))
	if domain == "" || email == "" {
		return false
	}
	if domain == "pangolite.localhost" || strings.HasSuffix(domain, ".localhost") || strings.HasSuffix(domain, ".local") {
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
    filename: /etc/traefik/pangolite-dynamic-base.yml

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

const dynamicBaseYAMLTemplate = `# managed by Pangolite - base dynamic config
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
