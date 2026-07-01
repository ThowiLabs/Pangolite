package app

import (
	"errors"
	"fmt"
	"net"
	"net/url"
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

	DisabledResponse403    = "403"
	DisabledResponse404    = "404"
	DisabledResponseHidden = "hidden"
	DisabledResponseHTML   = "html"

	ProtectionNone     = "none"
	ProtectionPassword = "password"
	ProtectionSession  = "session"

	RedirectStatusMovedPermanently = 301
	RedirectStatusPermanent        = 308

	ProtectionLoginHTML  = "html"
	ProtectionLoginBasic = "basic"

	DomainStatusActive = "active"
	DomainStatusLegacy = "legacy"

	SMTPSecurityStartTLS = "starttls"
	SMTPSecurityTLS      = "tls"
	SMTPSecurityNone     = "none"
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
	ID               string    `json:"id"`
	Domain           string    `json:"domain"`
	Enabled          bool      `json:"enabled"`
	Status           string    `json:"status"`
	Primary          bool      `json:"primary"`
	AgentCount       int       `json:"agentCount"`
	ResourceCount    int       `json:"resourceCount"`
	UnsafeAgentCount int       `json:"unsafeAgentCount"`
	DeleteLocked     bool      `json:"deleteLocked"`
	DeleteReason     string    `json:"deleteReason,omitempty"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}

type DomainUsage struct {
	Agents    int `json:"agents"`
	Resources int `json:"resources"`
}

func (u DomainUsage) Total() int {
	return u.Agents + u.Resources
}

type AppSettings struct {
	DashboardDomain     string `json:"dashboardDomain"`
	LetsEncryptEmail    string `json:"letsEncryptEmail"`
	BackupIntervalHours int    `json:"backupIntervalHours"`
	BackupRetentionDays int    `json:"backupRetentionDays"`
	SMTPEnabled         bool   `json:"smtpEnabled"`
	SMTPHost            string `json:"smtpHost"`
	SMTPPort            int    `json:"smtpPort"`
	SMTPSecurity        string `json:"smtpSecurity"`
	SMTPUsername        string `json:"smtpUsername"`
	SMTPPassword        string `json:"-"`
	SMTPPasswordSet     bool   `json:"smtpPasswordSet"`
	SMTPFromEmail       string `json:"smtpFromEmail"`
	SMTPFromName        string `json:"smtpFromName"`
}

type Resource struct {
	ProjectID            string    `json:"projectId"`
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Mode                 string    `json:"mode"`
	Domain               string    `json:"domain,omitempty"`
	PathPrefix           string    `json:"pathPrefix,omitempty"`
	PublicPort           int       `json:"publicPort,omitempty"`
	TunnelPort           int       `json:"tunnelPort,omitempty"`
	BackendScheme        string    `json:"backendScheme,omitempty"`
	BackendHost          string    `json:"backendHost"`
	BackendPort          int       `json:"backendPort"`
	OriginType           string    `json:"originType"`
	AgentID              string    `json:"agentId,omitempty"`
	TLS                  bool      `json:"tls"`
	RedirectEnabled      bool      `json:"redirectEnabled"`
	RedirectTarget       string    `json:"redirectTarget,omitempty"`
	RedirectStatusCode   int       `json:"redirectStatusCode,omitempty"`
	HideWhenUnavailable  bool      `json:"hideWhenUnavailable"`
	Enabled              bool      `json:"enabled"`
	DisabledResponseMode string    `json:"disabledResponseMode"`
	DisabledStatusCode   int       `json:"disabledStatusCode"`
	DisabledHTML         string    `json:"disabledHtml,omitempty"`
	DisabledTemplateID   string    `json:"disabledTemplateId,omitempty"`
	ProtectionMode       string    `json:"protectionMode"`
	ProtectionLoginMode  string    `json:"protectionLoginMode"`
	ProtectionHash       string    `json:"-"`
	ProtectionPassword   string    `json:"protectionPassword,omitempty"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

type Agent struct {
	ProjectID             string    `json:"projectId"`
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	Token                 string    `json:"token,omitempty"`
	TokenHash             string    `json:"-"`
	ServerURL             string    `json:"serverUrl,omitempty"`
	FallbackURL           string    `json:"fallbackUrl,omitempty"`
	FallbackConfirmedAt   time.Time `json:"fallbackConfirmedAt,omitempty"`
	FallbackReady         bool      `json:"fallbackReady"`
	DomainID              string    `json:"domainId,omitempty"`
	Enabled               bool      `json:"enabled"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
	LastSeen              time.Time `json:"lastSeen,omitempty"`
	OS                    string    `json:"os,omitempty"`
	Arch                  string    `json:"arch,omitempty"`
	Hostname              string    `json:"hostname,omitempty"`
	PublicIP              string    `json:"publicIp,omitempty"`
	PrivateIP             string    `json:"privateIp,omitempty"`
	Version               string    `json:"version,omitempty"`
	LastError             string    `json:"lastError,omitempty"`
	Online                bool      `json:"online"`
	ResourceCount         int       `json:"resourceCount"`
	WebResourceCount      int       `json:"webResourceCount"`
	WebEnabledCount       int       `json:"webEnabledCount"`
	WebSuspendedCount     int       `json:"webSuspendedCount"`
	TCPResourceCount      int       `json:"tcpResourceCount"`
	TCPEnabledCount       int       `json:"tcpEnabledCount"`
	TCPSuspendedCount     int       `json:"tcpSuspendedCount"`
	UDPResourceCount      int       `json:"udpResourceCount"`
	UDPEnabledCount       int       `json:"udpEnabledCount"`
	UDPSuspendedCount     int       `json:"udpSuspendedCount"`
	WebMaintenanceActive  bool      `json:"webMaintenanceActive"`
	MaintenanceActive     bool      `json:"maintenanceActive"`
	InstallCommand        string    `json:"installCommand,omitempty"`
	RemoveCommand         string    `json:"removeCommand,omitempty"`
	WindowsInstallCommand string    `json:"windowsInstallCommand,omitempty"`
	WindowsRemoveCommand  string    `json:"windowsRemoveCommand,omitempty"`
}

type AgentPublic struct {
	ProjectID             string    `json:"projectId"`
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	ServerURL             string    `json:"serverUrl,omitempty"`
	FallbackURL           string    `json:"fallbackUrl,omitempty"`
	FallbackConfirmedAt   time.Time `json:"fallbackConfirmedAt,omitempty"`
	FallbackReady         bool      `json:"fallbackReady"`
	DomainID              string    `json:"domainId,omitempty"`
	Enabled               bool      `json:"enabled"`
	CreatedAt             time.Time `json:"createdAt"`
	UpdatedAt             time.Time `json:"updatedAt"`
	LastSeen              time.Time `json:"lastSeen,omitempty"`
	OS                    string    `json:"os,omitempty"`
	Arch                  string    `json:"arch,omitempty"`
	Hostname              string    `json:"hostname,omitempty"`
	PublicIP              string    `json:"publicIp,omitempty"`
	PrivateIP             string    `json:"privateIp,omitempty"`
	Version               string    `json:"version,omitempty"`
	LastError             string    `json:"lastError,omitempty"`
	Online                bool      `json:"online"`
	ResourceCount         int       `json:"resourceCount"`
	WebResourceCount      int       `json:"webResourceCount"`
	WebEnabledCount       int       `json:"webEnabledCount"`
	WebSuspendedCount     int       `json:"webSuspendedCount"`
	TCPResourceCount      int       `json:"tcpResourceCount"`
	TCPEnabledCount       int       `json:"tcpEnabledCount"`
	TCPSuspendedCount     int       `json:"tcpSuspendedCount"`
	UDPResourceCount      int       `json:"udpResourceCount"`
	UDPEnabledCount       int       `json:"udpEnabledCount"`
	UDPSuspendedCount     int       `json:"udpSuspendedCount"`
	WebMaintenanceActive  bool      `json:"webMaintenanceActive"`
	MaintenanceActive     bool      `json:"maintenanceActive"`
	InstallCommand        string    `json:"installCommand,omitempty"`
	RemoveCommand         string    `json:"removeCommand,omitempty"`
	WindowsInstallCommand string    `json:"windowsInstallCommand,omitempty"`
	WindowsRemoveCommand  string    `json:"windowsRemoveCommand,omitempty"`
}

type User struct {
	ID                  int64     `json:"id"`
	Username            string    `json:"username"`
	Email               string    `json:"email"`
	PasswordHash        string    `json:"-"`
	ForcePasswordChange bool      `json:"forcePasswordChange"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type AgentDiscovery struct {
	ServerURL   string `json:"serverUrl"`
	FallbackURL string `json:"fallbackUrl,omitempty"`
	Domain      string `json:"domain,omitempty"`
	PublicIP    string `json:"publicIp,omitempty"`
}

type AgentHeartbeat struct {
	OS          string
	Arch        string
	Hostname    string
	PublicIP    string
	PrivateIP   string
	Version     string
	LastError   string
	ServerURL   string
	FallbackURL string
}

type ResourceHealth struct {
	ResourceID string    `json:"resourceId"`
	Name       string    `json:"name"`
	Mode       string    `json:"mode"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	StatusCode int       `json:"statusCode,omitempty"`
	LatencyMS  int64     `json:"latencyMs"`
	CheckedAt  time.Time `json:"checkedAt"`
}

type Session struct {
	IDHash    string
	UserID    int64
	CSRFToken string
	ExpiresAt time.Time
	CreatedAt time.Time
	LastSeen  time.Time
}

type PasswordResetToken struct {
	ID        int64
	TokenHash string
	UserID    int64
	ExpiresAt time.Time
	UsedAt    time.Time
	CreatedAt time.Time
}

var (
	domainRe     = regexp.MustCompile(`(?i)^([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)
	idRe         = regexp.MustCompile(`^[a-zA-Z0-9_-]{6,64}$`)
	usernameRe   = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,64}$`)
	slugRe       = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
	emailRe      = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	templateIDRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,62}[a-z0-9]$`)
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
		return "proyecto"
	}
	return out
}

