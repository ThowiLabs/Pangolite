package app

import (
	"net/http"
	"strings"
)

type panelRouteDefinition struct {
	Pattern       string
	Page          panelPage
	ProjectScoped bool
	Terminal      bool
}

var panelRouteDefinitions = []panelRouteDefinition{
	{Pattern: "GET /{$}", Page: panelPage{Template: "projects.html", Title: "Pangolite - Proyectos", Key: "projects", Crumb: "Dashboard", Heading: "Operación global"}},
	{Pattern: "GET /projects", Page: panelPage{Template: "projects.html", Title: "Pangolite - Proyectos", Key: "projects", Crumb: "Dashboard", Heading: "Operación global"}},
	{Pattern: "GET /projects/{id}", Page: panelPage{Template: "project_overview.html", Title: "Pangolite - Proyecto", Key: "project", Crumb: "Proyecto", Heading: "Resumen"}, ProjectScoped: true},
	{Pattern: "GET /projects/{id}/resources", Page: panelPage{Template: "resources.html", Title: "Pangolite - Recursos", Key: "resources", Crumb: "Proyecto", Heading: "Recursos"}, ProjectScoped: true},
	{Pattern: "GET /projects/{id}/resources/create", Page: panelPage{Template: "resource_create.html", Title: "Pangolite - Crear recurso", Key: "resource_create", Crumb: "Proyecto", Heading: "Crear recurso"}, ProjectScoped: true},
	{Pattern: "GET /projects/{id}/agents", Page: panelPage{Template: "agents.html", Title: "Pangolite - Clientes de sistema", Key: "agents", Crumb: "Proyecto", Heading: "Clientes de sistema"}, ProjectScoped: true},
	{Pattern: "GET /projects/{id}/agents/create", Page: panelPage{Template: "agent_create.html", Title: "Pangolite - Crear cliente de sistema", Key: "agent_create", Crumb: "Proyecto", Heading: "Crear cliente de sistema"}, ProjectScoped: true},
	{Pattern: "GET /logs", Page: panelPage{Template: "logs.html", Title: "Pangolite - Logs", Key: "logs", Crumb: "Logs", Heading: "Diagnóstico del sistema"}},
	{Pattern: "GET /maintenance", Page: panelPage{Template: "maintenance.html", Title: "Pangolite - Seguridad", Key: "maintenance", Crumb: "Seguridad", Heading: "Auditoría y respaldos"}},
	{Pattern: "GET /settings", Page: panelPage{Template: "settings.html", Title: "Pangolite - Ajustes", Key: "settings", Crumb: "Ajustes", Heading: "Configuración del sistema"}},
	{Pattern: "GET /perfil", Page: panelPage{Template: "profile.html", Title: "Pangolite - Mi perfil", Key: "profile", Crumb: "Mi cuenta", Heading: "Perfil y seguridad"}},
	{Pattern: "GET /ssh", Page: panelPage{Template: "ssh_connections.html", Title: "Pangolite - Conexiones SSH", Key: "ssh_connections", Crumb: "Acceso remoto", Heading: "Conexiones SSH"}},
	{Pattern: "GET /terminal", Page: panelPage{Template: "terminal.html", Title: "Pangolite - Terminal", Key: "terminal", Crumb: "Terminal", Heading: "Consola web"}, Terminal: true},
}

func (s *Server) registerPanelRoutes() {
	for _, definition := range panelRouteDefinitions {
		definition := definition
		s.mux.HandleFunc(definition.Pattern, s.panelRouteHandler(definition))
	}
}

func (s *Server) panelRouteHandler(definition panelRouteDefinition) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rs, ok := s.currentSession(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if rs.User.ForcePasswordChange {
			http.Redirect(w, r, "/password", http.StatusFound)
			return
		}
		if definition.ProjectScoped && !s.validateProjectPanelRoute(w, r) {
			return
		}
		if definition.Terminal && !s.validateTerminalPanelRoute(w, r) {
			return
		}
		renderUIPage(w, definition.Page.Template, s.panelData(r, rs, definition.Page))
	}
}

func (s *Server) validateProjectPanelRoute(w http.ResponseWriter, r *http.Request) bool {
	rawID := strings.TrimSpace(r.PathValue("id"))
	if rawID == "" {
		http.NotFound(w, r)
		return false
	}
	if _, err := s.store.ResolveProjectID(rawID); err != nil {
		http.NotFound(w, r)
		return false
	}
	return true
}

func (s *Server) validateTerminalPanelRoute(w http.ResponseWriter, r *http.Request) bool {
	query := r.URL.Query()
	target := strings.TrimSpace(query.Get("target"))
	agentID := strings.TrimSpace(query.Get("agentId"))
	projectID := strings.TrimSpace(query.Get("projectId"))

	if target != "" && target != "local" {
		http.Error(w, "destino de terminal inválido", http.StatusBadRequest)
		return false
	}
	if target == "local" && agentID != "" {
		http.Error(w, "la terminal local no admite agentId", http.StatusBadRequest)
		return false
	}

	var resolvedProjectID string
	if projectID != "" {
		var err error
		resolvedProjectID, err = s.store.ResolveProjectID(projectID)
		if err != nil {
			http.NotFound(w, r)
			return false
		}
	}
	if agentID != "" {
		agent, err := s.store.AgentByID(agentID)
		if err != nil {
			http.NotFound(w, r)
			return false
		}
		if resolvedProjectID != "" && agent.ProjectID != resolvedProjectID {
			http.Error(w, "el cliente no pertenece al proyecto indicado", http.StatusBadRequest)
			return false
		}
	}
	return true
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "ruta no encontrada")
		return
	}
	http.NotFound(w, r)
}
