package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

const sessionCookieName = "pangolite_session"

type loginAttempt struct {
	Fails       int
	LockedUntil time.Time
}

type Server struct {
	config  Config
	store   *Store
	hub     *TunnelHub
	bridges *BridgeManager
	mux     *http.ServeMux
	log     *slog.Logger

	loginMu       sync.Mutex
	loginAttempts map[string]loginAttempt

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
	if err := store.SetPrimaryManagedDomain(effective.DashboardDomain, ""); err != nil && logger != nil {
		logger.Warn("no se pudo registrar dominio principal del panel", "domain", effective.DashboardDomain, "error", err.Error())
	}
	if err := EnsureSuspensionTemplates(c.SuspensionTemplateDir); err != nil && logger != nil {
		logger.Warn("no se pudieron preparar plantillas de suspension", "dir", c.SuspensionTemplateDir, "error", err.Error())
	}
	c = effective
	hub := NewTunnelHub(64)
	s := &Server{config: c, store: store, hub: hub, bridges: NewBridgeManager(hub, logger), mux: http.NewServeMux(), log: logger, loginAttempts: map[string]loginAttempt{}}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return securityHeaders(s.recoverRequests(s.logRequests(s.mux)))
}

func (s *Server) Run(ctx context.Context) error {
	s.startAutomaticBackups(ctx)
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
	s.mux.HandleFunc("GET /reset", s.resetPasswordPage)
	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.HandleFunc("POST /api/logout", s.requireAuthAllowForce(s.logout))
	s.mux.HandleFunc("GET /api/session", s.sessionInfo)
	s.mux.HandleFunc("POST /api/password", s.requireAuthAllowForce(s.changePassword))
	s.mux.HandleFunc("PATCH /api/profile", s.requireAuth(s.updateProfile))
	s.mux.HandleFunc("GET /api/password-reset/status", s.passwordResetStatus)
	s.mux.HandleFunc("POST /api/password-reset/request", s.passwordResetRequest)
	s.mux.HandleFunc("POST /api/password-reset/confirm", s.passwordResetConfirm)
	s.mux.HandleFunc("GET /api/v1/traefik-config", s.traefikConfig)
	s.mux.HandleFunc("GET /api/projects", s.requireAuth(s.listProjects))
	s.mux.HandleFunc("POST /api/projects", s.requireAuth(s.createProject))
	s.mux.HandleFunc("PATCH /api/projects/{id}", s.requireAuth(s.updateProject))
	s.mux.HandleFunc("DELETE /api/projects/{id}", s.requireAuth(s.deleteProject))
	s.mux.HandleFunc("GET /api/settings", s.requireAuth(s.getSettings))
	s.mux.HandleFunc("PATCH /api/settings", s.requireAuth(s.updateSettings))
	s.mux.HandleFunc("POST /api/settings/smtp/test", s.requireAuth(s.testSMTPSettings))
	s.mux.HandleFunc("GET /api/system/network", s.requireAuth(s.getNetworkInfo))
	s.mux.HandleFunc("GET /api/certificates/status", s.requireAuth(s.getCertificateStatus))
	s.mux.HandleFunc("GET /api/system/logs", s.requireAuth(s.getSystemLogs))
	s.mux.HandleFunc("GET /api/system/logs/download", s.requireAuth(s.downloadSystemLogs))
	s.mux.HandleFunc("GET /api/audit", s.requireAuth(s.listAudit))
	s.mux.HandleFunc("GET /api/backups", s.requireAuth(s.listBackups))
	s.mux.HandleFunc("POST /api/backups", s.requireAuth(s.createBackup))
	s.mux.HandleFunc("GET /api/backups/{name}/download", s.requireAuth(s.downloadBackup))
	s.mux.HandleFunc("GET /api/domains", s.requireAuth(s.listManagedDomains))
	s.mux.HandleFunc("POST /api/domains", s.requireAuth(s.createManagedDomain))
	s.mux.HandleFunc("PATCH /api/domains/{id}", s.requireAuth(s.updateManagedDomain))
	s.mux.HandleFunc("DELETE /api/domains/{id}", s.requireAuth(s.deleteManagedDomain))
	s.mux.HandleFunc("GET /api/resources", s.requireAuth(s.listResources))
	s.mux.HandleFunc("GET /api/resources/health", s.requireAuth(s.resourceHealth))
	s.mux.HandleFunc("GET /api/suspension-templates", s.requireAuth(s.listSuspensionTemplates))
	s.mux.HandleFunc("POST /api/suspension-templates", s.requireAuth(s.createSuspensionTemplate))
	s.mux.HandleFunc("GET /api/suspension-templates/{id}", s.requireAuth(s.getSuspensionTemplate))
	s.mux.HandleFunc("PUT /api/suspension-templates/{id}", s.requireAuth(s.updateSuspensionTemplate))
	s.mux.HandleFunc("POST /api/resources", s.requireAuth(s.createResource))
	s.mux.HandleFunc("PATCH /api/resources/{id}", s.requireAuth(s.updateResourceControl))
	s.mux.HandleFunc("DELETE /api/resources/{id}", s.requireAuth(s.deleteResource))
	s.mux.HandleFunc("GET /api/agents", s.requireAuth(s.listAgents))
	s.mux.HandleFunc("GET /api/agents/{id}", s.requireAuth(s.getAgentDetail))
	s.mux.HandleFunc("GET /api/terminal/local", s.localTerminalSocket)
	s.mux.HandleFunc("GET /api/terminal/agents/{id}", s.agentTerminalSocket)
	s.mux.HandleFunc("POST /api/agents", s.requireAuth(s.createAgent))
	s.mux.HandleFunc("DELETE /api/agents/{id}", s.requireAuth(s.deleteAgent))
	s.mux.HandleFunc("POST /api/agents/{id}/token", s.requireAuth(s.rotateAgentToken))
	s.mux.HandleFunc("POST /api/agents/{id}/maintenance", s.requireAuth(s.updateAgentMaintenance))
	s.mux.HandleFunc("POST /api/agents/{id}/web-maintenance", s.requireAuth(s.updateAgentWebMaintenance))
	s.mux.HandleFunc("POST /api/render-traefik", s.requireAuth(s.renderTraefik))
	s.mux.HandleFunc("POST /api/agent/discover", s.agentDiscover)
	s.mux.HandleFunc("POST /api/agent/poll", s.agentPoll)
	s.mux.HandleFunc("POST /api/agent/jobs/{id}/response", s.agentJobResponse)
	s.mux.HandleFunc("POST /api/agent/stream-poll", s.agentStreamPoll)
	s.mux.HandleFunc("GET /api/agent/streams/{id}", s.agentStreamSocket)
	s.mux.HandleFunc("GET /download/{name}", s.downloadClientAsset)
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
	renderUIPage(w, "login.html", uiPageData{Title: "Pangolite - Iniciar sesion", Heading: "Pangolite", Subtitle: "Panel seguro de proxys y agentes", ScriptPath: "/assets/app/login.js"})
}

func (s *Server) passwordPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentSession(r); !ok {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	renderUIPage(w, "password.html", uiPageData{Title: "Pangolite - Cambiar contraseña", Heading: "Cambiar contraseña", Subtitle: "Reemplaza la contraseña temporal antes de administrar el panel.", ScriptPath: "/assets/app/password.js"})
}