func (d *ManagedDomain) Normalize(now time.Time) {
	d.Domain = strings.ToLower(strings.TrimSpace(d.Domain))
	d.Status = strings.ToLower(strings.TrimSpace(d.Status))
	if d.Status == "" {
		d.Status = DomainStatusActive
	}
	d.Enabled = d.Status == DomainStatusActive
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
	switch d.Status {
	case "", DomainStatusActive, DomainStatusLegacy:
		return nil
	default:
		return errors.New("estado de dominio invalido")
	}
}

func (a *AppSettings) Normalize() {
	a.DashboardDomain = strings.ToLower(strings.TrimSpace(a.DashboardDomain))
	a.LetsEncryptEmail = strings.ToLower(strings.TrimSpace(a.LetsEncryptEmail))
	a.SMTPHost = strings.TrimSpace(a.SMTPHost)
	a.SMTPSecurity = strings.ToLower(strings.TrimSpace(a.SMTPSecurity))
	a.SMTPUsername = strings.TrimSpace(a.SMTPUsername)
	a.SMTPPassword = strings.TrimSpace(a.SMTPPassword)
	a.SMTPFromEmail = strings.ToLower(strings.TrimSpace(a.SMTPFromEmail))
	a.SMTPFromName = strings.TrimSpace(a.SMTPFromName)
	if a.SMTPSecurity == "" {
		a.SMTPSecurity = SMTPSecurityStartTLS
	}
	if a.SMTPPort == 0 {
		switch a.SMTPSecurity {
		case SMTPSecurityTLS:
			a.SMTPPort = 465
		case SMTPSecurityNone:
			a.SMTPPort = 25
		default:
			a.SMTPPort = 587
		}
	}
	if a.BackupIntervalHours < 0 {
		a.BackupIntervalHours = 0
	}
	if a.BackupRetentionDays < 0 {
		a.BackupRetentionDays = 0
	}
}

