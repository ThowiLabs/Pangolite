package app

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	ModeHTTP = "http"
	ModeTCP  = "tcp"
	ModeUDP  = "udp"

	OriginLocal = "local"
	OriginAgent = "agent"

	DisabledResponse403  = "403"
	DisabledResponse404  = "404"
	DisabledResponseHTML = "html"
)

type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Notes     string    `json:"notes,omitempty"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type ManagedDomain struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AppSettings struct {
	DashboardDomain  string `json:"dashboardDomain"`
	LetsEncryptEmail string `json:"letsEncryptEmail"`
}

type Resource struct {
	ProjectID            string    `json:"projectId"`
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Mode                 string    `json:"mode"`
	Domain               string    `json:"domain,omitempty"`
	PathPrefix           string    `json:"pathPrefix,omitempty"`
	PublicPort           int       `json:"publicPort,omitempty"`
	BackendScheme        string    `json:"backendScheme,omitempty"`
	BackendHost          string    `json:"backendHost"`
	BackendPort          int       `json:"backendPort"`
	OriginType           string    `json:"originType"`
	AgentID              string    `json:"agentId,omitempty"`
	TLS                  bool      `json:"tls"`
	Enabled              bool      `json:"enabled"`
	DisabledResponseMode string    `json:"disabledResponseMode"`
	DisabledStatusCode   int       `json:"disabledStatusCode"`
	DisabledHTML         string    `json:"disabledHtml,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type Agent struct {
	ProjectID string    `json:"projectId"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"token,omitempty"`
	TokenHash string    `json:"-"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	LastSeen  time.Time `json:"lastSeen,omitempty"`
}

type AgentPublic struct {
	ProjectID string    `json:"projectId"`
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	LastSeen  time.Time `json:"lastSeen,omitempty"`
}

type User struct {
	ID                  int64     `json:"id"`
	Username            string    `json:"username"`
	PasswordHash        string    `json:"-"`
	ForcePasswordChange bool      `json:"forcePasswordChange"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type Session struct {
	IDHash    string
	UserID    int64
	CSRFToken string
	ExpiresAt time.Time
	CreatedAt time.Time
	LastSeen  time.Time
}

var (
	domainRe   = regexp.MustCompile(`(?i)^([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
	idRe       = regexp.MustCompile(`^[a-zA-Z0-9_-]{6,64}$`)
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,64}$`)
	slugRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	emailRe    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
)

func (p *Project) Normalize(now time.Time) {
	p.Name = strings.TrimSpace(p.Name)
	p.Slug = strings.ToLower(strings.TrimSpace(p.Slug))
	p.Notes = strings.TrimSpace(p.Notes)
	if p.Slug == "" {
		p.Slug = slugify(p.Name)
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now.UTC()
	}
	p.UpdatedAt = now.UTC()
}

func (p Project) Validate() error {
	if p.ID != "" && !idRe.MatchString(p.ID) {
		return errors.New("id de proyecto invalido")
	}
	if p.Name == "" || len(p.Name) > 120 {
		return errors.New("nombre de proyecto requerido y menor a 120 caracteres")
	}
	if !slugRe.MatchString(p.Slug) {
		return errors.New("slug de proyecto invalido")
	}
	if len(p.Notes) > 2000 {
		return errors.New("notas de proyecto demasiado largas")
	}
	return nil
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 64 {
		out = strings.Trim(out[:64], "-")
	}
	if len(out) < 3 {
		return "cliente"
	}
	return out
}

func (d *ManagedDomain) Normalize(now time.Time) {
	d.Domain = strings.ToLower(strings.TrimSpace(d.Domain))
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now.UTC()
	}
	d.UpdatedAt = now.UTC()
}

func (d ManagedDomain) Validate() error {
	if d.ID != "" && !idRe.MatchString(d.ID) {
		return errors.New("id de dominio invalido")
	}
	if !domainRe.MatchString(d.Domain) {
		return errors.New("dominio invalido")
	}
	if strings.HasPrefix(d.Domain, "localhost") || strings.HasSuffix(d.Domain, ".localhost") {
		return errors.New("localhost no debe registrarse como dominio administrado")
	}
	return nil
}