func (s *Server) resetPasswordPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.currentSession(r); ok {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	renderUIPage(w, "reset.html", uiPageData{Title: "Pangolite - Restablecer contraseña", Heading: "Restablecer contraseña", Subtitle: "Define una nueva contraseña usando el enlace enviado por correo.", ScriptPath: "/assets/app/reset.js"})
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
	if retryAt, blocked := s.loginBlocked(req.Username, r); blocked {
		s.log.Warn("login bloqueado temporalmente", "user", NormalizeUsername(req.Username), "remote", clientIPForRateLimit(r), "retry_at", retryAt.Format(time.RFC3339))
		writeError(w, http.StatusTooManyRequests, "demasiados intentos fallidos; espera unos minutos antes de intentar otra vez")
		return
	}
	user, ok := s.store.AuthenticateUser(req.Username, req.Password)
	if !ok {
		s.recordLoginFailure(req.Username, r)
		s.log.Warn("login fallido", "user", NormalizeUsername(req.Username), "remote", clientIPForRateLimit(r))
		writeError(w, http.StatusUnauthorized, "usuario o contraseña invalidos")
		return
	}
	s.recordLoginSuccess(req.Username, r)
	rawID, sess, err := s.store.CreateSession(user.ID, sessionDuration(s.config))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo crear sesion")
		return
	}
	s.setSessionCookie(w, r, rawID, sess.ExpiresAt)
	s.log.Info("login correcto", "user", user.Username)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": publicUser(user), "csrfToken": sess.CSRFToken})
}

func (s *Server) loginKey(username string, r *http.Request) string {
	return NormalizeUsername(username) + "|" + clientIPForRateLimit(r)
}

func (s *Server) loginBlocked(username string, r *http.Request) (time.Time, bool) {
	if s.loginAttempts == nil {
		return time.Time{}, false
	}
	key := s.loginKey(username, r)
	now := time.Now().UTC()
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	attempt := s.loginAttempts[key]
	if attempt.LockedUntil.After(now) {
		return attempt.LockedUntil, true
	}
	if attempt.LockedUntil.Before(now) && !attempt.LockedUntil.IsZero() {
		delete(s.loginAttempts, key)
	}
	return time.Time{}, false
}

func (s *Server) recordLoginFailure(username string, r *http.Request) {
	if s.loginAttempts == nil {
		return
	}
	key := s.loginKey(username, r)
	now := time.Now().UTC()
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	attempt := s.loginAttempts[key]
	if attempt.LockedUntil.Before(now) {
		attempt.LockedUntil = time.Time{}
	}
	attempt.Fails++
	if attempt.Fails >= 5 {
		attempt.LockedUntil = now.Add(10 * time.Minute)
	}
	s.loginAttempts[key] = attempt
}

func (s *Server) recordLoginSuccess(username string, r *http.Request) {
	if s.loginAttempts == nil {
		return
	}
	s.loginMu.Lock()
	delete(s.loginAttempts, s.loginKey(username, r))
	s.loginMu.Unlock()
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
		Email           string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	requireCurrent := !rs.User.ForcePasswordChange
	if err := s.store.ChangePassword(rs.User.ID, req.CurrentPassword, req.NewPassword, req.Email, requireCurrent); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if rs.User.ForcePasswordChange && s.config.InitialPasswordFile != "" {
		if err := os.Remove(s.config.InitialPasswordFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("no se pudo eliminar password temporal", "path", s.config.InitialPasswordFile, "error", err.Error())
		}
	}
	s.log.Info("password actualizada", "user", rs.User.Username)
	s.recordAudit(r, rs, "user.password.change", "user", fmt.Sprint(rs.User.ID), "", map[string]any{"forcePasswordChange": rs.User.ForcePasswordChange})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	if err := s.store.UpdateUserEmail(rs.User.ID, req.Email); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, err := s.store.UserByID(rs.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "no se pudo recargar perfil")
		return
	}
	s.log.Info("perfil actualizado", "user", user.Username)
	s.recordAudit(r, requestSession{Session: rs.Session, User: user, RawID: rs.RawID}, "user.profile.update", "user", fmt.Sprint(user.ID), "", map[string]any{"emailSet": strings.TrimSpace(user.Email) != ""})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "user": publicUser(user)})
}

func (s *Server) passwordResetStatus(w http.ResponseWriter, _ *http.Request) {
	settings := s.store.LoadAppSettings(s.config)
	enabled := settings.SMTPReady()
	writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled})
}