func (a AppSettings) Validate() error {
	if a.DashboardDomain != "" {
		if !domainRe.MatchString(a.DashboardDomain) {
			return errors.New("dominio del panel invalido")
		}
		if strings.HasPrefix(a.DashboardDomain, "localhost") || strings.HasSuffix(a.DashboardDomain, ".localhost") || strings.HasSuffix(a.DashboardDomain, ".local") {
			return errors.New("usa un dominio publico real para el panel")
		}
		if a.LetsEncryptEmail == "" {
			return errors.New("correo ACME requerido cuando configuras dominio del panel")
		}
	}
	if a.LetsEncryptEmail != "" && !emailRe.MatchString(a.LetsEncryptEmail) {
		return errors.New("correo ACME invalido")
	}
	if err := a.ValidateSMTP(false); err != nil {
		return err
	}
	if a.BackupIntervalHours > 0 && a.BackupIntervalHours < 1 {
		return errors.New("intervalo de respaldos invalido")
	}
	if a.BackupIntervalHours > 720 {
		return errors.New("intervalo de respaldos demasiado alto")
	}
	if a.BackupRetentionDays > 3650 {
		return errors.New("retencion de respaldos demasiado alta")
	}
	return nil
}

func (a AppSettings) ValidateSMTP(requirePassword bool) error {
	if !a.SMTPEnabled {
		return nil
	}
	if a.SMTPHost == "" {
		return errors.New("host SMTP requerido")
	}
	if strings.ContainsAny(a.SMTPHost, "/\\ \t\r\n") {
		return errors.New("host SMTP invalido")
	}
	if a.SMTPPort < 1 || a.SMTPPort > 65535 {
		return errors.New("puerto SMTP invalido")
	}
	switch a.SMTPSecurity {
	case SMTPSecurityStartTLS, SMTPSecurityTLS, SMTPSecurityNone:
	default:
		return errors.New("seguridad SMTP invalida")
	}
	if a.SMTPFromEmail == "" || !emailRe.MatchString(a.SMTPFromEmail) {
		return errors.New("correo remitente SMTP invalido")
	}
	if len(a.SMTPFromName) > 120 {
		return errors.New("nombre remitente SMTP demasiado largo")
	}
	if requirePassword && a.SMTPUsername != "" && a.SMTPPassword == "" {
		return errors.New("contraseña SMTP requerida para validar autenticacion")
	}
	return nil
}

