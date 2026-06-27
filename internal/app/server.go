package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

const sessionCookieName = "pangolite_session"

type Server struct {
	config  Config
	store   *Store
	hub     *TunnelHub
	bridges *BridgeManager
	mux     *http.ServeMux
	log     *slog.Logger

	traefikRestartMu    sync.Mutex
	traefikRestartTimer *time.Timer
}

type requestSession struct {
	Session Session
	User    User
	RawID   string
}

func NewServer(c Config, store *Store, logger *slog.Logger) *Server {
	effective := store.EffectiveConfig(c)
	if err := store.EnsureManagedDomain(effective.DashboardDomain); err != nil && logger != nil {
		logger.Warn("no se pudo registrar dominio del panel", "domain", effective.DashboardDomain, "error", err.Error())
	}
	hub := NewTunnelHub(64)
	s := &Server{config: c, store: store, hub: hub, bridges: NewBridgeManager(hub, logger), mux: http.NewServeMux(), log: logger}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.recoverRequests(s.logRequests(s.mux)))
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.refreshBridgeListeners(); err != nil && s.log != nil {
		s.log.Warn("no se pudieron preparar puentes de clientes NAT", "error", err.Error())
	}
	defer s.bridges.Close()
	srv := &http.Server{Addr: s.config.Addr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1)
	go func() {
		s.log.Info("panel iniciado", "addr", s.config.Addr)
		err := srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
			return
		}
		errc <- nil
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errc:
		return err
	}
}

func (s *Server) routes() {
	if sub, err := fs.Sub(assetsFS, "assets"); err == nil {
		s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(sub))))
	}
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /login", s.loginPage)
	s.mux.HandleFunc("GET /password", s.passwordPage)
	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.HandleFunc("POST /api/logout", s.requireAuthAllowForce(s.logout))
	s.mux.HandleFunc("GET /api/session", s.sessionInfo)
	s.mux.HandleFunc("POST /api/password", s.requireAuthAllowForce(s.changePassword))
	s.mux.HandleFunc("GET /api/v1/traefik-config", s.traefikConfig)
	s.mux.HandleFunc("GET /api/projects", s.requireAuth(s.listProjects))
	s.mux.HandleFunc("POST /api/projects", s.requireAuth(s.createProject))
	s.mux.HandleFunc("PATCH /api/projects/{id}", s.requireAuth(s.updateProject))
	s.mux.HandleFunc("GET /api/settings", s.requireAuth(s.getSettings))
	s.mux.HandleFunc("PATCH /api/settings", s.requireAuth(s.updateSettings))
	s.mux.HandleFunc("GET /api/system/network", s.requireAuth(s.getNetworkInfo))
	s.mux.HandleFunc("GET /api/system/logs", s.requireAuth(s.getSystemLogs))
	s.mux.HandleFunc("GET /api/domains", s.requireAuth(s.listManagedDomains))
	s.mux.HandleFunc("POST /api/domains", s.requireAuth(s.createManagedDomain))
	s.mux.HandleFunc("DELETE /api/domains/{id}", s.requireAuth(s.deleteManagedDomain))
	s.mux.HandleFunc("GET /api/resources", s.requireAuth(s.listResources))
	s.mux.HandleFunc("GET /api/resources/health", s.requireAuth(s.resourceHealth))
	s.mux.HandleFunc("POST /api/resources", s.requireAuth(s.createResource))
	s.mux.HandleFunc("PATCH /api/resources/{id}", s.requireAuth(s.updateResourceControl))
	s.mux.HandleFunc("DELETE /api/resources/{id}", s.requireAuth(s.deleteResource))
	s.mux.HandleFunc("GET /api/agents", s.requireAuth(s.listAgents))
	s.mux.HandleFunc("GET /api/agents/{id}", s.requireAuth(s.getAgentDetail))
	s.mux.HandleFunc("POST /api/agents", s.requireAuth(s.createAgent))
	s.mux.HandleFunc("DELETE /api/agents/{id}", s.requireAuth(s.disableAgent))
	s.mux.HandleFunc("POST /api/agents/{id}/token", s.requireAuth(s.rotateAgentToken))
	s.mux.HandleFunc("POST /api/render-traefik", s.requireAuth(s.renderTraefik))
	s.mux.HandleFunc("POST /api/agent/poll", s.agentPoll)
	s.mux.HandleFunc("POST /api/agent/jobs/{id}/response", s.agentJobResponse)
	s.mux.HandleFunc("POST /api/agent/stream-poll", s.agentStreamPoll)
	s.mux.HandleFunc("GET /api/agent/streams/{id}", s.agentStreamSocket)
	s.mux.HandleFunc("GET /download/pangolite-client-linux-amd64", s.downloadClientLinuxAMD64)
	s.mux.HandleFunc("GET /download/pangolite-client-windows-amd64.exe", s.downloadClientWindowsAMD64)
	s.mux.HandleFunc("/", s.publicOrIndex)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentSession(r); ok {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	writeHTML(w, loginHTML)
}