func (a *AppSettings) Normalize() {
	a.DashboardDomain = strings.ToLower(strings.TrimSpace(a.DashboardDomain))
	a.LetsEncryptEmail = strings.ToLower(strings.TrimSpace(a.LetsEncryptEmail))
}

func (a AppSettings) Validate() error {
	if a.DashboardDomain == "" && a.LetsEncryptEmail == "" {
		return nil
	}
	if a.DashboardDomain == "" {
		return errors.New("dominio del panel requerido")
	}
	if !domainRe.MatchString(a.DashboardDomain) {
		return errors.New("dominio del panel invalido")
	}
	if strings.HasPrefix(a.DashboardDomain, "localhost") || strings.HasSuffix(a.DashboardDomain, ".localhost") || strings.HasSuffix(a.DashboardDomain, ".local") {
		return errors.New("usa un dominio publico real para el panel")
	}
	if !emailRe.MatchString(a.LetsEncryptEmail) {
		return errors.New("correo ACME invalido")
	}
	return nil
}

func (r *Resource) Normalize(now time.Time) {
	r.ProjectID = strings.TrimSpace(r.ProjectID)
	if r.ProjectID == "" {
		r.ProjectID = "default"
	}
	r.Name = strings.TrimSpace(r.Name)
	r.Mode = strings.ToLower(strings.TrimSpace(r.Mode))
	r.Domain = strings.ToLower(strings.TrimSpace(r.Domain))
	r.PathPrefix = strings.TrimSpace(r.PathPrefix)
	r.BackendScheme = strings.ToLower(strings.TrimSpace(r.BackendScheme))
	r.BackendHost = strings.TrimSpace(r.BackendHost)
	r.OriginType = strings.ToLower(strings.TrimSpace(r.OriginType))
	r.AgentID = strings.TrimSpace(r.AgentID)
	if r.OriginType == "" {
		if r.AgentID != "" {
			r.OriginType = OriginAgent
		} else {
			r.OriginType = OriginLocal
		}
	}
	if r.OriginType == OriginLocal {
		r.AgentID = ""
	}
	r.DisabledResponseMode = strings.ToLower(strings.TrimSpace(r.DisabledResponseMode))
	r.DisabledHTML = strings.TrimSpace(r.DisabledHTML)
	if r.DisabledResponseMode == "" {
		r.DisabledResponseMode = DisabledResponse403
	}
	if r.DisabledStatusCode == 0 {
		r.DisabledStatusCode = 403
	}
	if r.BackendScheme == "" && r.Mode == ModeHTTP {
		r.BackendScheme = "http"
	}
	if r.PathPrefix == "" {
		r.PathPrefix = "/"
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now.UTC()
	}
	r.UpdatedAt = now.UTC()
}

