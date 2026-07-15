package app

import (
	"bytes"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

type uiPageData struct {
	Title               string
	Heading             string
	Subtitle            string
	ScriptPath          string
	Path                string
	PageKey             string
	Crumb               string
	PageHeading         string
	User                User
	Username            string
	CSRFToken           string
	Projects            []Project
	Stats               map[string]map[string]int
	CurrentID           string
	CurrentProject      Project
	HasProject          bool
	Resources           []Resource
	Agents              []AgentPublic
	Domains             []ManagedDomain
	Settings            AppSettings
	Network             NetworkInfo
	ServerHostname      string
	ServerOS            string
	ServerArch          string
	Certificate         CertificateStatus
	AuditEvents         []AuditEvent
	Backups             []BackupInfo
	BackupDir           string
	LogLines            []string
	LogPath             string
	MaxLogEntries       int
	TraefikConfig       string
	SuspensionTemplates []SuspensionTemplate
	BootstrapJSON       template.JS
}

type panelPage struct {
	Template string
	Title    string
	Key      string
	Crumb    string
	Heading  string
}

func (s *Server) panelData(r *http.Request, rs requestSession, page panelPage) uiPageData {
	settings := s.store.LoadAppSettings(s.config)
	effective := s.store.EffectiveConfig(s.config)
	network := DetectNetworkInfo(s.config.PublicIP, settings.DashboardDomain)
	certificate := ResolveCertificateStatus(effective, settings.DashboardDomain, settings.DashboardDomain != "")
	projects := s.store.ListProjects()
	stats := s.store.ProjectStats()
	currentID := s.currentProjectIDFromRequest(r)
	hostname, _ := os.Hostname()
	data := uiPageData{
		Title:          page.Title,
		Path:           r.URL.Path,
		PageKey:        page.Key,
		Crumb:          page.Crumb,
		PageHeading:    page.Heading,
		User:           rs.User,
		Username:       rs.User.Username,
		CSRFToken:      rs.Session.CSRFToken,
		Projects:       projects,
		Stats:          stats,
		CurrentID:      currentID,
		Domains:        s.store.ListManagedDomains(),
		Settings:       settings,
		Network:        network,
		ServerHostname: strings.TrimSpace(hostname),
		ServerOS:       runtime.GOOS,
		ServerArch:     runtime.GOARCH,
		Certificate:    certificate,
		BackupDir:      s.config.BackupDir,
		LogPath:        s.config.LogPath,
		MaxLogEntries:  defaultMaxLogLines,
	}
	if templates, err := ListSuspensionTemplates(s.config.SuspensionTemplateDir); err == nil {
		data.SuspensionTemplates = templates
	}
	if currentID != "" {
		if p, err := s.store.ProjectByID(currentID); err == nil {
			data.CurrentProject = p
			data.HasProject = true
			data.Crumb = p.Name
			data.Resources = s.store.ListResourcesByProject(currentID)
			data.Agents = s.store.ListAgentsByProject(currentID)
		}
	}
	if page.Key == "maintenance" {
		if events, err := s.store.ListAuditEvents(defaultAuditLimit, ""); err == nil {
			data.AuditEvents = events
		}
		if backups, err := ListBackups(s.config.BackupDir); err == nil {
			data.Backups = backups
		}
	}
	if page.Key == "logs" {
		if lines, err := ReadLastLogLines(s.config.LogPath, 300); err == nil {
			data.LogLines = lines
		}
	}
	if page.Key == "settings" {
		if b, err := EncodeTraefikJSON(s.store.ListResources()); err == nil {
			data.TraefikConfig = string(b)
		}
	}
	if (page.Key == "terminal" && currentID == "") || page.Key == "ssh_connections" {
		data.Agents = s.store.ListAgents()
	}
	data.BootstrapJSON = template.JS(mustJSON(map[string]any{
		"csrfToken":           data.CSRFToken,
		"user":                publicUser(rs.User),
		"projects":            data.Projects,
		"stats":               data.Stats,
		"currentProject":      data.CurrentProject,
		"hasProject":          data.HasProject,
		"resources":           data.Resources,
		"agents":              data.Agents,
		"domains":             data.Domains,
		"settings":            data.Settings,
		"network":             data.Network,
		"certificate":         data.Certificate,
		"auditEvents":         data.AuditEvents,
		"backups":             data.Backups,
		"backupDir":           data.BackupDir,
		"logLines":            data.LogLines,
		"logPath":             data.LogPath,
		"maxLogEntries":       data.MaxLogEntries,
		"traefikConfig":       data.TraefikConfig,
		"suspensionTemplates": data.SuspensionTemplates,
		"serverOS":            data.ServerOS,
		"serverArch":          data.ServerArch,
		"serverHostname":      data.ServerHostname,
		"path":                data.Path,
		"pageKey":             data.PageKey,
	}))
	return data
}

func (s *Server) projectIDFromRequest(r *http.Request) string {
	return s.currentProjectIDFromRequest(r)
}

func (s *Server) currentProjectIDFromRequest(r *http.Request) string {
	if id := projectIDFromPath(r.URL.Path); id != "" {
		if resolvedID, err := s.store.ResolveProjectID(id); err == nil {
			return resolvedID
		}
		return ""
	}
	if r.URL.Path != "/terminal" {
		return ""
	}
	if id := strings.TrimSpace(r.URL.Query().Get("projectId")); id != "" {
		if resolvedID, err := s.store.ResolveProjectID(id); err == nil {
			return resolvedID
		}
	}
	if agentID := strings.TrimSpace(r.URL.Query().Get("agentId")); agentID != "" {
		if agent, err := s.store.AgentByID(agentID); err == nil {
			return agent.ProjectID
		}
	}
	return ""
}

func projectIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "projects" {
		return parts[1]
	}
	return ""
}