func (s *Server) passwordPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentSession(r); !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	writeHTML(w, passwordHTML)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "JSON invalido")
			return
		}
	} else {
		if err := r.ParseForm(); err != nil {
			writeError(w, http.StatusBadRequest, "formulario invalido")
			return
		}
		req.Username = r.Form.Get("username")
		req.Password = r.Form.Get("password")
	}
	user, ok := s.store.AuthenticateUser(req.Username, req.Password)
	if !ok {
		s.log.Warn("login fallido", "user", NormalizeUsername(req.Username), "remote", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "usuario o contraseña invalidos")
		return
	}
	rawID, sess, err := s.store.CreateSession(user.ID, sessionDuration(s.config))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo crear sesion")
		return
	}
	s.setSessionCookie(w, r, rawID, sess.ExpiresAt)
	s.log.Info("login correcto", "user", user.Username)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": publicUser(user), "csrfToken": sess.CSRFToken})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request, rs requestSession) {
	s.store.DeleteSession(rs.RawID)
	s.clearSessionCookie(w, r)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) sessionInfo(w http.ResponseWriter, r *http.Request) {
	rs, ok := s.currentSession(r)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"authenticated": true, "user": publicUser(rs.User), "csrfToken": rs.Session.CSRFToken})
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	requireCurrent := !rs.User.ForcePasswordChange
	if err := s.store.ChangePassword(rs.User.ID, req.CurrentPassword, req.NewPassword, requireCurrent); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if rs.User.ForcePasswordChange && s.config.InitialPasswordFile != "" {
		if err := os.Remove(s.config.InitialPasswordFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("no se pudo eliminar password temporal", "path", s.config.InitialPasswordFile, "error", err.Error())
		}
	}
	s.log.Info("password actualizada", "user", rs.User.Username)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) traefikConfig(w http.ResponseWriter, _ *http.Request) {
	b, err := EncodeTraefikJSON(s.store.ListResources())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo generar config de Traefik")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write(b)
}