func (s *Server) passwordResetRequest(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	settings := s.store.LoadAppSettings(s.config)
	if !settings.SMTPReady() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Si la cuenta existe y la recuperacion esta habilitada, enviaremos instrucciones."})
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		writeError(w, http.StatusBadRequest, "correo requerido")
		return
	}
	if err := ValidateEmailAddress(email); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resetKey := "reset:" + email
	if retryAt, blocked := s.loginBlocked(resetKey, r); blocked {
		s.log.Warn("recuperacion bloqueada temporalmente", "email", email, "remote", clientIPForRateLimit(r), "retry_at", retryAt.Format(time.RFC3339))
		writeError(w, http.StatusTooManyRequests, "demasiadas solicitudes de recuperacion; espera unos minutos antes de intentar otra vez")
		return
	}
	s.recordLoginFailure(resetKey, r)
	if user, err := s.store.UserByEmail(email); err == nil {
		token, err := s.store.CreatePasswordResetToken(user.ID, 20*time.Minute)
		if err == nil {
			link := strings.TrimRight(s.publicPanelBaseURL(r), "/") + "/reset?token=" + url.QueryEscape(token)
			body := "Solicitaste restablecer la contraseña de Pangolite.\n\nAbre este enlace para definir una nueva contraseña:\n" + link + "\n\nEl enlace vence en 20 minutos. Si no solicitaste este cambio, ignora este mensaje."
			if mailErr := sendSMTPMail(settings, mailMessage{ToEmail: user.Email, Subject: "Restablecer contraseña de Pangolite", Text: body}); mailErr != nil {
				s.log.Warn("no se pudo enviar recuperacion de contraseña", "user", user.Username, "error", mailErr.Error())
			} else {
				s.log.Info("recuperacion de contraseña enviada", "user", user.Username)
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Si la cuenta existe y la recuperacion esta habilitada, enviaremos instrucciones."})
}

func (s *Server) passwordResetConfirm(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req struct {
		Token       string `json:"token"`
		NewPassword string `json:"newPassword"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	user, err := s.store.ConsumePasswordResetToken(req.Token, req.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.clearSessionCookie(w, r)
	s.log.Info("password restablecida por correo", "user", user.Username)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) publicPanelBaseURL(r *http.Request) string {
	effective := s.store.EffectiveConfig(s.config)
	if strings.TrimSpace(effective.DashboardDomain) != "" {
		return "https://" + strings.TrimSpace(effective.DashboardDomain)
	}
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		host = s.config.Addr
	}
	return scheme + "://" + host
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
	s.recordAudit(r, rs, "project.create", "project", project.ID, project.ID, map[string]any{"name": project.Name, "slug": project.Slug})
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
		Enabled *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	current, err := s.store.ProjectByID(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	enabled := current.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	project, err := s.store.UpdateProject(id, req.Name, req.Notes, enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Info("proyecto actualizado", "id", project.ID, "enabled", project.Enabled, "user", rs.User.Username)
	s.recordAudit(r, rs, "project.update", "project", project.ID, project.ID, map[string]any{"name": project.Name, "enabled": project.Enabled})
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	if err := s.store.DeleteProjectIfEmpty(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.log.Info("proyecto eliminado", "id", id, "user", rs.User.Username)
	s.recordAudit(r, rs, "project.delete", "project", id, id, nil)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func (s *Server) getSettings(w http.ResponseWriter, _ *http.Request, _ requestSession) {
	settings := s.store.LoadAppSettings(s.config)
	effective := s.store.EffectiveConfig(s.config)
	network := DetectNetworkInfo(s.config.PublicIP, settings.DashboardDomain)
	certificate := ResolveCertificateStatus(effective, settings.DashboardDomain, settings.DashboardDomain != "")
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings, "network": network, "certificate": certificate})
}

func (s *Server) getCertificateStatus(w http.ResponseWriter, r *http.Request, _ requestSession) {
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	sslEnabled := true
	if raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("ssl"))); raw == "0" || raw == "false" || raw == "no" {
		sslEnabled = false
	}
	writeJSON(w, http.StatusOK, map[string]any{"certificate": ResolveCertificateStatus(s.store.EffectiveConfig(s.config), domain, sslEnabled)})
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
	if limit > defaultMaxLogLines {
		limit = defaultMaxLogLines
	}
	lines, err := ReadLastLogLines(s.config.LogPath, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": s.config.LogPath, "maxEntries": defaultMaxLogLines, "lines": lines})
}

func (s *Server) downloadSystemLogs(w http.ResponseWriter, r *http.Request, _ requestSession) {
	path := strings.TrimSpace(s.config.LogPath)
	if path == "" {
		writeError(w, http.StatusNotFound, "logs no configurados")
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "archivo de logs no disponible")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="pangolite.log"`)
	http.ServeFile(w, r, path)
}

func (s *Server) updateSettings(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	before := s.store.LoadAppSettings(s.config)
	beforeConfig := s.config
	beforeConfig.DashboardDomain = before.DashboardDomain
	beforeConfig.LetsEncryptEmail = before.LetsEncryptEmail

	var raw struct {
		DashboardDomain     string `json:"dashboardDomain"`
		LetsEncryptEmail    string `json:"letsEncryptEmail"`
		BackupIntervalHours int    `json:"backupIntervalHours"`
		BackupRetentionDays int    `json:"backupRetentionDays"`
		SMTPEnabled         bool   `json:"smtpEnabled"`
		SMTPHost            string `json:"smtpHost"`
		SMTPPort            int    `json:"smtpPort"`
		SMTPSecurity        string `json:"smtpSecurity"`
		SMTPUsername        string `json:"smtpUsername"`
		SMTPPassword        string `json:"smtpPassword"`
		SMTPFromEmail       string `json:"smtpFromEmail"`
		SMTPFromName        string `json:"smtpFromName"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&raw); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	req := AppSettings{
		DashboardDomain:     raw.DashboardDomain,
		LetsEncryptEmail:    raw.LetsEncryptEmail,
		BackupIntervalHours: raw.BackupIntervalHours,
		BackupRetentionDays: raw.BackupRetentionDays,
		SMTPEnabled:         raw.SMTPEnabled,
		SMTPHost:            raw.SMTPHost,
		SMTPPort:            raw.SMTPPort,
		SMTPSecurity:        raw.SMTPSecurity,
		SMTPUsername:        raw.SMTPUsername,
		SMTPPassword:        raw.SMTPPassword,
		SMTPFromEmail:       raw.SMTPFromEmail,
		SMTPFromName:        raw.SMTPFromName,
	}
	if strings.TrimSpace(req.SMTPPassword) == "" {
		req.SMTPPasswordSet = before.SMTPPasswordSet
	}
	req.Normalize()
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.SMTPEnabled {
		probe := req
		if strings.TrimSpace(probe.SMTPPassword) == "" {
			probe.SMTPPassword = before.SMTPPassword
		}
		probe.SMTPPasswordSet = strings.TrimSpace(probe.SMTPPassword) != ""
		if err := validateSMTPConnectivity(probe); err != nil {
			writeError(w, http.StatusBadRequest, "SMTP no valido: "+err.Error())
			return
		}
	}
	if req.DashboardDomain != "" {
		if _, err := ValidateDashboardDomainDNS(req.DashboardDomain, s.config.PublicIP); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	candidateConfig := s.config
	candidateConfig.DashboardDomain = req.DashboardDomain
	candidateConfig.LetsEncryptEmail = req.LetsEncryptEmail
	if !ACMEEnabled(candidateConfig) && s.hasHTTPSSLResources() {
		writeError(w, http.StatusBadRequest, "no puedes desactivar ACME mientras existan recursos web con SSL activo; desactiva Usar SSL en esos recursos primero")
		return
	}
	settings, err := s.store.SaveAppSettings(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.config.BackupIntervalHours = settings.BackupIntervalHours
	s.config.BackupRetentionDays = settings.BackupRetentionDays
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
	s.recordAudit(r, rs, "settings.update", "settings", "dashboard", "", map[string]any{"dashboardDomain": settings.DashboardDomain, "acmeEmailSet": settings.LetsEncryptEmail != "", "traefik": traefikResult.Message})
	certificate := ResolveCertificateStatus(afterConfig, settings.DashboardDomain, settings.DashboardDomain != "")
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings, "network": DetectNetworkInfo(s.config.PublicIP, settings.DashboardDomain), "certificate": certificate, "traefik": traefikResult})
}

func (s *Server) testSMTPSettings(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	settings := s.store.LoadAppSettings(s.config)
	before := settings
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil {
		writeError(w, http.StatusBadRequest, "payload SMTP demasiado grande")
		return
	}
	if len(bytes.TrimSpace(body)) > 0 {
		var raw struct {
			SMTPEnabled   bool   `json:"smtpEnabled"`
			SMTPHost      string `json:"smtpHost"`
			SMTPPort      int    `json:"smtpPort"`
			SMTPSecurity  string `json:"smtpSecurity"`
			SMTPUsername  string `json:"smtpUsername"`
			SMTPPassword  string `json:"smtpPassword"`
			SMTPFromEmail string `json:"smtpFromEmail"`
			SMTPFromName  string `json:"smtpFromName"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			writeError(w, http.StatusBadRequest, "JSON invalido")
			return
		}
		settings.SMTPEnabled = raw.SMTPEnabled
		settings.SMTPHost = raw.SMTPHost
		settings.SMTPPort = raw.SMTPPort
		settings.SMTPSecurity = raw.SMTPSecurity
		settings.SMTPUsername = raw.SMTPUsername
		settings.SMTPPassword = raw.SMTPPassword
		settings.SMTPFromEmail = raw.SMTPFromEmail
		settings.SMTPFromName = raw.SMTPFromName
	}
	if strings.TrimSpace(settings.SMTPPassword) == "" {
		settings.SMTPPassword = before.SMTPPassword
	}
	settings.SMTPPasswordSet = strings.TrimSpace(settings.SMTPPassword) != ""
	settings.SMTPEnabled = true
	settings.Normalize()
	if err := validateSMTPConnectivity(settings); err != nil {
		writeError(w, http.StatusBadRequest, "SMTP no valido: "+err.Error())
		return
	}
	s.log.Info("conexion SMTP verificada", "user", rs.User.Username, "host", settings.SMTPHost)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Conexion SMTP verificada correctamente. Puedes guardar la configuracion."})
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
	settings := s.store.LoadAppSettings(s.config)
	if strings.TrimSpace(settings.DashboardDomain) == "" {
		settings.DashboardDomain = domain.Domain
		if saved, saveErr := s.store.SaveAppSettings(settings); saveErr == nil {
			settings = saved
			s.config.DashboardDomain = saved.DashboardDomain
			domain, _ = s.store.ManagedDomainByID(domain.ID)
			_ = s.applyTraefikDynamicOnly()
		} else if s.log != nil {
			s.log.Warn("no se pudo marcar dominio nuevo como principal", "domain", domain.Domain, "error", saveErr.Error())
		}
	}
	s.log.Info("dominio administrado creado", "id", domain.ID, "domain", domain.Domain, "user", rs.User.Username)
	s.recordAudit(r, rs, "domain.create", "domain", domain.ID, "", map[string]any{"domain": domain.Domain})
	writeJSON(w, http.StatusCreated, map[string]any{"domain": domain, "domains": s.store.ListManagedDomains(), "settings": settings, "network": DetectNetworkInfo(s.config.PublicIP, settings.DashboardDomain)})
}

func (s *Server) updateManagedDomain(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	defer r.Body.Close()
	var req struct {
		Status  string `json:"status"`
		Primary bool   `json:"primary"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	var (
		domain ManagedDomain
		err    error
	)
	status := strings.ToLower(strings.TrimSpace(req.Status))
	switch {
	case req.Primary:
		domain, err = s.store.ManagedDomainByID(id)
		if err == nil {
			if _, dnsErr := ValidateDashboardDomainDNS(domain.Domain, s.config.PublicIP); dnsErr != nil {
				err = dnsErr
				break
			}
			before := s.store.LoadAppSettings(s.config)
			next := before
			next.DashboardDomain = domain.Domain
			if _, saveErr := s.store.SaveAppSettings(next); saveErr != nil {
				err = saveErr
			} else {
				domain, err = s.store.ManagedDomainByID(id)
			}
		}
	case status == DomainStatusLegacy:
		domain, err = s.store.MarkManagedDomainLegacy(id)
	case status == DomainStatusActive:
		domain, err = s.store.ActivateManagedDomain(id)
	default:
		writeError(w, http.StatusBadRequest, "accion de dominio invalida")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	settings := s.store.LoadAppSettings(s.config)
	s.config.DashboardDomain = settings.DashboardDomain
	s.config.LetsEncryptEmail = settings.LetsEncryptEmail
	traefikResult := s.applyTraefikDynamicOnly()
	effective := s.store.EffectiveConfig(s.config)
	certificate := ResolveCertificateStatus(effective, settings.DashboardDomain, settings.DashboardDomain != "")
	s.log.Info("dominio administrado actualizado", "id", domain.ID, "domain", domain.Domain, "status", domain.Status, "primary", domain.Primary, "user", rs.User.Username)
	s.recordAudit(r, rs, "domain.update", "domain", domain.ID, "", map[string]any{"domain": domain.Domain, "status": domain.Status, "primary": domain.Primary, "traefik": traefikResult.Message})
	writeJSON(w, http.StatusOK, map[string]any{"domain": domain, "domains": s.store.ListManagedDomains(), "settings": settings, "network": DetectNetworkInfo(s.config.PublicIP, settings.DashboardDomain), "certificate": certificate, "traefik": traefikResult})
}

func (s *Server) deleteManagedDomain(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	domain, err := s.store.ManagedDomainByID(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	effective := s.store.EffectiveConfig(s.config)
	if strings.EqualFold(strings.TrimSpace(effective.DashboardDomain), strings.TrimSpace(domain.Domain)) {
		writeError(w, http.StatusBadRequest, "no se puede eliminar el dominio principal actual; configura otro dominio principal primero")
		return
	}
	if err := s.store.DeleteManagedDomain(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	traefikResult := s.applyTraefikDynamicOnly()
	s.log.Info("dominio administrado eliminado", "id", id, "user", rs.User.Username)
	s.recordAudit(r, rs, "domain.delete", "domain", id, "", map[string]any{"traefik": traefikResult.Message})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id, "domains": s.store.ListManagedDomains(), "traefik": traefikResult})
}

func (s *Server) listResources(w http.ResponseWriter, r *http.Request, _ requestSession) {
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	resources := s.store.ListResources()
	if projectID != "" {
		resources = s.store.ListResourcesByProject(projectID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"resources": resources})
}

func (s *Server) listSuspensionTemplates(w http.ResponseWriter, _ *http.Request, _ requestSession) {
	templates, err := ListSuspensionTemplates(s.config.SuspensionTemplateDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": templates, "variables": []string{"$nombredominio", "$dominio", "$nombrerecurso", "$recurso", "$proyecto", "$codigo", "$motivo", "$fecha"}})
}

func (s *Server) getSuspensionTemplate(w http.ResponseWriter, r *http.Request, _ requestSession) {
	template, err := ReadSuspensionTemplate(s.config.SuspensionTemplateDir, r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func (s *Server) createSuspensionTemplate(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var req struct {
		ID   string `json:"id"`
		HTML string `json:"html"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 160<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	template, err := SaveSuspensionTemplate(s.config.SuspensionTemplateDir, req.ID, req.HTML)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.recordAudit(r, rs, "template.create", "suspension_template", template.ID, "", map[string]any{"path": template.Path})
	writeJSON(w, http.StatusCreated, template)
}

func (s *Server) updateSuspensionTemplate(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var req struct {
		HTML string `json:"html"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 160<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	template, err := SaveSuspensionTemplate(s.config.SuspensionTemplateDir, r.PathValue("id"), req.HTML)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.recordAudit(r, rs, "template.update", "suspension_template", template.ID, "", map[string]any{"path": template.Path})
	writeJSON(w, http.StatusOK, template)
}

func (s *Server) createResource(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var resource Resource
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&resource); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	resource.Enabled = true
	if err := s.prepareResourceSecurity(&resource); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.validatePublicPortForCreate(resource); err != nil {
		s.log.Warn("validacion de puerto publico fallo", "mode", resource.Mode, "public_port", resource.PublicPort, "origin", resource.OriginType, "agent", resource.AgentID, "user", rs.User.Username, "error", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.validateHTTPSSL(resource); err != nil {
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
	s.recordAudit(r, rs, "resource.create", "resource", created.ID, created.ProjectID, map[string]any{"name": created.Name, "mode": created.Mode, "origin": created.OriginType, "publicPort": created.PublicPort, "agentId": created.AgentID, "traefik": traefikResult.Message})
	w.Header().Set("X-Pangolite-Traefik", traefikResult.Message)
	writeJSON(w, http.StatusCreated, map[string]any{"resource": created, "traefik": traefikResult})
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

func (s *Server) validateHTTPSSL(resource Resource) error {
	resource.Normalize(time.Now().UTC())
	if resource.Mode == ModeHTTP && resource.TLS && !ACMEEnabled(s.store.EffectiveConfig(s.config)) {
		return errors.New("para usar SSL configura primero un correo ACME real en Ajustes o desactiva Usar SSL")
	}
	return nil
}

func (s *Server) prepareResourceSecurity(resource *Resource) error {
	if resource == nil {
		return nil
	}
	resource.ProtectionMode = strings.ToLower(strings.TrimSpace(resource.ProtectionMode))
	resource.ProtectionLoginMode = strings.ToLower(strings.TrimSpace(resource.ProtectionLoginMode))
	if resource.ProtectionMode == "" {
		resource.ProtectionMode = ProtectionNone
	}
	if resource.ProtectionLoginMode == "" {
		resource.ProtectionLoginMode = ProtectionLoginHTML
	}
	if resource.ProtectionMode == ProtectionPassword {
		if strings.TrimSpace(resource.ProtectionPassword) != "" {
			hash, err := HashProtectionPassword(resource.ProtectionPassword)
			if err != nil {
				return err
			}
			resource.ProtectionHash = hash
		}
	} else {
		resource.ProtectionHash = ""
	}
	resource.ProtectionPassword = ""
	if resource.DisabledResponseMode == DisabledResponseHTML {
		if strings.TrimSpace(resource.DisabledTemplateID) != "" {
			if _, err := ReadSuspensionTemplate(s.config.SuspensionTemplateDir, resource.DisabledTemplateID); err != nil {
				return err
			}
		} else if strings.TrimSpace(resource.DisabledHTML) != "" {
			if err := ValidateSuspensionHTML(resource.DisabledHTML); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) hasHTTPSSLResources() bool {
	for _, resource := range s.store.ListResources() {
		if resource.Mode == ModeHTTP && resource.TLS {
			return true
		}
	}
	return false
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
	s.recordAudit(r, rs, "resource.delete", "resource", id, "", map[string]any{"traefik": traefikResult.Message})
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
		DisabledTemplateID   string `json:"disabledTemplateId"`
		ProtectionMode       string `json:"protectionMode"`
		ProtectionLoginMode  string `json:"protectionLoginMode"`
		ProtectionPassword   string `json:"protectionPassword"`
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
	fullEdit := req.Name != "" || req.Mode != "" || req.Domain != "" || req.PublicPort != 0 || req.BackendHost != "" || req.BackendPort != 0 || req.OriginType != "" || req.PathPrefix != "" || req.BackendScheme != "" || req.TLS != nil || req.ProjectID != "" || req.ProtectionMode != "" || req.ProtectionLoginMode != "" || req.ProtectionPassword != ""
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
		templateID := req.DisabledTemplateID
		if templateID == "" {
			templateID = current.DisabledTemplateID
		}
		if mode == DisabledResponseHTML {
			if templateID != "" {
				if _, err := ReadSuspensionTemplate(s.config.SuspensionTemplateDir, templateID); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
			} else if strings.TrimSpace(html) != "" {
				if err := ValidateSuspensionHTML(html); err != nil {
					writeError(w, http.StatusBadRequest, err.Error())
					return
				}
			}
		}
		updated, err := s.store.UpdateResourceControl(id, enabled, mode, status, html, templateID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		traefikResult := s.applyTraefikAfterResourceChange(beforeResources)
		s.log.Info("control de recurso actualizado", "id", id, "enabled", updated.Enabled, "mode", updated.DisabledResponseMode, "user", rs.User.Username, "traefik", traefikResult.Message)
		s.recordAudit(r, rs, "resource.control", "resource", updated.ID, updated.ProjectID, map[string]any{"enabled": updated.Enabled, "disabledResponseMode": updated.DisabledResponseMode, "traefik": traefikResult.Message})
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
	next.DisabledTemplateID = req.DisabledTemplateID
	if req.ProtectionMode != "" {
		next.ProtectionMode = req.ProtectionMode
	}
	if req.ProtectionLoginMode != "" {
		next.ProtectionLoginMode = req.ProtectionLoginMode
	}
	next.ProtectionPassword = req.ProtectionPassword
	if err := s.prepareResourceSecurity(&next); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	next.Normalize(time.Now().UTC())
	if err := s.validatePublicPortForUpdate(current, next); err != nil {
		s.log.Warn("validacion de puerto publico en edicion fallo", "resource", id, "mode", next.Mode, "public_port", next.PublicPort, "origin", next.OriginType, "agent", next.AgentID, "user", rs.User.Username, "error", err.Error())
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.validateHTTPSSL(next); err != nil {
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
	s.recordAudit(r, rs, "resource.update", "resource", updated.ID, updated.ProjectID, map[string]any{"name": updated.Name, "mode": updated.Mode, "origin": updated.OriginType, "publicPort": updated.PublicPort, "agentId": updated.AgentID, "traefik": traefikResult.Message})
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
	baseURL := strings.TrimRight(s.publicBaseURL(r), "/")
	fallbackURL := strings.TrimRight(s.publicIPBaseURL(r), "/")
	agent, err := s.store.AddAgent(Agent{ProjectID: req.ProjectID, Name: req.Name, ServerURL: baseURL, FallbackURL: fallbackURL, DomainID: s.domainIDForBaseURL(baseURL)})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.attachAgentCommands(r, &agent)
	s.log.Info("cliente NAT creado", "id", agent.ID, "name", agent.Name, "user", rs.User.Username)
	s.recordAudit(r, rs, "agent.create", "agent", agent.ID, agent.ProjectID, map[string]any{"name": agent.Name, "serverUrl": agent.ServerURL, "domainId": agent.DomainID})
	writeJSON(w, http.StatusCreated, agent)
}

func (s *Server) attachAgentCommands(r *http.Request, agent *Agent) {
	if agent == nil || agent.Token == "" {
		return
	}
	base := strings.TrimSpace(agent.ServerURL)
	if base == "" {
		base = s.publicBaseURL(r)
	}
	baseClean := strings.TrimRight(base, "/")
	fallbackClean := strings.TrimRight(strings.TrimSpace(agent.FallbackURL), "/")
	linuxBaseURL := baseClean + "/download/pangolite-client-linux"
	winURL := baseClean + "/download/pangolite-client-windows-amd64.exe"
	fallbackArg := ""
	if fallbackClean != "" && fallbackClean != baseClean {
		fallbackArg = " --fallback-url " + shellQuote(fallbackClean)
	}
	agent.InstallCommand = fmt.Sprintf("arch=$(uname -m); case \"$arch\" in x86_64|amd64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; i386|i486|i586|i686) arch=386 ;; armv7l|armv7) arch=armv7 ;; *) echo Arquitectura no soportada: $arch >&2; exit 1 ;; esac; curl -fsSL %s-$arch -o /tmp/pangolite-client && chmod +x /tmp/pangolite-client && sudo /tmp/pangolite-client --install --server-url %s%s --agent-id %s --token %s", shellQuote(linuxBaseURL), shellQuote(baseClean), fallbackArg, shellQuote(agent.ID), shellQuote(agent.Token))
	agent.RemoveCommand = "sudo /opt/pangolite-client/pangolite-client --remove"
	winFallbackArg := ""
	if fallbackClean != "" && fallbackClean != baseClean {
		winFallbackArg = " --fallback-url " + fallbackClean
	}
	agent.WindowsInstallCommand = fmt.Sprintf("$u=%q; $o=Join-Path $env:TEMP 'pangolite-client.exe'; Invoke-WebRequest -UseBasicParsing $u -OutFile $o; Start-Process -Verb RunAs $o -ArgumentList '--install --server-url %s%s --agent-id %s --token %s'", winURL, baseClean, winFallbackArg, agent.ID, agent.Token)
	agent.WindowsRemoveCommand = `Start-Process -Verb RunAs 'C:\ProgramData\Pangolite Client\pangolite-client.exe' -ArgumentList '--remove'`
}

func (s *Server) domainIDForBaseURL(base string) string {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return ""
	}
	if d, err := s.store.ManagedDomainByDomain(host); err == nil {
		return d.ID
	}
	return ""
}

func (s *Server) publicBaseURL(r *http.Request) string {
	effective := s.store.EffectiveConfig(s.config)
	if strings.TrimSpace(effective.DashboardDomain) != "" {
		return "https://" + strings.TrimSpace(effective.DashboardDomain)
	}
	if fallback := s.publicIPBaseURL(r); fallback != "" {
		return fallback
	}
	host := "127.0.0.1:2424"
	if r != nil && strings.TrimSpace(r.Host) != "" {
		host = cleanPublicHost(r.Host)
	}
	return "http://" + host
}

func (s *Server) publicIPBaseURL(r *http.Request) string {
	host := strings.TrimSpace(s.config.PublicIP)
	if host == "" && r != nil {
		candidate := cleanPublicHost(r.Host)
		if h, _, err := net.SplitHostPort(candidate); err == nil {
			candidate = h
		}
		candidate = strings.Trim(candidate, "[]")
		if ip := net.ParseIP(candidate); ip != nil && !ip.IsLoopback() {
			host = ip.String()
		}
	}
	if host == "" {
		return ""
	}
	port := ListenPortFromAddr(s.config.Addr)
	if port <= 0 {
		port = 2424
	}
	return "http://" + net.JoinHostPort(host, fmt.Sprint(port))
}

func cleanPublicHost(host string) string {
	host = strings.TrimSpace(host)
	host = strings.Trim(host, "[]")
	if strings.ContainsAny(host, " \t\n\r'\"`$;&|<>\\") {
		return "127.0.0.1:2424"
	}
	if host == "" || len(host) > 253 {
		return "127.0.0.1:2424"
	}
	return host
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func (s *Server) deleteAgent(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	defer r.Body.Close()
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	if !s.store.VerifyUserPassword(rs.User.ID, req.Password) {
		s.log.Warn("eliminacion de cliente rechazada por contraseña incorrecta", "id", id, "user", rs.User.Username)
		writeError(w, http.StatusUnauthorized, "contraseña incorrecta")
		return
	}
	beforeResources := s.store.ListResources()
	agent, deletedResources, err := s.store.DeleteAgentAndResources(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	traefikResult := s.applyTraefikAfterResourceChange(beforeResources)
	s.log.Info("cliente NAT eliminado", "id", id, "name", agent.Name, "resources", len(deletedResources), "user", rs.User.Username, "traefik", traefikResult.Message)
	s.recordAudit(r, rs, "agent.delete", "agent", id, agent.ProjectID, map[string]any{"name": agent.Name, "deletedResources": len(deletedResources), "traefik": traefikResult.Message})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id, "deletedResources": len(deletedResources), "traefik": traefikResult})
}

func (s *Server) updateAgentWebMaintenance(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	defer r.Body.Close()
	var req struct {
		Suspended            *bool  `json:"suspended"`
		DisabledResponseMode string `json:"disabledResponseMode"`
		DisabledStatusCode   int    `json:"disabledStatusCode"`
		DisabledHTML         string `json:"disabledHtml"`
		DisabledTemplateID   string `json:"disabledTemplateId"`
		Reason               string `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 192<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	if req.Suspended == nil {
		writeError(w, http.StatusBadRequest, "suspended requerido")
		return
	}
	req2 := agentMaintenanceRequest{Suspended: req.Suspended, Web: true, DisabledResponseMode: req.DisabledResponseMode, DisabledStatusCode: req.DisabledStatusCode, DisabledHTML: req.DisabledHTML, DisabledTemplateID: req.DisabledTemplateID, Reason: req.Reason}
	s.applyAgentMaintenanceRequest(w, r, rs, id, req2)
}

type agentMaintenanceRequest struct {
	Suspended            *bool  `json:"suspended"`
	Web                  bool   `json:"web"`
	TCP                  bool   `json:"tcp"`
	UDP                  bool   `json:"udp"`
	DisabledResponseMode string `json:"disabledResponseMode"`
	DisabledStatusCode   int    `json:"disabledStatusCode"`
	DisabledHTML         string `json:"disabledHtml"`
	DisabledTemplateID   string `json:"disabledTemplateId"`
	Reason               string `json:"reason"`
}

func (s *Server) updateAgentMaintenance(w http.ResponseWriter, r *http.Request, rs requestSession) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id requerido")
		return
	}
	defer r.Body.Close()
	var req agentMaintenanceRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 256<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	if req.Suspended == nil {
		writeError(w, http.StatusBadRequest, "suspended requerido")
		return
	}
	s.applyAgentMaintenanceRequest(w, r, rs, id, req)
}

func (s *Server) applyAgentMaintenanceRequest(w http.ResponseWriter, r *http.Request, rs requestSession, id string, req agentMaintenanceRequest) {
	if !req.Web && !req.TCP && !req.UDP {
		writeError(w, http.StatusBadRequest, "selecciona web, tcp o udp")
		return
	}
	beforeResources := s.store.ListResources()
	if !*req.Suspended {
		agent, resources, err := s.store.ResumeAgentResources(id, req.Web, req.TCP, req.UDP)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		traefikResult := s.applyTraefikAfterResourceChange(beforeResources)
		s.log.Info("mantenimiento de cliente reactivado", "id", id, "web", req.Web, "tcp", req.TCP, "udp", req.UDP, "user", rs.User.Username, "traefik", traefikResult.Message)
		s.recordAudit(r, rs, "agent.maintenance.resume", "agent", id, agent.ProjectID, map[string]any{"web": req.Web, "tcp": req.TCP, "udp": req.UDP, "resources": len(resources), "traefik": traefikResult.Message})
		writeJSON(w, http.StatusOK, map[string]any{"agent": agent, "resources": resources, "suspended": false, "traefik": traefikResult})
		return
	}
	webOpts := AgentWebMaintenanceOptions{ResponseMode: DisabledResponse403, StatusCode: 403}
	var err error
	if req.Web {
		webOpts, err = s.agentWebMaintenanceOptions(req.DisabledResponseMode, req.DisabledStatusCode, req.DisabledHTML, req.DisabledTemplateID, req.Reason)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	agent, resources, err := s.store.SuspendAgentResources(id, AgentMaintenanceOptions{Web: req.Web, TCP: req.TCP, UDP: req.UDP, ResponseMode: webOpts.ResponseMode, StatusCode: webOpts.StatusCode, HTML: webOpts.HTML, TemplateID: webOpts.TemplateID, Reason: webOpts.Reason})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	traefikResult := s.applyTraefikAfterResourceChange(beforeResources)
	affected := agent.WebSuspendedCount + agent.TCPSuspendedCount + agent.UDPSuspendedCount
	s.log.Info("mantenimiento de cliente activado", "id", id, "resources", affected, "web", req.Web, "tcp", req.TCP, "udp", req.UDP, "user", rs.User.Username, "traefik", traefikResult.Message)
	s.recordAudit(r, rs, "agent.maintenance.suspend", "agent", id, agent.ProjectID, map[string]any{"resources": affected, "web": req.Web, "tcp": req.TCP, "udp": req.UDP, "mode": webOpts.ResponseMode, "traefik": traefikResult.Message})
	writeJSON(w, http.StatusOK, map[string]any{"agent": agent, "resources": resources, "suspended": true, "affected": affected, "traefik": traefikResult})
}

func (s *Server) agentWebMaintenanceOptions(mode string, status int, html string, templateID string, reason string) (AgentWebMaintenanceOptions, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = DisabledResponseHTML
	}
	if mode != DisabledResponse403 && mode != DisabledResponse404 && mode != DisabledResponseHidden && mode != DisabledResponseHTML {
		return AgentWebMaintenanceOptions{}, errors.New("disabledResponseMode debe ser 403, 404, hidden o html")
	}
	templateID = strings.TrimSpace(templateID)
	html = strings.TrimSpace(html)
	if mode == DisabledResponse403 {
		status = 403
		html = ""
		templateID = ""
	}
	if mode == DisabledResponse404 || mode == DisabledResponseHidden {
		status = 404
		html = ""
		templateID = ""
	}
	if mode == DisabledResponseHTML {
		if status == 0 {
			status = 200
		}
		if status != 200 && status != 403 && status != 404 {
			return AgentWebMaintenanceOptions{}, errors.New("disabledStatusCode para html debe ser 200, 403 o 404")
		}
		if templateID != "" {
			if _, err := ReadSuspensionTemplate(s.config.SuspensionTemplateDir, templateID); err != nil {
				return AgentWebMaintenanceOptions{}, err
			}
		} else if html == "" {
			html = defaultAgentWebMaintenanceHTML()
		}
		if html != "" {
			if err := ValidateSuspensionHTML(html); err != nil {
				return AgentWebMaintenanceOptions{}, err
			}
		}
	}
	return AgentWebMaintenanceOptions{ResponseMode: mode, StatusCode: status, HTML: html, TemplateID: templateID, Reason: strings.TrimSpace(reason)}, nil
}

func defaultAgentWebMaintenanceHTML() string {
	return `<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Mantenimiento</title></head><body style="margin:0;min-height:100vh;display:grid;place-items:center;background:#0b1020;color:#f8fafc;font-family:system-ui,-apple-system,Segoe UI,sans-serif"><main style="max-width:680px;margin:24px;padding:36px;border:1px solid rgba(148,163,184,.28);border-radius:24px;background:rgba(15,23,42,.92);box-shadow:0 24px 80px rgba(0,0,0,.35)"><p style="margin:0 0 12px;color:#93c5fd;font-weight:700;letter-spacing:.08em;text-transform:uppercase">Mantenimiento programado</p><h1 style="margin:0 0 14px;font-size:clamp(28px,5vw,46px);line-height:1.05">Servicio web temporalmente pausado</h1><p style="margin:0;color:#cbd5e1;font-size:18px;line-height:1.6">Estamos realizando mantenimiento. Los túneles TCP/UDP permanecen disponibles para administración interna.</p></main></body></html>`
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
	baseURL := strings.TrimRight(s.publicBaseURL(r), "/")
	fallbackURL := strings.TrimRight(s.publicIPBaseURL(r), "/")
	domainID := s.domainIDForBaseURL(baseURL)
	if err := s.store.UpdateAgentInstallEndpoints(id, baseURL, fallbackURL, domainID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	agent.ServerURL = baseURL
	agent.FallbackURL = fallbackURL
	agent.DomainID = domainID
	s.attachAgentCommands(r, &agent)
	s.log.Info("token de cliente NAT rotado", "id", id, "user", rs.User.Username)
	s.recordAudit(r, rs, "agent.token.rotate", "agent", id, agent.ProjectID, map[string]any{"name": agent.Name, "serverUrl": agent.ServerURL, "fallbackUrl": agent.FallbackURL, "domainId": agent.DomainID})
	writeJSON(w, http.StatusOK, agent)
}

func (s *Server) renderTraefik(w http.ResponseWriter, r *http.Request, rs requestSession) {
	result := s.applyTraefikStaticAndRestart()
	if !result.OK {
		writeError(w, http.StatusBadRequest, result.Message)
		return
	}
	s.recordAudit(r, rs, "traefik.render", "traefik", "static", "", map[string]any{"message": result.Message, "restarted": result.Restarted})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": result.Message, "traefik": result})
}

type TraefikApplyResult struct {
	OK              bool   `json:"ok"`
	Message         string `json:"message"`
	Restarted       bool   `json:"restarted"`
	RestartRequired bool   `json:"restartRequired"`
	StaticChanged   bool   `json:"staticChanged"`
	DynamicChanged  bool   `json:"dynamicChanged"`
	ServiceManager  string `json:"serviceManager,omitempty"`
	Warning         string `json:"warning,omitempty"`
}

func (s *Server) applyTraefikDynamicOnly() TraefikApplyResult {
	if !s.config.AutoTraefik {
		return TraefikApplyResult{OK: true, Message: "Traefik automatico desactivado", DynamicChanged: false}
	}
	effective := s.store.EffectiveConfig(s.config)
	if err := RenderDynamicTraefikWithPanelDomains(effective, s.store.PanelDomainsForTraefik(effective.DashboardDomain)); err != nil {
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
	if err := RenderStaticTraefikWithPanelDomains(effective, s.store.ListResources(), s.store.PanelDomainsForTraefik(effective.DashboardDomain)); err != nil {
		s.log.Warn("no se pudo renderizar Traefik", "error", err.Error())
		return TraefikApplyResult{OK: false, Message: "no se pudo renderizar Traefik: " + err.Error()}
	}
	manager := DetectServiceManager()
	if !manager.Available() {
		return TraefikApplyResult{OK: true, Message: "configuracion estatica escrita; no se detecto gestor de servicios para reiniciar Traefik automaticamente", RestartRequired: true, StaticChanged: true, ServiceManager: manager.String(), Warning: "Este cambio agrego o retiro entrypoints TCP/UDP. Reinicia Traefik manualmente para aplicarlo."}
	}
	s.scheduleTraefikRestart("cambio de entrypoints TCP/UDP")
	return TraefikApplyResult{OK: true, Message: fmt.Sprintf("Traefik se reiniciara en segundo plano con %s para aplicar entrypoints TCP/UDP", manager.String()), Restarted: false, RestartRequired: true, StaticChanged: true, DynamicChanged: true, ServiceManager: manager.String(), Warning: "Para agregar o quitar tuneles TCP/UDP Traefik debe reiniciar su servicio global y las conexiones activas podrian cortarse unos segundos."}
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
		manager, err := RestartService(ctx, "traefik")
		if err != nil {
			if s.log != nil {
				s.log.Warn("reinicio de Traefik fallo", "reason", reason, "service_manager", manager, "error", err.Error())
			}
			return
		}
		if s.log != nil {
			s.log.Info("Traefik reiniciado automaticamente", "reason", reason, "service_manager", manager)
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

func (s *Server) checkResourceHealth(ctx context.Context, res Resource) (out ResourceHealth) {
	started := time.Now()
	out = ResourceHealth{ResourceID: res.ID, Name: res.Name, Mode: res.Mode, Status: "unknown", CheckedAt: started.UTC()}
	defer func() { out.LatencyMS = time.Since(started).Milliseconds() }()
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
			out.StatusCode = resp.StatusCode
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
		out.StatusCode = resp.StatusCode
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
		OS:          safeHeaderValue(r.Header.Get("X-Pangolite-Client-OS"), 32),
		Arch:        safeHeaderValue(r.Header.Get("X-Pangolite-Client-Arch"), 32),
		Hostname:    safeHeaderValue(r.Header.Get("X-Pangolite-Client-Hostname"), 120),
		PrivateIP:   safeHeaderValue(r.Header.Get("X-Pangolite-Client-Private-IP"), 64),
		PublicIP:    safeHeaderValue(requestPublicIP(r), 64),
		Version:     safeHeaderValue(r.Header.Get("X-Pangolite-Client-Version"), 48),
		LastError:   safeHeaderValue(r.Header.Get("X-Pangolite-Client-Last-Error"), 240),
		ServerURL:   safeHeaderValue(r.Header.Get("X-Pangolite-Client-Server-URL"), 500),
		FallbackURL: safeHeaderValue(r.Header.Get("X-Pangolite-Client-Fallback-URL"), 500),
	})
}

func safeHeaderValue(value string, max int) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, value)
	if max > 0 && len(value) > max {
		value = value[:max]
	}
	return value
}

func clientIPForRateLimit(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && strings.TrimSpace(host) != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
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

func (s *Server) downloadClientAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	allowed := map[string]string{
		"pangolite-client-linux-amd64":       envClientPath("PANGOLITE_CLIENT_LINUX_AMD64", "/opt/pangolite/public/pangolite-client-linux-amd64"),
		"pangolite-client-linux-arm64":       envClientPath("PANGOLITE_CLIENT_LINUX_ARM64", "/opt/pangolite/public/pangolite-client-linux-arm64"),
		"pangolite-client-linux-386":         envClientPath("PANGOLITE_CLIENT_LINUX_386", "/opt/pangolite/public/pangolite-client-linux-386"),
		"pangolite-client-linux-armv7":       envClientPath("PANGOLITE_CLIENT_LINUX_ARMV7", "/opt/pangolite/public/pangolite-client-linux-armv7"),
		"pangolite-client-windows-amd64.exe": envClientPath("PANGOLITE_CLIENT_WINDOWS_AMD64", "/opt/pangolite/public/pangolite-client-windows-amd64.exe"),
	}
	path, ok := allowed[name]
	if !ok {
		writeError(w, http.StatusNotFound, "cliente no disponible")
		return
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		writeError(w, http.StatusNotFound, "cliente no disponible en este servidor: "+name)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, name))
	http.ServeFile(w, r, path)
}

func envClientPath(key, fallback string) string {
	if path := strings.TrimSpace(os.Getenv(key)); path != "" {
		return path
	}
	return fallback
}

func (s *Server) agentDiscover(w http.ResponseWriter, r *http.Request) {
	agentID, token := agentCredentials(r)
	agent, ok := s.store.AuthenticateAgent(agentID, token)
	if !ok {
		writeError(w, http.StatusUnauthorized, "credenciales de cliente invalidas")
		return
	}
	s.touchAgentFromRequest(agentID, r)
	s.writeAgentEndpointHintHeaders(w, r)
	discovery := s.agentDiscoveryForRequest(r)
	if discovery.ServerURL == "" {
		discovery.ServerURL = strings.TrimRight(agent.ServerURL, "/")
	}
	if discovery.FallbackURL == "" {
		discovery.FallbackURL = strings.TrimRight(agent.FallbackURL, "/")
	}
	if discovery.ServerURL == "" && discovery.FallbackURL != "" {
		discovery.ServerURL = discovery.FallbackURL
	}
	if discovery.ServerURL == "" {
		writeError(w, http.StatusServiceUnavailable, "no hay URL publica disponible para el panel")
		return
	}
	writeJSON(w, http.StatusOK, discovery)
}

func (s *Server) agentDiscoveryForRequest(r *http.Request) AgentDiscovery {
	effective := s.store.EffectiveConfig(s.config)
	domain := strings.TrimSpace(effective.DashboardDomain)
	fallback := strings.TrimRight(s.publicIPBaseURL(r), "/")
	serverURL := ""
	if domain != "" {
		serverURL = "https://" + domain
	} else {
		serverURL = fallback
	}
	return AgentDiscovery{ServerURL: serverURL, FallbackURL: fallback, Domain: domain, PublicIP: strings.TrimSpace(s.config.PublicIP)}
}

func (s *Server) writeAgentEndpointHintHeaders(w http.ResponseWriter, r *http.Request) {
	discovery := s.agentDiscoveryForRequest(r)
	if discovery.ServerURL != "" {
		w.Header().Set("X-Pangolite-Server-URL", discovery.ServerURL)
	}
	if discovery.FallbackURL != "" {
		w.Header().Set("X-Pangolite-Fallback-URL", discovery.FallbackURL)
	}
	if discovery.Domain != "" {
		w.Header().Set("X-Pangolite-Domain", discovery.Domain)
	}
	if discovery.PublicIP != "" {
		w.Header().Set("X-Pangolite-Public-IP", discovery.PublicIP)
	}
}

func (s *Server) agentPoll(w http.ResponseWriter, r *http.Request) {
	agentID, token := agentCredentials(r)
	if _, ok := s.store.AuthenticateAgent(agentID, token); !ok {
		writeError(w, http.StatusUnauthorized, "credenciales de agente invalidas")
		return
	}
	s.touchAgentFromRequest(agentID, r)
	s.writeAgentEndpointHintHeaders(w, r)
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
	s.writeAgentEndpointHintHeaders(w, r)
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
	s.writeAgentEndpointHintHeaders(w, r)
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
		if !s.ensureResourceAccess(w, r, resource) {
			return
		}
		if resource.UsesAgent() {
			s.proxyViaAgent(w, r, resource)
			return
		}
		if resource.ProtectionMode != ProtectionNone {
			s.proxyLocalResource(w, r, resource)
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
	s.index(w, r, rs)
}

func (s *Server) serveDisabledResource(w http.ResponseWriter, r *http.Request, resource Resource) {
	status := resource.DisabledStatusCode
	if status == 0 {
		status = http.StatusForbidden
	}
	switch resource.DisabledResponseMode {
	case DisabledResponse404:
		http.NotFound(w, r)
	case DisabledResponseHidden:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		w.WriteHeader(http.StatusNotFound)
	case DisabledResponseHTML:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		if r.Method != http.MethodHead {
			htmlText := resource.DisabledHTML
			if strings.TrimSpace(resource.DisabledTemplateID) != "" {
				if tpl, err := ReadSuspensionTemplate(s.config.SuspensionTemplateDir, resource.DisabledTemplateID); err == nil {
					project, _ := s.store.ProjectByID(resource.ProjectID)
					htmlText = RenderSuspensionHTML(tpl.HTML, SuspensionTemplateVars{Resource: resource, Project: project, Status: status, Reason: "Recurso suspendido", Now: time.Now().UTC()})
				}
			}
			if strings.TrimSpace(htmlText) == "" {
				htmlText = defaultDisabledHTML(resource.Name, status)
			}
			_, _ = w.Write([]byte(htmlText))
		}
	default:
		http.Error(w, "403 recurso deshabilitado", http.StatusForbidden)
	}
}

type publicResourcePageData struct {
	Status       int
	ResourceName string
	Domain       string
	Message      string
	Action       string
}

func renderPublicResourceHTML(page string, data publicResourcePageData) string {
	t, err := template.ParseFS(templatesFS, "templates/public/"+page)
	if err != nil {
		return ""
	}
	var b bytes.Buffer
	if err := t.ExecuteTemplate(&b, "public_page", data); err != nil {
		return ""
	}
	return b.String()
}

func defaultDisabledHTML(name string, status int) string {
	if strings.TrimSpace(name) == "" {
		name = "Servicio no disponible"
	}
	if htmlText := renderPublicResourceHTML("disabled_default.html", publicResourcePageData{Status: status, ResourceName: name}); strings.TrimSpace(htmlText) != "" {
		return htmlText
	}
	return fmt.Sprintf("%d - %s", status, name)
}

func (s *Server) ensureResourceAccess(w http.ResponseWriter, r *http.Request, resource Resource) bool {
	if resource.ProtectionMode == ProtectionNone {
		return true
	}
	switch resource.ProtectionMode {
	case ProtectionSession:
		if rs, ok := s.currentSession(r); ok && !rs.User.ForcePasswordChange {
			return true
		}
		s.serveResourceSessionLogin(w, r, resource)
		return false
	case ProtectionPassword:
		if resource.ProtectionLoginMode == ProtectionLoginBasic {
			_, password, ok := r.BasicAuth()
			if ok && VerifyProtectionPassword(resource.ProtectionHash, password) {
				return true
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="Pangolite recurso protegido"`)
			writeError(w, http.StatusUnauthorized, "credenciales requeridas")
			return false
		}
		if s.hasResourcePasswordCookie(r, resource) {
			return true
		}
		if r.Method == http.MethodPost && strings.TrimSpace(r.FormValue("pangolite_resource_password")) != "" {
			if VerifyProtectionPassword(resource.ProtectionHash, r.FormValue("pangolite_resource_password")) {
				s.setResourcePasswordCookie(w, r, resource)
				http.Redirect(w, r, r.URL.Path, http.StatusFound)
				return false
			}
			s.serveResourcePasswordLogin(w, r, resource, "Contraseña incorrecta")
			return false
		}
		s.serveResourcePasswordLogin(w, r, resource, "")
		return false
	default:
		return true
	}
}