func (r *Resource) Validate() error {
	if r.ID != "" && !idRe.MatchString(r.ID) {
		return errors.New("id invalido")
	}
	if r.ProjectID == "" || !idRe.MatchString(r.ProjectID) {
		return errors.New("projectId invalido")
	}
	if r.Name == "" || len(r.Name) > 120 {
		return errors.New("name requerido y menor a 120 caracteres")
	}
	if r.Mode != ModeHTTP && r.Mode != ModeTCP && r.Mode != ModeUDP {
		return errors.New("mode debe ser http, tcp o udp")
	}
	if r.BackendHost == "" || strings.ContainsAny(r.BackendHost, " /\\\t\n\r") {
		return errors.New("backendHost invalido")
	}
	if r.BackendPort < 1 || r.BackendPort > 65535 {
		return errors.New("backendPort debe estar entre 1 y 65535")
	}
	if r.OriginType != OriginLocal && r.OriginType != OriginAgent {
		return errors.New("originType debe ser local o agent")
	}
	if r.OriginType == OriginAgent && r.AgentID == "" {
		return errors.New("agentId requerido cuando el origen es cliente de sistema")
	}
	if r.AgentID != "" && !idRe.MatchString(r.AgentID) {
		return errors.New("agentId invalido")
	}
	if r.OriginType == OriginAgent && r.Mode != ModeHTTP {
		return errors.New("TCP/UDP mediante cliente de sistema requiere la fase de streams remotos")
	}
	if r.DisabledResponseMode != DisabledResponse403 && r.DisabledResponseMode != DisabledResponse404 && r.DisabledResponseMode != DisabledResponseHTML {
		return errors.New("disabledResponseMode debe ser 403, 404 o html")
	}
	if r.DisabledResponseMode == DisabledResponse403 {
		r.DisabledStatusCode = 403
	}
	if r.DisabledResponseMode == DisabledResponse404 {
		r.DisabledStatusCode = 404
	}
	if r.DisabledResponseMode == DisabledResponseHTML {
		if r.DisabledStatusCode != 403 && r.DisabledStatusCode != 404 && r.DisabledStatusCode != 200 {
			return errors.New("disabledStatusCode para html debe ser 200, 403 o 404")
		}
		if len(r.DisabledHTML) > 131072 {
			return errors.New("disabledHtml no debe superar 128 KB")
		}
	}
	if r.Mode == ModeHTTP {
		if !domainRe.MatchString(r.Domain) {
			return errors.New("domain invalido para recurso HTTP")
		}
		if r.BackendScheme != "http" && r.BackendScheme != "https" {
			return errors.New("backendScheme debe ser http o https")
		}
		if !strings.HasPrefix(r.PathPrefix, "/") {
			return errors.New("pathPrefix debe iniciar con /")
		}
	}
	if r.Mode == ModeTCP || r.Mode == ModeUDP {
		if r.PublicPort < 1 || r.PublicPort > 65535 {
			return errors.New("publicPort debe estar entre 1 y 65535")
		}
		if r.PublicPort == 80 || r.PublicPort == 443 || r.PublicPort == 2424 {
			return errors.New("publicPort 80/443/2424 quedan reservados para HTTP/HTTPS y el panel Pangolite")
		}
	}
	return nil
}

func (r Resource) UsesAgent() bool {
	return r.OriginType == OriginAgent && r.AgentID != ""
}

func (r Resource) ServiceURL() string {
	return fmt.Sprintf("%s://%s:%d", r.BackendScheme, r.BackendHost, r.BackendPort)
}

func (r Resource) ServiceAddress() string {
	return fmt.Sprintf("%s:%d", r.BackendHost, r.BackendPort)
}

func (a *Agent) Normalize(now time.Time) {
	a.ProjectID = strings.TrimSpace(a.ProjectID)
	if a.ProjectID == "" {
		a.ProjectID = "default"
	}
	a.Name = strings.TrimSpace(a.Name)
	a.ID = strings.TrimSpace(a.ID)
	a.Token = strings.TrimSpace(a.Token)
	a.TokenHash = strings.TrimSpace(a.TokenHash)
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now.UTC()
	}
	a.UpdatedAt = now.UTC()
}

func (a Agent) Validate() error {
	if a.ID != "" && !idRe.MatchString(a.ID) {
		return errors.New("id de agente invalido")
	}
	if a.ProjectID == "" || !idRe.MatchString(a.ProjectID) {
		return errors.New("projectId de agente invalido")
	}
	if a.Name == "" || len(a.Name) > 120 {
		return errors.New("name de agente requerido y menor a 120 caracteres")
	}
	if a.Token != "" && len(a.Token) < 24 {
		return errors.New("token de agente demasiado corto")
	}
	if a.TokenHash == "" && a.Token == "" {
		return errors.New("token de agente requerido")
	}
	return nil
}

func (a Agent) Public() AgentPublic {
	return AgentPublic{ProjectID: a.ProjectID, ID: a.ID, Name: a.Name, Enabled: a.Enabled, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, LastSeen: a.LastSeen}
}

func NormalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func ValidateUsername(username string) error {
	if !usernameRe.MatchString(username) {
		return errors.New("usuario invalido: usa 3-64 caracteres alfanumericos, punto, guion o guion bajo")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 6 {
		return errors.New("la contraseña debe tener al menos 6 caracteres")
	}
	if len(password) > 256 {
		return errors.New("la contraseña es demasiado larga")
	}
	return nil
}