func (s *Server) listProjects(w http.ResponseWriter, _ *http.Request, _ requestSession) {
	writeJSON(w, http.StatusOK, map[string]any{"projects": s.store.ListProjects(), "stats": s.store.ProjectStats()})
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var req struct {
		Name  string `json:"name"`
		Slug  string `json:"slug"`
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	project, err := s.store.AddProject(Project{Name: req.Name, Slug: req.Slug, Notes: req.Notes})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Info("proyecto creado", "id", project.ID, "name", project.Name, "user", rs.User.Username)
	writeJSON(w, http.StatusCreated, project)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	defer r.Body.Close()
	var req struct {
		Name    string `json:"name"`
		Notes   string `json:"notes"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	project, err := s.store.UpdateProject(id, req.Name, req.Notes, req.Enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Info("proyecto actualizado", "id", project.ID, "enabled", project.Enabled, "user", rs.User.Username)
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request, _ requestSession) {
	settings := s.store.LoadAppSettings(s.config)
	network := DetectNetworkInfo(s.config.PublicIP, settings.DashboardDomain)
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings, "network": network})
}

func (s *Server) getNetworkInfo(w http.ResponseWriter, _ *http.Request, _ requestSession) {
	settings := s.store.LoadAppSettings(s.config)
	writeJSON(w, http.StatusOK, DetectNetworkInfo(s.config.PublicIP, settings.DashboardDomain))
}

func (s *Server) getSystemLogs(w http.ResponseWriter, r *http.Request, _ requestSession) {
	limit := 300
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		var parsed int
		if _, err := fmt.Sscanf(raw, "%d", &parsed); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	lines, err := ReadLastLogLines(s.config.LogPath, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": s.config.LogPath, "maxEntries": defaultMaxLogLines, "lines": lines})
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	before := s.store.LoadAppSettings(s.config)
	beforeConfig := s.config
	beforeConfig.DashboardDomain = before.DashboardDomain
	beforeConfig.LetsEncryptEmail = before.LetsEncryptEmail

	var req AppSettings
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	req.Normalize()
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.DashboardDomain != "" {
		if _, err := ValidateDashboardDomainDNS(req.DashboardDomain, s.config.PublicIP); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	settings, err := s.store.SaveAppSettings(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	afterConfig := s.config
	afterConfig.DashboardDomain = settings.DashboardDomain
	afterConfig.LetsEncryptEmail = settings.LetsEncryptEmail
	domainChanged := strings.TrimSpace(beforeConfig.DashboardDomain) != strings.TrimSpace(afterConfig.DashboardDomain)
	emailChanged := strings.TrimSpace(beforeConfig.LetsEncryptEmail) != strings.TrimSpace(afterConfig.LetsEncryptEmail)
	acmeStateChanged := ACMEEnabled(beforeConfig) != ACMEEnabled(afterConfig)
	traefikResult := s.applyTraefikDynamicOnly()
	if domainChanged || emailChanged || acmeStateChanged {
		traefikResult = s.applyTraefikStaticAndRestart()
	}
	s.log.Info("ajustes actualizados", "dashboard_domain", settings.DashboardDomain, "user", rs.User.Username, "traefik", traefikResult.Message)
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings, "network": DetectNetworkInfo(s.config.PublicIP, settings.DashboardDomain), "traefik": traefikResult})
}

func (s *Server) listManagedDomains(w http.ResponseWriter, _ *http.Request, _ requestSession) {
	writeJSON(w, http.StatusOK, map[string]any{"domains": s.store.ListManagedDomains()})
}

func (s *Server) createManagedDomain(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	domain, err := s.store.AddManagedDomain(req.Domain)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Info("dominio administrado creado", "id", domain.ID, "domain", domain.Domain, "user", rs.User.Username)
	writeJSON(w, http.StatusCreated, domain)
}

func (s *Server) deleteManagedDomain(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	if err := s.store.DeleteManagedDomain(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.log.Info("dominio administrado eliminado", "id", id, "user", rs.User.Username)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func (s *Server) listResources(w http.ResponseWriter, r *http.Request, _ requestSession) {
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	resources := s.store.ListResources()
	if projectID != "" {
		resources = s.store.ListResourcesByProject(projectID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"resources": resources})
}

func (s *Server) createResource(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var resource Resource
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&resource); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	resource.Enabled = true
	if err := s.validatePublicPortForCreate(resource); err != nil {
		s.log.Warn("validacion de puerto publico fallo", "mode", resource.Mode, "public_port", resource.PublicPort, "origin", resource.OriginType, "agent", resource.AgentID, "user", rs.User.Username, "error", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	beforeResources := s.store.ListResources()
	created, err := s.store.AddResource(resource)
	if err != nil {
		s.log.Warn("crear recurso fallo", "mode", resource.Mode, "public_port", resource.PublicPort, "origin", resource.OriginType, "agent", resource.AgentID, "user", rs.User.Username, "error", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	traefikResult := s.applyTraefikAfterResourceChange(beforeResources)
	s.log.Info("recurso creado", "id", created.ID, "mode", created.Mode, "name", created.Name, "public_port", created.PublicPort, "tunnel_port", created.TunnelPort, "origin", created.OriginType, "agent", created.AgentID, "user", rs.User.Username, "traefik", traefikResult.Message)
	w.Header().Set("X-Pangolite-Traefik", traefikResult.Message)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) validatePublicPortForCreate(resource Resource) error {
	if resource.Mode != ModeTCP && resource.Mode != ModeUDP {
		return nil
	}
	return s.validatePublicPort(resource.Mode, resource.PublicPort, "", true)
}

func (s *Server) validatePublicPortForUpdate(current, next Resource) error {
	if next.Mode != ModeTCP && next.Mode != ModeUDP {
		return nil
	}
	mustCheckSystem := current.Mode != next.Mode || current.PublicPort != next.PublicPort
	return s.validatePublicPort(next.Mode, next.PublicPort, next.ID, mustCheckSystem)
}

func (s *Server) validatePublicPort(mode string, port int, excludeID string, checkSystem bool) error {
	if mode != ModeTCP && mode != ModeUDP {
		return nil
	}
	if port == 80 || port == 443 {
		return fmt.Errorf("el puerto publico %d esta reservado para HTTP/HTTPS de Traefik", port)
	}
	if port == ListenPortFromAddr(s.config.Addr) {
		return fmt.Errorf("el puerto publico %d esta reservado por el panel Pangolite", port)
	}
	conflict, err := s.store.ResourcePublicPortConflictExcept(mode, port, excludeID)
	if err != nil {
		return err
	}
	if conflict.ID != "" {
		return fmt.Errorf("ya existe un recurso %s usando el puerto publico %d: %s (%s)", strings.ToUpper(mode), port, conflict.Name, conflict.ID)
	}
	if !checkSystem {
		return nil
	}
	if mode == ModeTCP {
		return TCPPortAvailable(port)
	}
	return UDPPortAvailable(port)
}

func (s *Server) deleteResource(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	beforeResources := s.store.ListResources()
	if err := s.store.DeleteResource(id); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no encontrado") {
			// La eliminacion es idempotente para evitar dobles clicks o reintentos del navegador.
			s.log.Info("recurso ya estaba eliminado", "id", id, "user", rs.User.Username)
			writeJSON(w, http.StatusOK, map[string]any{"deleted": id, "alreadyDeleted": true, "traefik": TraefikApplyResult{OK: true, Message: "El recurso ya estaba eliminado"}})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	traefikResult := s.applyTraefikAfterResourceChange(beforeResources)
	s.log.Info("recurso eliminado", "id", id, "user", rs.User.Username, "traefik", traefikResult.Message)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id, "traefik": traefikResult})
}

func (s *Server) updateResourceControl(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	defer r.Body.Close()
	var req struct {
		ProjectID            string `json:"projectId"`
		Name                 string `json:"name"`
		Mode                 string `json:"mode"`
		Domain               string `json:"domain"`
		PathPrefix           string `json:"pathPrefix"`
		PublicPort           int    `json:"publicPort"`
		BackendScheme        string `json:"backendScheme"`
		BackendHost          string `json:"backendHost"`
		BackendPort          int    `json:"backendPort"`
		OriginType           string `json:"originType"`
		AgentID              string `json:"agentId"`
		TLS                  *bool  `json:"tls"`
		Enabled              *bool  `json:"enabled"`
		DisabledResponseMode string `json:"disabledResponseMode"`
		DisabledStatusCode   int    `json:"disabledStatusCode"`
		DisabledHTML         string `json:"disabledHtml"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	current, err := s.store.ResourceByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	fullEdit := req.Name != "" || req.Mode != "" || req.Domain != "" || req.PublicPort != 0 || req.BackendHost != "" || req.BackendPort != 0 || req.OriginType != "" || req.PathPrefix != "" || req.BackendScheme != "" || req.TLS != nil || req.ProjectID != ""
	beforeResources := s.store.ListResources()
	if !fullEdit {
		enabled := current.Enabled
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		mode := req.DisabledResponseMode
		if mode == "" {
			mode = current.DisabledResponseMode
		}
		status := req.DisabledStatusCode
		if status == 0 {
			status = current.DisabledStatusCode
		}
		html := req.DisabledHTML
		if html == "" {
			html = current.DisabledHTML
		}
		updated, err := s.store.UpdateResourceControl(id, enabled, mode, status, html)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		traefikResult := s.applyTraefikAfterResourceChange(beforeResources)
		s.log.Info("control de recurso actualizado", "id", id, "enabled", updated.Enabled, "mode", updated.DisabledResponseMode, "user", rs.User.Username, "traefik", traefikResult.Message)
		w.Header().Set("X-Pangolite-Traefik", traefikResult.Message)
		writeJSON(w, http.StatusOK, map[string]any{"resource": updated, "traefik": traefikResult})
		return
	}
	next := current
	if req.ProjectID != "" {
		next.ProjectID = req.ProjectID
	}
	if req.Name != "" {
		next.Name = req.Name
	}
	if req.Mode != "" {
		next.Mode = req.Mode
	}
	next.Domain = req.Domain
	next.PathPrefix = req.PathPrefix
	next.PublicPort = req.PublicPort
	next.BackendScheme = req.BackendScheme
	if req.BackendHost != "" {
		next.BackendHost = req.BackendHost
	}
	if req.BackendPort != 0 {
		next.BackendPort = req.BackendPort
	}
	if req.OriginType != "" {
		next.OriginType = req.OriginType
	}
	next.AgentID = req.AgentID
	if req.TLS != nil {
		next.TLS = *req.TLS
	}
	if req.Enabled != nil {
		next.Enabled = *req.Enabled
	}
	if req.DisabledResponseMode != "" {
		next.DisabledResponseMode = req.DisabledResponseMode
	}
	if req.DisabledStatusCode != 0 {
		next.DisabledStatusCode = req.DisabledStatusCode
	}
	next.DisabledHTML = req.DisabledHTML
	next.Normalize(time.Now().UTC())
	if err := s.validatePublicPortForUpdate(current, next); err != nil {
		s.log.Warn("validacion de puerto publico en edicion fallo", "resource", id, "mode", next.Mode, "public_port", next.PublicPort, "origin", next.OriginType, "agent", next.AgentID, "user", rs.User.Username, "error", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	updated, err := s.store.UpdateResource(id, next)
	if err != nil {
		s.log.Warn("editar recurso fallo", "resource", id, "mode", next.Mode, "public_port", next.PublicPort, "origin", next.OriginType, "agent", next.AgentID, "user", rs.User.Username, "error", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	traefikResult := s.applyTraefikAfterResourceChange(beforeResources)
	s.log.Info("recurso editado", "id", id, "mode", updated.Mode, "name", updated.Name, "user", rs.User.Username, "traefik", traefikResult.Message)
	w.Header().Set("X-Pangolite-Traefik", traefikResult.Message)
	writeJSON(w, http.StatusOK, map[string]any{"resource": updated, "traefik": traefikResult})
}

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request, _ requestSession) {
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	agents := s.store.ListAgents()
	if projectID != "" {
		agents = s.store.ListAgentsByProject(projectID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}

func (s *Server) getAgentDetail(w http.ResponseWriter, r *http.Request, _ requestSession) {
	id := r.PathValue("id")
	agent, err := s.store.AgentByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	resources := s.store.ListResourcesByAgent(id)
	writeJSON(w, http.StatusOK, map[string]any{"agent": agent, "resources": resources})
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var req struct {
		ProjectID string `json:"projectId"`
		Name      string `json:"name"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	agent, err := s.store.AddAgent(Agent{ProjectID: req.ProjectID, Name: req.Name})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.attachAgentCommands(r, &agent)
	s.log.Info("cliente NAT creado", "id", agent.ID, "name", agent.Name, "user", rs.User.Username)
	writeJSON(w, http.StatusCreated, agent)
}

func (s *Server) attachAgentCommands(r *http.Request, agent *Agent) {
	if agent == nil || agent.Token == "" {
		return
	}
	base := s.publicBaseURL(r)
	baseClean := strings.TrimRight(base, "/")
	linuxURL := baseClean + "/download/pangolite-client-linux-amd64"
	winURL := baseClean + "/download/pangolite-client-windows-amd64.exe"
	agent.InstallCommand = fmt.Sprintf("curl -fsSL %s -o /tmp/pangolite-client && chmod +x /tmp/pangolite-client && sudo /tmp/pangolite-client --install --server-url %s --agent-id %s --token %s", shellQuote(linuxURL), shellQuote(baseClean), shellQuote(agent.ID), shellQuote(agent.Token))
	agent.RemoveCommand = "sudo /opt/pangolite-client/pangolite-client --remove"
	agent.WindowsInstallCommand = fmt.Sprintf("$u=%q; $o=Join-Path $env:TEMP 'pangolite-client.exe'; Invoke-WebRequest -UseBasicParsing $u -OutFile $o; Start-Process -Verb RunAs $o -ArgumentList '--install --server-url %s --agent-id %s --token %s'", winURL, baseClean, agent.ID, agent.Token)
	agent.WindowsRemoveCommand = `Start-Process -Verb RunAs 'C:\ProgramData\Pangolite Client\pangolite-client.exe' -ArgumentList '--remove'`
}

func (s *Server) publicBaseURL(r *http.Request) string {
	effective := s.store.EffectiveConfig(s.config)
	if strings.TrimSpace(effective.DashboardDomain) != "" {
		return "https://" + strings.TrimSpace(effective.DashboardDomain)
	}
	host := "127.0.0.1:2424"
	if r != nil && strings.TrimSpace(r.Host) != "" {
		host = r.Host
	}
	return "http://" + host
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (s *Server) disableAgent(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	if err := s.store.DisableAgent(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.log.Info("agente deshabilitado", "id", id, "user", rs.User.Username)
	writeJSON(w, http.StatusOK, map[string]any{"disabled": id})
}

func (s *Server) rotateAgentToken(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	agent, err := s.store.RotateAgentToken(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	s.attachAgentCommands(r, &agent)
	s.log.Info("token de cliente NAT rotado", "id", id, "user", rs.User.Username)
	writeJSON(w, http.StatusOK, agent)
}

func (s *Server) renderTraefik(w http.ResponseWriter, _ *http.Request, _ requestSession) {
	result := s.applyTraefikStaticAndRestart()
	if !result.OK {
		writeError(w, http.StatusBadRequest, result.Message)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": result.Message, "traefik": result})
}

type TraefikApplyResult struct {
	OK             bool   `json:"ok"`
	Message        string `json:"message"`
	Restarted      bool   `json:"restarted"`
	StaticChanged  bool   `json:"staticChanged"`
	DynamicChanged bool   `json:"dynamicChanged"`
}

func (s *Server) applyTraefikDynamicOnly() TraefikApplyResult {
	if !s.config.AutoTraefik {
		return TraefikApplyResult{OK: true, Message: "Traefik automatico desactivado", DynamicChanged: false}
	}
	effective := s.store.EffectiveConfig(s.config)
	if err := RenderDynamicTraefik(effective); err != nil {
		s.log.Warn("no se pudo actualizar configuracion dinamica de Traefik", "error", err.Error())
		return TraefikApplyResult{OK: false, Message: "no se pudo actualizar Traefik: " + err.Error(), DynamicChanged: false}
	}
	return TraefikApplyResult{OK: true, Message: "Traefik recargara la configuracion dinamica automaticamente", DynamicChanged: true}
}

func (s *Server) applyTraefikAfterResourceChange(before []Resource) TraefikApplyResult {
	if err := s.refreshBridgeListeners(); err != nil && s.log != nil {
		s.log.Warn("no se pudieron sincronizar puentes de clientes NAT", "error", err.Error())
	}
	if !s.config.AutoTraefik {
		return TraefikApplyResult{OK: true, Message: "Traefik automatico desactivado", DynamicChanged: false}
	}
	after := s.store.ListResources()
	if TraefikPortSignature(before) != TraefikPortSignature(after) {
		return s.applyTraefikStaticAndRestart()
	}
	return TraefikApplyResult{OK: true, Message: "Traefik detectara el cambio por configuracion dinamica", DynamicChanged: true}
}

func (s *Server) applyTraefikStaticAndRestart() TraefikApplyResult {
	if !s.config.AutoTraefik {
		return TraefikApplyResult{OK: true, Message: "Traefik automatico desactivado", StaticChanged: false}
	}
	effective := s.store.EffectiveConfig(s.config)
	if err := RenderStaticTraefik(effective, s.store.ListResources()); err != nil {
		s.log.Warn("no se pudo renderizar Traefik", "error", err.Error())
		return TraefikApplyResult{OK: false, Message: "no se pudo renderizar Traefik: " + err.Error()}
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return TraefikApplyResult{OK: true, Message: "configuracion escrita; systemctl no disponible para reiniciar Traefik", StaticChanged: true}
	}
	s.scheduleTraefikRestart("cambio de entrypoints TCP/UDP")
	return TraefikApplyResult{OK: true, Message: "Traefik aplicara el cambio automaticamente en segundo plano", Restarted: false, StaticChanged: true, DynamicChanged: true}
}

func (s *Server) scheduleTraefikRestart(reason string) {
	s.traefikRestartMu.Lock()
	defer s.traefikRestartMu.Unlock()

	if s.traefikRestartTimer != nil {
		s.traefikRestartTimer.Stop()
		if s.log != nil {
			s.log.Info("reinicio de Traefik reprogramado", "reason", reason, "delay", "15s")
		}
	} else if s.log != nil {
		s.log.Info("reinicio de Traefik programado", "reason", reason, "delay", "15s")
	}

	s.traefikRestartTimer = time.AfterFunc(15*time.Second, func() {
		s.traefikRestartMu.Lock()
		s.traefikRestartTimer = nil
		s.traefikRestartMu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "systemctl", "restart", "traefik")
		out, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = err.Error()
			}
			if s.log != nil {
				s.log.Warn("reinicio de Traefik fallo", "reason", reason, "error", msg)
			}
			return
		}
		if s.log != nil {
			s.log.Info("Traefik reiniciado automaticamente", "reason", reason)
		}
	})
}

func (s *Server) refreshBridgeListeners() error {
	if s.bridges == nil {
		return nil
	}
	return s.bridges.Sync(s.store.ListResources())
}

func (s *Server) resourceHealth(w http.ResponseWriter, r *http.Request, _ requestSession) {
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	resources := s.store.ListResources()
	if projectID != "" {
		resources = s.store.ListResourcesByProject(projectID)
	}
	checks := make([]ResourceHealth, 0, len(resources))
	for _, res := range resources {
		checks = append(checks, s.checkResourceHealth(r.Context(), res))
	}
	writeJSON(w, http.StatusOK, map[string]any{"checks": checks})
}

func (s *Server) checkResourceHealth(ctx context.Context, res Resource) ResourceHealth {
	out := ResourceHealth{ResourceID: res.ID, Name: res.Name, Mode: res.Mode, Status: "unknown", CheckedAt: time.Now().UTC()}
	if !res.Enabled {
		out.Status = "suspended"
		out.Message = "recurso suspendido"
		return out
	}
	if res.UsesAgent() {
		ag, err := s.store.AgentByID(res.AgentID)
		if err != nil || !ag.Online {
			out.Status = "offline"
			out.Message = "cliente NAT sin conexion reciente"
			return out
		}
	}
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	switch res.Mode {
	case ModeHTTP:
		scheme := res.BackendScheme
		if scheme == "" {
			scheme = "http"
		}
		if res.UsesAgent() {
			jobID, err := randomID()
			if err != nil {
				out.Status = "error"
				out.Message = err.Error()
				return out
			}
			resp, err := s.hub.Submit(checkCtx, res.AgentID, AgentJob{ID: jobID, Kind: ModeHTTP, ResourceID: res.ID, Method: http.MethodHead, Path: "/", TargetScheme: scheme, TargetHost: res.BackendHost, TargetPort: res.BackendPort})
			if err != nil {
				out.Status = "warning"
				out.Message = err.Error()
				return out
			}
			if resp.Error != "" {
				out.Status = "warning"
				out.Message = resp.Error
				return out
			}
			out.Status = "ok"
			out.Message = fmt.Sprintf("backend remoto responde HTTP %d", resp.StatusCode)
			return out
		}
		url := fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(res.BackendHost, fmt.Sprint(res.BackendPort)))
		req, err := http.NewRequestWithContext(checkCtx, http.MethodHead, url, nil)
		if err != nil {
			out.Status = "error"
			out.Message = err.Error()
			return out
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			out.Status = "warning"
			out.Message = err.Error()
			return out
		}
		_ = resp.Body.Close()
		out.Status = "ok"
		out.Message = fmt.Sprintf("backend responde HTTP %d", resp.StatusCode)
		return out
	case ModeTCP:
		addr := res.ServiceAddress()
		if res.UsesAgent() {
			addr = res.BridgeAddress()
		}
		d := net.Dialer{Timeout: 2 * time.Second}
		conn, err := d.DialContext(checkCtx, "tcp", addr)
		if err != nil {
			out.Status = "warning"
			out.Message = err.Error()
			return out
		}
		_ = conn.Close()
		out.Status = "ok"
		out.Message = "conexion TCP aceptada"
		return out
	default:
		out.Status = "unknown"
		out.Message = "health check no disponible para este protocolo"
		return out
	}
}

func (s *Server) touchAgentFromRequest(agentID string, r *http.Request) {
	s.store.TouchAgent(agentID, AgentHeartbeat{
		OS:        r.Header.Get("X-Pangolite-Client-OS"),
		Arch:      r.Header.Get("X-Pangolite-Client-Arch"),
		Hostname:  r.Header.Get("X-Pangolite-Client-Hostname"),
		PrivateIP: r.Header.Get("X-Pangolite-Client-Private-IP"),
		PublicIP:  requestPublicIP(r),
		Version:   r.Header.Get("X-Pangolite-Client-Version"),
		LastError: r.Header.Get("X-Pangolite-Client-Last-Error"),
	})
}

func requestPublicIP(r *http.Request) string {
	for _, h := range []string{"X-Forwarded-For", "X-Real-IP"} {
		if v := strings.TrimSpace(r.Header.Get(h)); v != "" {
			if i := strings.IndexByte(v, ','); i >= 0 {
				v = strings.TrimSpace(v[:i])
			}
			return v
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func (s *Server) downloadClientLinuxAMD64(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(os.Getenv("PANGOLITE_CLIENT_LINUX_AMD64"))
	if path == "" {
		path = "/opt/pangolite/public/pangolite-client-linux-amd64"
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "cliente linux amd64 no disponible en este servidor")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="pangolite-client-linux-amd64"`)
	http.ServeFile(w, r, path)
}

func (s *Server) downloadClientWindowsAMD64(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(os.Getenv("PANGOLITE_CLIENT_WINDOWS_AMD64"))
	if path == "" {
		path = "/opt/pangolite/public/pangolite-client-windows-amd64.exe"
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "cliente Windows amd64 no disponible en este servidor")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="pangolite-client-windows-amd64.exe"`)
	http.ServeFile(w, r, path)
}

func (s *Server) agentPoll(w http.ResponseWriter, r *http.Request) {
	agentID, token := agentCredentials(r)
	if _, ok := s.store.AuthenticateAgent(agentID, token); !ok {
		writeError(w, http.StatusUnauthorized, "credenciales de agente invalidas")
		return
	}
	s.touchAgentFromRequest(agentID, r)
	ctx, cancel := context.WithTimeout(r.Context(), AgentPollTimeout)
	defer cancel()
	job, ok, err := s.hub.Poll(ctx, agentID)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		writeError(w, http.StatusInternalServerError, "no se pudo consultar trabajos")
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) agentJobResponse(w http.ResponseWriter, r *http.Request) {
	agentID, token := agentCredentials(r)
	if _, ok := s.store.AuthenticateAgent(agentID, token); !ok {
		writeError(w, http.StatusUnauthorized, "credenciales de agente invalidas")
		return
	}
	jobID := r.PathValue("id")
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "job id requerido")
		return
	}
	defer r.Body.Close()
	var resp AgentResponse
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	resp.JobID = jobID
	if resp.StatusCode < 100 || resp.StatusCode > 599 {
		resp.StatusCode = http.StatusBadGateway
	}
	if !s.hub.Complete(jobID, resp) {
		writeError(w, http.StatusNotFound, "job no encontrado o expirado")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) agentStreamPoll(w http.ResponseWriter, r *http.Request) {
	agentID, token := agentCredentials(r)
	if _, ok := s.store.AuthenticateAgent(agentID, token); !ok {
		writeError(w, http.StatusUnauthorized, "credenciales de cliente invalidas")
		return
	}
	s.touchAgentFromRequest(agentID, r)
	ctx, cancel := context.WithTimeout(r.Context(), AgentPollTimeout)
	defer cancel()
	job, ok, err := s.hub.PollStream(ctx, agentID)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		writeError(w, http.StatusInternalServerError, "no se pudo consultar streams")
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) agentStreamSocket(w http.ResponseWriter, r *http.Request) {
	agentID, token := agentCredentials(r)
	if _, ok := s.store.AuthenticateAgent(agentID, token); !ok {
		writeError(w, http.StatusUnauthorized, "credenciales de cliente invalidas")
		return
	}
	s.touchAgentFromRequest(agentID, r)
	streamID := r.PathValue("id")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream id requerido")
		return
	}
	sess, ok := s.hub.AttachStream(streamID, agentID)
	if !ok {
		writeError(w, http.StatusNotFound, "stream no encontrado o expirado")
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		if s.log != nil {
			s.log.Warn("websocket de stream rechazado", "stream", streamID, "error", err.Error())
		}
		return
	}
	defer s.hub.CompleteStream(streamID)
	if err := bridgeWebSocketNetConn(r.Context(), ws, sess.ClientConn); err != nil && s.log != nil {
		s.log.Debug("stream TCP cerrado", "stream", streamID, "agent", agentID, "error", err.Error())
	}
}

func (s *Server) publicOrIndex(w http.ResponseWriter, r *http.Request) {
	if resource, ok := s.store.FindHTTPPanelResource(r.Host, r.URL.Path); ok {
		if !resource.Enabled {
			s.serveDisabledResource(w, r, resource)
			return
		}
		if resource.UsesAgent() {
			s.proxyViaAgent(w, r, resource)
			return
		}
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusNotFound, "ruta no encontrada")
		return
	}
	rs, ok := s.currentSession(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	if rs.User.ForcePasswordChange {
		http.Redirect(w, r, "/password", http.StatusFound)
		return
	}
	s.index(w, r)
}

func (s *Server) serveDisabledResource(w http.ResponseWriter, r *http.Request, resource Resource) {
	status := resource.DisabledStatusCode
	if status == 0 {
		status = http.StatusForbidden
	}
	switch resource.DisabledResponseMode {
	case DisabledResponse404:
		http.NotFound(w, r)
	case DisabledResponseHTML:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		if r.Method != http.MethodHead {
			html := resource.DisabledHTML
			if strings.TrimSpace(html) == "" {
				html = defaultDisabledHTML(resource.Name, status)
			}
			_, _ = w.Write([]byte(html))
		}
	default:
		http.Error(w, "403 recurso deshabilitado", http.StatusForbidden)
	}
}

func defaultDisabledHTML(name string, status int) string {
	if strings.TrimSpace(name) == "" {
		name = "Servicio no disponible"
	}
	return fmt.Sprintf(`<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>%d - Servicio no disponible</title><style>body{margin:0;min-height:100vh;display:grid;place-items:center;background:#000;color:#fff;font-family:-apple-system-body,ui-sans-serif,-apple-system,system-ui,Segoe UI,Arial,sans-serif}.card{max-width:560px;margin:24px;padding:28px;border:1px solid rgba(255,255,255,.18);border-radius:16px;background:rgba(255,255,255,.08)}.k{color:#afafaf;font-size:14px}.h{font-size:24px;font-weight:780;margin:8px 0}.p{color:#cdcdcd}</style></head><body><main class="card"><div class="k">Codigo %d</div><h1 class="h">%s</h1><p class="p">Este recurso se encuentra temporalmente deshabilitado.</p></main></body></html>`, status, status, htmlEscape(name))
}

func htmlEscape(value string) string {
	value = strings.ReplaceAll(value, "&", "&amp;")
	value = strings.ReplaceAll(value, "<", "&lt;")
	value = strings.ReplaceAll(value, ">", "&gt;")
	value = strings.ReplaceAll(value, `"`, "&quot;")
	value = strings.ReplaceAll(value, "'", "&#39;")
	return value
}

func (s *Server) proxyViaAgent(w http.ResponseWriter, r *http.Request, resource Resource) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "no se pudo leer request del tunel HTTP")
		return
	}
	jobID, err := randomID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo crear job")
		return
	}
	job := AgentJob{
		ID:           jobID,
		Kind:         ModeHTTP,
		ResourceID:   resource.ID,
		Method:       r.Method,
		Path:         r.URL.EscapedPath(),
		RawQuery:     r.URL.RawQuery,
		Header:       cloneSafeHeader(r.Header),
		Body:         body,
		TargetScheme: resource.BackendScheme,
		TargetHost:   resource.BackendHost,
		TargetPort:   resource.BackendPort,
	}
	resp, err := s.hub.Submit(r.Context(), resource.AgentID, job)
	if err != nil {
		s.log.Warn("tunel HTTP fallo", "agent", resource.AgentID, "resource", resource.ID, "error", err.Error())
		writeError(w, http.StatusServiceUnavailable, "agente no disponible o sin respuesta")
		return
	}
	if resp.Error != "" {
		writeError(w, http.StatusBadGateway, resp.Error)
		return
	}
	copySafeHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	if r.Method != http.MethodHead {
		_, _ = w.Write(resp.Body)
	}
}

type authedHandler func(http.ResponseWriter, *http.Request, requestSession)

func (s *Server) requireAuth(next authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rs, ok := s.authorizePanelRequest(w, r, false)
		if !ok {
			return
		}
		next(w, r, rs)
	}
}

func (s *Server) requireAuthAllowForce(next authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rs, ok := s.authorizePanelRequest(w, r, true)
		if !ok {
			return
		}
		next(w, r, rs)
	}
}

func (s *Server) authorizePanelRequest(w http.ResponseWriter, r *http.Request, allowForcePassword bool) (requestSession, bool) {
	rs, ok := s.currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "sesion requerida")
		return requestSession{}, false
	}
	if isUnsafeMethod(r.Method) && r.Header.Get("X-CSRF-Token") != rs.Session.CSRFToken {
		writeError(w, http.StatusForbidden, "CSRF token invalido")
		return requestSession{}, false
	}
	if rs.User.ForcePasswordChange && !allowForcePassword {
		writeError(w, http.StatusForbidden, "debes cambiar la contraseña temporal antes de continuar")
		return requestSession{}, false
	}
	return rs, true
}