func (s *Server) resourceAccessCookieName(resource Resource) string {
	return "pangolite_resource_" + safeName(resource.ID)
}

func (s *Server) resourceAccessCookieValue(resource Resource) string {
	return hashToken(resource.ID + ":" + resource.ProtectionHash)
}

func (s *Server) hasResourcePasswordCookie(r *http.Request, resource Resource) bool {
	cookie, err := r.Cookie(s.resourceAccessCookieName(resource))
	if err != nil {
		return false
	}
	return cookie.Value == s.resourceAccessCookieValue(resource)
}

func (s *Server) setResourcePasswordCookie(w http.ResponseWriter, r *http.Request, resource Resource) {
	secure := r.TLS != nil
	if forwarded := strings.ToLower(r.Header.Get("X-Forwarded-Proto")); forwarded == "https" {
		secure = true
	}
	http.SetCookie(w, &http.Cookie{Name: s.resourceAccessCookieName(resource), Value: s.resourceAccessCookieValue(resource), Path: "/", MaxAge: 86400, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode})
}

func (s *Server) serveResourcePasswordLogin(w http.ResponseWriter, r *http.Request, resource Resource, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	if r.Method == http.MethodHead {
		return
	}
	msg := "Este recurso está protegido por contraseña."
	if message != "" {
		msg = message
	}
	_, _ = w.Write([]byte(renderPublicResourceHTML("resource_password.html", publicResourcePageData{ResourceName: resource.Name, Domain: resource.Domain, Message: msg, Action: r.URL.Path})))
}