func (a AppSettings) SMTPReady() bool {
	return a.SMTPEnabled && a.ValidateSMTP(false) == nil && (a.SMTPUsername == "" || a.SMTPPasswordSet)
}

func ValidateEmailAddress(email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return errors.New("correo requerido")
	}
	if len(email) > 254 || !emailRe.MatchString(email) {
		return errors.New("correo invalido")
	}
	return nil
}

func (r *Resource) Normalize(now time.Time) {
	r.ProjectID = strings.TrimSpace(r.ProjectID)
	r.Name = strings.TrimSpace(r.Name)
	r.Mode = strings.ToLower(strings.TrimSpace(r.Mode))
	r.Domain = strings.ToLower(strings.TrimSpace(r.Domain))
	r.PathPrefix = strings.TrimSpace(r.PathPrefix)
	r.BackendScheme = strings.ToLower(strings.TrimSpace(r.BackendScheme))
	r.BackendHost = strings.TrimSpace(r.BackendHost)
	r.RedirectTarget = strings.TrimSpace(r.RedirectTarget)
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
	r.DisabledTemplateID = strings.TrimSpace(r.DisabledTemplateID)
	r.ProtectionMode = strings.ToLower(strings.TrimSpace(r.ProtectionMode))
	r.ProtectionPassword = strings.TrimSpace(r.ProtectionPassword)
	r.ProtectionLoginMode = strings.ToLower(strings.TrimSpace(r.ProtectionLoginMode))
	if r.ProtectionMode == "" {
		r.ProtectionMode = ProtectionNone
	}
	if r.ProtectionLoginMode == "" {
		r.ProtectionLoginMode = ProtectionLoginHTML
	}
	if r.Mode != ModeHTTP {
		r.ProtectionMode = ProtectionNone
		r.ProtectionLoginMode = ProtectionLoginHTML
		r.ProtectionHash = ""
		r.RedirectEnabled = false
		r.RedirectTarget = ""
		r.RedirectStatusCode = 0
		r.HideWhenUnavailable = false
	}
	if r.DisabledResponseMode == "" {
		r.DisabledResponseMode = DisabledResponse403
	}
	if r.DisabledStatusCode == 0 {
		r.DisabledStatusCode = 403
	}
	if r.BackendScheme == "" && r.Mode == ModeHTTP {
		r.BackendScheme = "http"
	}
	if r.RedirectStatusCode == 0 {
		r.RedirectStatusCode = RedirectStatusPermanent
	}
	if r.RedirectEnabled && r.Mode == ModeHTTP {
		r.OriginType = OriginLocal
		r.AgentID = ""
		r.HideWhenUnavailable = false
		r.ProtectionMode = ProtectionNone
		r.ProtectionLoginMode = ProtectionLoginHTML
		r.ProtectionHash = ""
		if r.BackendScheme == "" {
			r.BackendScheme = "http"
		}
		if r.BackendHost == "" {
			r.BackendHost = "127.0.0.1"
		}
		if r.BackendPort == 0 {
			r.BackendPort = 80
		}
	}
	if !r.RedirectEnabled {
		r.RedirectTarget = ""
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
	if r.OriginType == OriginAgent && (r.Mode == ModeTCP || r.Mode == ModeUDP) && r.TunnelPort != 0 {
		if r.TunnelPort < 1024 || r.TunnelPort > 65535 {
			return errors.New("tunnelPort interno invalido")
		}
	}
	if r.DisabledResponseMode != DisabledResponse403 && r.DisabledResponseMode != DisabledResponse404 && r.DisabledResponseMode != DisabledResponseHidden && r.DisabledResponseMode != DisabledResponseHTML {
		return errors.New("disabledResponseMode debe ser 403, 404, hidden o html")
	}
	if r.DisabledTemplateID != "" && !templateIDRe.MatchString(r.DisabledTemplateID) {
		return errors.New("disabledTemplateId invalido")
	}
	if r.DisabledResponseMode == DisabledResponse403 {
		r.DisabledStatusCode = 403
	}
	if r.DisabledResponseMode == DisabledResponse404 {
		r.DisabledStatusCode = 404
	}
	if r.DisabledResponseMode == DisabledResponseHidden {
		r.DisabledStatusCode = 404
		r.DisabledHTML = ""
		r.DisabledTemplateID = ""
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
		if len(r.PathPrefix) > 200 || strings.ContainsAny(r.PathPrefix, "`\n\r\t") || strings.Contains(r.PathPrefix, " ") {
			return errors.New("pathPrefix contiene caracteres no permitidos")
		}
		if r.ProtectionMode != ProtectionNone && r.ProtectionMode != ProtectionPassword && r.ProtectionMode != ProtectionSession {
			return errors.New("protectionMode debe ser none, password o session")
		}
		if r.ProtectionLoginMode != ProtectionLoginHTML && r.ProtectionLoginMode != ProtectionLoginBasic {
			return errors.New("protectionLoginMode debe ser html o basic")
		}
		if r.ProtectionMode == ProtectionPassword && r.ProtectionHash == "" {
			return errors.New("password de proteccion requerido")
		}
		if !domainRe.MatchString(r.Domain) {
			return errors.New("domain invalido para recurso HTTP")
		}
		if r.BackendScheme != "http" && r.BackendScheme != "https" {
			return errors.New("backendScheme debe ser http o https")
		}
		if r.RedirectEnabled {
			if err := validateRedirectTarget(r.RedirectTarget); err != nil {
				return err
			}
			if strings.EqualFold(redirectTargetHost(r.RedirectTarget), r.Domain) {
				return errors.New("redirectTarget no puede ser el mismo dominio del recurso")
			}
			if r.RedirectStatusCode != RedirectStatusMovedPermanently && r.RedirectStatusCode != RedirectStatusPermanent {
				return errors.New("redirectStatusCode debe ser 301 o 308")
			}
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

func validateRedirectTarget(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return errors.New("redirectTarget requerido cuando la redireccion permanente esta activa")
	}
	if len(target) > 2048 || strings.ContainsAny(target, "`\n\r\t") {
		return errors.New("redirectTarget contiene caracteres no permitidos")
	}
	if strings.HasPrefix(strings.ToLower(target), "http://") || strings.HasPrefix(strings.ToLower(target), "https://") {
		u, err := url.Parse(target)
		if err != nil || u.Scheme == "" || u.Hostname() == "" {
			return errors.New("redirectTarget debe apuntar a un dominio o URL http/https valida")
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return errors.New("redirectTarget solo puede usar http o https")
		}
		host := strings.ToLower(strings.TrimSpace(u.Hostname()))
		if !domainRe.MatchString(host) && net.ParseIP(host) == nil {
			return errors.New("redirectTarget debe apuntar a un dominio o URL http/https valida")
		}
		return nil
	}
	if strings.ContainsAny(target, "/?# ") {
		return errors.New("si redirectTarget incluye ruta usa una URL completa http:// o https://")
	}
	if !domainRe.MatchString(strings.ToLower(target)) {
		return errors.New("redirectTarget debe ser un dominio valido o una URL http/https")
	}
	return nil
}

func redirectTargetHost(target string) string {
	target = strings.TrimSpace(target)
	if target == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(target), "http://") || strings.HasPrefix(strings.ToLower(target), "https://") {
		u, err := url.Parse(target)
		if err != nil {
			return ""
		}
		return strings.ToLower(strings.TrimSpace(u.Hostname()))
	}
	return strings.ToLower(target)
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

func (r Resource) BridgeAddress() string {
	if r.TunnelPort <= 0 {
		return ""
	}
	return fmt.Sprintf("127.0.0.1:%d", r.TunnelPort)
}

func (a *Agent) Normalize(now time.Time) {
	a.ProjectID = strings.TrimSpace(a.ProjectID)
	a.Name = strings.TrimSpace(a.Name)
	a.ID = strings.TrimSpace(a.ID)
	a.Token = strings.TrimSpace(a.Token)
	a.TokenHash = strings.TrimSpace(a.TokenHash)
	a.ServerURL = strings.TrimRight(strings.TrimSpace(a.ServerURL), "/")
	a.FallbackURL = strings.TrimRight(strings.TrimSpace(a.FallbackURL), "/")
	a.DomainID = strings.TrimSpace(a.DomainID)
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
	if a.DomainID != "" && !idRe.MatchString(a.DomainID) {
		return errors.New("id de dominio de agente invalido")
	}
	if len(a.ServerURL) > 500 {
		return errors.New("url de servidor demasiado larga")
	}
	if len(a.FallbackURL) > 500 {
		return errors.New("url fallback demasiado larga")
	}
	return nil
}

func (a Agent) Public() AgentPublic {
	fallbackReady := a.FallbackURL != "" && !a.FallbackConfirmedAt.IsZero()
	return AgentPublic{ProjectID: a.ProjectID, ID: a.ID, Name: a.Name, ServerURL: a.ServerURL, FallbackURL: a.FallbackURL, FallbackConfirmedAt: a.FallbackConfirmedAt, FallbackReady: fallbackReady, DomainID: a.DomainID, Enabled: a.Enabled, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, LastSeen: a.LastSeen, OS: a.OS, Arch: a.Arch, Hostname: a.Hostname, PublicIP: a.PublicIP, PrivateIP: a.PrivateIP, Version: a.Version, LastError: a.LastError, Online: a.Online, ResourceCount: a.ResourceCount}
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