func (s *Server) currentSession(r *http.Request) (requestSession, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return requestSession{}, false
	}
	sess, user, ok := s.store.SessionWithUser(cookie.Value)
	if !ok {
		return requestSession{}, false
	}
	return requestSession{Session: sess, User: user, RawID: cookie.Value}, true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, value string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   s.secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookie(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) secureCookie(r *http.Request) bool {
	switch strings.ToLower(s.config.CookieSecureOverride) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	writeHTML(w, appHTML)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(p)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer no soporta hijack")
	}
	return hijacker.Hijack()
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (s *Server) recoverRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.log.Error("panic en request", "method", r.Method, "path", r.URL.Path, "host", r.Host, "panic", fmt.Sprint(recovered), "stack", string(debug.Stack()))
				writeError(w, http.StatusInternalServerError, "error interno; revisa logs del sistema")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		if r.URL.Path != "/healthz" && r.URL.Path != "/api/v1/traefik-config" && r.URL.Path != "/api/agent/poll" && !strings.HasPrefix(r.URL.Path, "/assets/") {
			s.log.Info("request", "method", r.Method, "path", r.URL.Path, "host", r.Host, "status", rec.status, "duration", time.Since(start).String())
		}
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		next.ServeHTTP(w, r)
	})
}

func writeHTML(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func publicUser(u User) map[string]any {
	return map[string]any{"id": u.ID, "username": u.Username, "forcePasswordChange": u.ForcePasswordChange}
}

func isUnsafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func agentCredentials(r *http.Request) (string, string) {
	agentID := strings.TrimSpace(r.Header.Get("X-Pangolite-Agent"))
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return agentID, strings.TrimSpace(token)
}

func urlForToken(token string) string {
	if token == "" {
		return ""
	}
	return fmt.Sprintf("?token=%s", token)
}