func (s *Server) serveResourceSessionLogin(w http.ResponseWriter, r *http.Request, resource Resource) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(renderPublicResourceHTML("resource_session.html", publicResourcePageData{ResourceName: resource.Name, Domain: resource.Domain})))
}

func (s *Server) proxyLocalResource(w http.ResponseWriter, r *http.Request, resource Resource) {
	scheme := resource.BackendScheme
	if scheme == "" {
		scheme = "http"
	}
	target, err := url.Parse(fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(resource.BackendHost, fmt.Sprint(resource.BackendPort))))
	if err != nil {
		writeError(w, http.StatusBadGateway, "backend invalido")
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Proto", forwardedProto(r))
		stripInternalProxyHeaders(req.Header)
		if resource.ProtectionMode != ProtectionNone {
			req.Header.Del("Authorization")
		}
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		if s.log != nil {
			s.log.Warn("proxy local fallo", "resource", resource.ID, "error", err.Error())
		}
		writeError(w, http.StatusBadGateway, "backend no disponible")
	}
	proxy.ServeHTTP(w, r)
}

func forwardedProto(r *http.Request) string {
	if proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
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
		Header:       cloneProxyRequestHeader(r.Header),
		Body:         body,
		TargetScheme: resource.BackendScheme,
		TargetHost:   resource.BackendHost,
		TargetPort:   resource.BackendPort,
	}
	if resource.ProtectionMode != ProtectionNone {
		job.Header.Del("Authorization")
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

func (s *Server) index(w http.ResponseWriter, r *http.Request, rs requestSession) {
	page := panelPageForPath(r.URL.Path)
	renderUIPage(w, page.Template, s.panelData(r, rs))
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
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; img-src 'self' data:; font-src 'self' https://cdn.jsdelivr.net data:; connect-src 'self' https://cdn.jsdelivr.net; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
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
	return map[string]any{"id": u.ID, "username": u.Username, "email": u.Email, "forcePasswordChange": u.ForcePasswordChange}
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
