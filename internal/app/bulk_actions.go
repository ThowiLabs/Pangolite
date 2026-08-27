package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const maxBulkSelection = 200

type bulkActionRequest struct {
	IDs    []string `json:"ids"`
	Action string   `json:"action"`
}

type BulkActionResult struct {
	ID      string `json:"id"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

func normalizeBulkIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, errors.New("selecciona al menos un elemento")
	}
	if len(ids) > maxBulkSelection {
		return nil, fmt.Errorf("maximo %d elementos por accion masiva", maxBulkSelection)
	}
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if !idRe.MatchString(id) {
			return nil, errors.New("id invalido en seleccion masiva")
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, errors.New("seleccion vacia")
	}
	return out, nil
}

func (s *Store) BulkSetResourcesEnabled(ctx context.Context, projectID string, ids []string, enabled bool) ([]Resource, error) {
	projectID = strings.TrimSpace(projectID)
	if !idRe.MatchString(projectID) {
		return nil, errors.New("proyecto invalido")
	}
	ids, err := normalizeBulkIDs(ids)
	if err != nil {
		return nil, err
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+3)
	args = append(args, boolInt(enabled), formatTime(time.Now().UTC()), projectID)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("iniciar accion masiva: %w", err)
	}
	defer tx.Rollback()
	var found int
	countArgs := make([]any, 0, len(ids)+1)
	countArgs = append(countArgs, projectID)
	for _, id := range ids {
		countArgs = append(countArgs, id)
	}
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources WHERE project_id = ? AND id IN (`+strings.Join(placeholders, ",")+`)`, countArgs...).Scan(&found); err != nil {
		return nil, fmt.Errorf("validar recursos: %w", err)
	}
	if found != len(ids) {
		return nil, errors.New("uno o mas recursos no pertenecen al proyecto")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE resources SET enabled = ?, updated_at = ? WHERE project_id = ? AND id IN (`+strings.Join(placeholders, ",")+`)`, args...); err != nil {
		return nil, fmt.Errorf("actualizar recursos: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("confirmar accion masiva: %w", err)
	}
	selected := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		selected[id] = struct{}{}
	}
	out := make([]Resource, 0, len(ids))
	for _, resource := range s.ListResourcesByProject(projectID) {
		if _, ok := selected[resource.ID]; ok {
			out = append(out, resource)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *Store) ValidateAgentsInProject(projectID string, ids []string) ([]AgentPublic, error) {
	projectID = strings.TrimSpace(projectID)
	if !idRe.MatchString(projectID) {
		return nil, errors.New("proyecto invalido")
	}
	ids, err := normalizeBulkIDs(ids)
	if err != nil {
		return nil, err
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	out := make([]AgentPublic, 0, len(ids))
	for _, agent := range s.ListAgentsByProject(projectID) {
		if _, ok := wanted[agent.ID]; ok {
			out = append(out, agent)
			delete(wanted, agent.ID)
		}
	}
	if len(wanted) != 0 {
		return nil, errors.New("uno o mas clientes no pertenecen al proyecto")
	}
	return out, nil
}

func (s *Server) bulkProjectResources(w http.ResponseWriter, r *http.Request, rs requestSession) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	defer r.Body.Close()
	var req bulkActionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	ids, err := normalizeBulkIDs(req.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "health" {
		selected := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			selected[id] = struct{}{}
		}
		resources := make([]Resource, 0, len(ids))
		for _, resource := range s.store.ListResourcesByProject(projectID) {
			if _, ok := selected[resource.ID]; ok {
				resources = append(resources, resource)
				delete(selected, resource.ID)
			}
		}
		if len(selected) != 0 {
			writeError(w, http.StatusBadRequest, "uno o mas recursos no pertenecen al proyecto")
			return
		}
		checks := s.checkResourcesBounded(r.Context(), resources, 4)
		s.recordAudit(r, rs, "resource.bulk.health", "project", projectID, projectID, map[string]any{"count": len(ids)})
		writeJSON(w, http.StatusOK, map[string]any{"action": action, "checks": checks})
		return
	}
	if action != "activate" && action != "suspend" {
		writeError(w, http.StatusBadRequest, "accion masiva de recursos no soportada")
		return
	}
	before := s.store.ListResources()
	var reservations []*PublicL4Reservation
	if action == "activate" {
		selected := make(map[string]struct{}, len(ids))
		for _, id := range ids {
			selected[id] = struct{}{}
		}
		toActivate := make([]Resource, 0, len(ids))
		for _, resource := range before {
			if _, ok := selected[resource.ID]; !ok || resource.ProjectID != projectID || resource.Enabled {
				continue
			}
			resource.Enabled = true
			toActivate = append(toActivate, resource)
		}
		reservations, err = s.reservePublicL4Resources(toActivate)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		defer abortPublicL4Reservations(reservations)
	}
	updated, err := s.store.BulkSetResourcesEnabled(r.Context(), projectID, ids, action == "activate")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	commitPublicL4Reservations(reservations, updated)
	traefik := s.applyTraefikAfterResourceChange(before)
	s.recordAudit(r, rs, "resource.bulk."+action, "project", projectID, projectID, map[string]any{"count": len(updated), "ids": ids, "traefik": traefik.Message})
	writeJSON(w, http.StatusOK, map[string]any{"action": action, "resources": updated, "traefik": traefik})
}

func (s *Server) checkResourcesBounded(ctx context.Context, resources []Resource, concurrency int) []ResourceHealth {
	if concurrency < 1 {
		concurrency = 1
	}
	type item struct {
		index int
		check ResourceHealth
	}
	jobs := make(chan int)
	results := make(chan item, len(resources))
	workers := concurrency
	if workers > len(resources) {
		workers = len(resources)
	}
	for i := 0; i < workers; i++ {
		go func() {
			for idx := range jobs {
				results <- item{index: idx, check: s.checkResourceHealth(ctx, resources[idx])}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := range resources {
			select {
			case jobs <- i:
			case <-ctx.Done():
				return
			}
		}
	}()
	out := make([]ResourceHealth, len(resources))
	for i := 0; i < len(resources); i++ {
		select {
		case result := <-results:
			out[result.index] = result.check
		case <-ctx.Done():
			return out[:i]
		}
	}
	return out
}

func (s *Server) bulkProjectAgents(w http.ResponseWriter, r *http.Request, rs requestSession) {
	projectID := strings.TrimSpace(r.PathValue("id"))
	defer r.Body.Close()
	var req bulkActionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	ids, err := normalizeBulkIDs(req.IDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	agents, err := s.store.ValidateAgentsInProject(projectID, ids)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action != "maintenance" && action != "resume" && action != "reconnect" {
		writeError(w, http.StatusBadRequest, "accion masiva de clientes no soportada")
		return
	}
	before := s.store.ListResources()
	results := make([]BulkActionResult, 0, len(agents))
	for _, agent := range agents {
		result := BulkActionResult{ID: agent.ID, OK: true}
		switch action {
		case "maintenance":
			_, affected, err := s.store.SuspendAgentResources(agent.ID, AgentMaintenanceOptions{Web: true, TCP: true, UDP: true, ResponseMode: DisabledResponse403, StatusCode: 503, Reason: "Mantenimiento masivo del proyecto"})
			if err != nil {
				result.OK = false
				result.Message = err.Error()
			} else {
				result.Message = fmt.Sprintf("%d recurso(s) revisados", len(affected))
			}
		case "resume":
			toResume, err := s.store.ResourcesToResumeAgent(agent.ID, true, true, true)
			if err != nil {
				result.OK = false
				result.Message = err.Error()
				break
			}
			reservations, err := s.reservePublicL4Resources(toResume)
			if err != nil {
				result.OK = false
				result.Message = err.Error()
				break
			}
			_, affected, err := s.store.ResumeAgentResources(agent.ID, true, true, true)
			if err != nil {
				abortPublicL4Reservations(reservations)
				result.OK = false
				result.Message = err.Error()
			} else {
				commitPublicL4Reservations(reservations, affected)
				result.Message = fmt.Sprintf("%d recurso(s) revisados", len(affected))
			}
		case "reconnect":
			s.hub.RemoveAgent(agent.ID)
			result.Message = "streams y colas reiniciados; el cliente volvera a registrar su conexion"
		}
		results = append(results, result)
	}
	traefik := TraefikApplyResult{OK: true, Message: "Sin cambios de Traefik"}
	if action == "maintenance" || action == "resume" {
		traefik = s.applyTraefikAfterResourceChange(before)
	}
	s.recordAudit(r, rs, "agent.bulk."+action, "project", projectID, projectID, map[string]any{"count": len(agents), "ids": ids, "traefik": traefik.Message})
	writeJSON(w, http.StatusOK, map[string]any{"action": action, "results": results, "traefik": traefik})
}