func mustJSON(v any) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		return "{}"
	}
	return strings.TrimSpace(b.String())
}

func renderUIPage(w http.ResponseWriter, page string, data uiPageData) {
	funcs := template.FuncMap{
		"projectStats":         projectStats,
		"totalStat":            totalStat,
		"fmtTime":              fmtTemplateTime,
		"fmtBytes":             fmtBytes,
		"resourceEntry":        resourceEntry,
		"resourceBackend":      resourceBackend,
		"resourceOrigin":       resourceOrigin,
		"resourceAgentName":    resourceAgentName(data.Agents),
		"resourceRowClass":     resourceRowClass,
		"resourceStatusText":   resourceStatusText,
		"resourceModeLabel":    resourceModeLabel,
		"agentStateText":       agentStateText,
		"agentStateClass":      agentStateClass,
		"agentSystem":          agentSystem,
		"projectName":          projectName(data.Projects),
		"projectSlug":          projectSlug(data.Projects),
		"connectionTotal":      connectionTotal,
		"availableConnections": availableConnections,
		"agentTerminalReady":   agentTerminalReady,
		"agentTerminalState":   agentTerminalState,
		"agentTerminalClass":   agentTerminalClass,
		"domainStateText":      domainStateText,
		"dnsStateText":         dnsStateText,
		"dnsStateClass":        dnsStateClass,
		"certText":             certTextTemplate,
		"certClass":            certClassTemplate,
		"logLineClass":         logLineClass,
	}
	t, err := template.New("ui").Funcs(funcs).ParseFS(templatesFS, "templates/layouts/*.html", "templates/components/*.html", "templates/pages/"+page)
	if err != nil {
		http.Error(w, "plantilla no disponible", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "page", data); err != nil {
		http.Error(w, "no se pudo renderizar la plantilla", http.StatusInternalServerError)
	}
}

func projectStats(stats map[string]map[string]int, id string, key string) int {
	if stats == nil || stats[id] == nil {
		return 0
	}
	return stats[id][key]
}

func totalStat(stats map[string]map[string]int, key string) int {
	total := 0
	for _, st := range stats {
		total += st[key]
	}
	return total
}

func fmtTemplateTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func fmtBytes(size int64) string {
	if size < 1024 {
		return "< 1 KB"
	}
	if size < 1024*1024 {
		return strings.TrimRight(strings.TrimRight(templateFloat(float64(size)/1024), "0"), ".") + " KB"
	}
	return strings.TrimRight(strings.TrimRight(templateFloat(float64(size)/(1024*1024)), "0"), ".") + " MB"
}

func templateFloat(v float64) string {
	return strings.TrimRight(strings.TrimRight(jsonNumber(v), "0"), ".")
}

func jsonNumber(v float64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func resourceEntry(r Resource) string {
	if r.Mode == ModeHTTP {
		scheme := "http://"
		if r.TLS {
			scheme = "https://"
		}
		path := r.PathPrefix
		if path == "" {
			path = "/"
		}
		return scheme + r.Domain + path
	}
	return strings.ToUpper(r.Mode) + " :" + intToString(r.PublicPort)
}

func resourceBackend(r Resource) string {
	if r.Mode == ModeHTTP && r.RedirectEnabled {
		return "redirect -> " + r.RedirectTarget
	}
	prefix := ""
	if r.Mode == ModeHTTP && r.BackendScheme != "" {
		prefix = r.BackendScheme + "://"
	}
	return prefix + r.BackendHost + ":" + intToString(r.BackendPort)
}

func resourceOrigin(r Resource) string {
	if r.OriginType == OriginAgent {
		return "Remoto"
	}
	return "Local"
}

func resourceAgentName(agents []AgentPublic) func(string) string {
	byID := map[string]string{}
	for _, a := range agents {
		byID[a.ID] = a.Name
	}
	return func(id string) string {
		if id == "" {
			return "-"
		}
		if name := byID[id]; name != "" {
			return name
		}
		return id
	}
}

func resourceRowClass(r Resource) string {
	if r.Enabled {
		return "resource-row is-active"
	}
	return "resource-row is-suspended"
}

func resourceStatusText(r Resource) string {
	if r.Enabled {
		return "Activo"
	}
	return "Suspendido"
}

func resourceModeLabel(mode string) string {
	return strings.ToUpper(strings.TrimSpace(mode))
}

func agentStateText(a AgentPublic) string {
	if a.Online {
		return "Online"
	}
	if a.Enabled {
		return "Offline"
	}
	return "Inactivo"
}

func agentStateClass(a AgentPublic) string {
	if a.Online {
		return "on"
	}
	return "off"
}

func agentSystem(a AgentPublic) string {
	parts := []string{}
	if strings.TrimSpace(a.OS) != "" {
		parts = append(parts, a.OS)
	}
	if strings.TrimSpace(a.Arch) != "" {
		parts = append(parts, a.Arch)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "/")
}

func projectName(projects []Project) func(string) string {
	byID := make(map[string]string, len(projects))
	for _, project := range projects {
		byID[project.ID] = project.Name
	}
	return func(id string) string {
		if name := strings.TrimSpace(byID[id]); name != "" {
			return name
		}
		return "Proyecto no disponible"
	}
}

func projectSlug(projects []Project) func(string) string {
	byID := make(map[string]string, len(projects))
	for _, project := range projects {
		byID[project.ID] = project.Slug
	}
	return func(id string) string {
		return strings.TrimSpace(byID[id])
	}
}

func connectionTotal(agents []AgentPublic) int {
	return len(agents) + 1
}

func availableConnections(serverOS string, agents []AgentPublic) int {
	total := 0
	if !strings.EqualFold(strings.TrimSpace(serverOS), "windows") {
		total++
	}
	for _, agent := range agents {
		if agentTerminalReady(agent) {
			total++
		}
	}
	return total
}

func agentTerminalReady(a AgentPublic) bool {
	return a.Enabled && a.Online && !strings.EqualFold(strings.TrimSpace(a.OS), "windows")
}

func agentTerminalState(a AgentPublic) string {
	if !a.Enabled {
		return "Inactivo"
	}
	if strings.EqualFold(strings.TrimSpace(a.OS), "windows") {
		return "Windows no compatible"
	}
	if a.Online {
		return "Disponible"
	}
	return "Offline"
}

func agentTerminalClass(a AgentPublic) string {
	if agentTerminalReady(a) {
		return "on"
	}
	if strings.EqualFold(strings.TrimSpace(a.OS), "windows") {
		return "warning"
	}
	return "off"
}

func domainStateText(d ManagedDomain) string {
	if d.Primary {
		return "Principal"
	}
	switch d.Status {
	case DomainStatusLegacy:
		return "Heredado"
	case DomainStatusActive:
		return "Activo"
	default:
		if d.Enabled {
			return "Activo"
		}
		return "Inactivo"
	}
}

func dnsStateText(info NetworkInfo, domain string) string {
	if strings.TrimSpace(domain) == "" {
		return "Sin dominio"
	}
	if info.DNSMatchesServer {
		return "Correcto"
	}
	if len(info.DashboardDomainIPs) > 0 {
		return "No coincide"
	}
	return "Pendiente"
}

func dnsStateClass(info NetworkInfo, domain string) string {
	if strings.TrimSpace(domain) == "" {
		return "warn"
	}
	if info.DNSMatchesServer {
		return "good"
	}
	if len(info.DashboardDomainIPs) > 0 {
		return "bad"
	}
	return "warn"
}

func certTextTemplate(c CertificateStatus) string {
	switch c.Status {
	case "issued":
		return "Emitido"
	case "pending":
		return "Pendiente"
	case "disabled":
		return "Desactivado"
	case "missing_domain":
		return "Sin dominio"
	case "acme_disabled":
		return "ACME pendiente"
	case "unavailable":
		return "No disponible"
	default:
		if c.Status == "" {
			return "Sin revisar"
		}
		return c.Status
	}
}

func certClassTemplate(c CertificateStatus) string {
	switch c.Status {
	case "issued":
		return "good"
	case "pending", "disabled", "missing_domain", "acme_disabled":
		return "warn"
	case "unavailable":
		return "bad"
	default:
		return "warn"
	}
}

func logLineClass(line string) string {
	line = strings.ToUpper(line)
	classes := "log-line"
	if strings.Contains(line, "LEVEL=ERROR") || strings.Contains(line, " ERROR ") {
		return classes + " error"
	}
	if strings.Contains(line, "LEVEL=WARN") || strings.Contains(line, " WARN ") {
		return classes + " warn"
	}
	return classes
}

func intToString(v int) string {
	b, _ := json.Marshal(v)
	return string(b)
}
