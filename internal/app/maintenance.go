package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultAuditLimit = 200

type AuditEvent struct {
	ID         int64     `json:"id"`
	Action     string    `json:"action"`
	EntityType string    `json:"entityType"`
	EntityID   string    `json:"entityId,omitempty"`
	ProjectID  string    `json:"projectId,omitempty"`
	Username   string    `json:"username"`
	RemoteIP   string    `json:"remoteIp,omitempty"`
	Metadata   string    `json:"metadata,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

type BackupInfo struct {
	Name      string    `json:"name"`
	SizeBytes int64     `json:"sizeBytes"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Store) RecordAudit(ctx context.Context, ev AuditEvent) error {
	if s == nil || s.db == nil {
		return nil
	}
	ev.Action = strings.TrimSpace(ev.Action)
	ev.EntityType = strings.TrimSpace(ev.EntityType)
	ev.EntityID = strings.TrimSpace(ev.EntityID)
	ev.ProjectID = strings.TrimSpace(ev.ProjectID)
	ev.Username = strings.TrimSpace(ev.Username)
	ev.RemoteIP = strings.TrimSpace(ev.RemoteIP)
	ev.Metadata = strings.TrimSpace(ev.Metadata)
	if ev.Action == "" || ev.EntityType == "" {
		return nil
	}
	if ev.Username == "" {
		ev.Username = "system"
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO audit_events (action, entity_type, entity_id, project_id, username, remote_ip, metadata, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, ev.Action, ev.EntityType, ev.EntityID, ev.ProjectID, ev.Username, ev.RemoteIP, ev.Metadata, formatTime(ev.CreatedAt))
	return err
}

func (s *Store) ListAuditEvents(limit int, projectID string) ([]AuditEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = defaultAuditLimit
	}
	projectID = strings.TrimSpace(projectID)
	query := `SELECT id, action, entity_type, entity_id, project_id, username, remote_ip, metadata, created_at FROM audit_events`
	args := []any{}
	if projectID != "" {
		query += ` WHERE project_id = ?`
		args = append(args, projectID)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var ev AuditEvent
		var created string
		if err := rows.Scan(&ev.ID, &ev.Action, &ev.EntityType, &ev.EntityID, &ev.ProjectID, &ev.Username, &ev.RemoteIP, &ev.Metadata, &created); err != nil {
			return nil, err
		}
		ev.CreatedAt = parseTime(created)
		out = append(out, ev)
	}
	return out, rows.Err()
}

func (s *Store) CreateBackup(ctx context.Context, backupDir, label string) (BackupInfo, error) {
	if s == nil || s.db == nil {
		return BackupInfo{}, sql.ErrConnDone
	}
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return BackupInfo{}, fmt.Errorf("crear directorio de respaldos: %w", err)
	}
	name := backupFilename(label, time.Now().UTC())
	path := filepath.Join(backupDir, name)
	if err := ensureInsideDir(backupDir, path); err != nil {
		return BackupInfo{}, err
	}
	if _, err := os.Stat(path); err == nil {
		return BackupInfo{}, fmt.Errorf("el respaldo ya existe: %s", name)
	}
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO `+sqliteStringLiteral(path)); err != nil {
		return BackupInfo{}, fmt.Errorf("crear respaldo SQLite: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return BackupInfo{}, err
	}
	return BackupInfo{Name: info.Name(), SizeBytes: info.Size(), CreatedAt: info.ModTime().UTC()}, nil
}

func ListBackups(backupDir string) ([]BackupInfo, error) {
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return nil, fmt.Errorf("crear directorio de respaldos: %w", err)
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}
	out := []BackupInfo{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".db") || !strings.HasPrefix(entry.Name(), "pangolite-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupInfo{Name: entry.Name(), SizeBytes: info.Size(), CreatedAt: info.ModTime().UTC()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func BackupPath(backupDir, name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "" || strings.Contains(name, string(os.PathSeparator)) || !strings.HasSuffix(name, ".db") || !strings.HasPrefix(name, "pangolite-") {
		return "", fmt.Errorf("nombre de respaldo invalido")
	}
	path := filepath.Join(backupDir, name)
	if err := ensureInsideDir(backupDir, path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request, _ requestSession) {
	limit := defaultAuditLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	events, err := s.store.ListAuditEvents(limit, r.URL.Query().Get("projectId"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *Server) listBackups(w http.ResponseWriter, _ *http.Request, _ requestSession) {
	backups, err := ListBackups(s.config.BackupDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"backupDir": s.config.BackupDir, "backups": backups})
}

func (s *Server) createBackup(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var req struct {
		Label string `json:"label"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req)
	}
	backup, err := s.store.CreateBackup(r.Context(), s.config.BackupDir, req.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.recordAudit(r, rs, "backup.create", "backup", backup.Name, "", map[string]any{"sizeBytes": backup.SizeBytes})
	if s.log != nil {
		s.log.Info("respaldo SQLite creado", "name", backup.Name, "user", rs.User.Username, "size", backup.SizeBytes)
	}
	writeJSON(w, http.StatusCreated, backup)
}

func (s *Server) downloadBackup(w http.ResponseWriter, r *http.Request, _ requestSession) {
	name := r.PathValue("name")
	path, err := BackupPath(s.config.BackupDir, name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := os.Stat(path); err != nil {
		writeError(w, http.StatusNotFound, "respaldo no encontrado")
		return
	}
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	http.ServeFile(w, r, path)
}

func (s *Server) recordAudit(r *http.Request, rs requestSession, action, entityType, entityID, projectID string, metadata map[string]any) {
	if s == nil || s.store == nil {
		return
	}
	meta := ""
	if len(metadata) > 0 {
		b, err := json.Marshal(metadata)
		if err == nil {
			meta = string(b)
		}
	}
	remote := ""
	if r != nil {
		remote = firstClientIP(r)
	}
	if err := s.store.RecordAudit(context.Background(), AuditEvent{Action: action, EntityType: entityType, EntityID: entityID, ProjectID: projectID, Username: rs.User.Username, RemoteIP: remote, Metadata: meta}); err != nil && s.log != nil {
		s.log.Warn("no se pudo registrar auditoria", "action", action, "entity", entityType, "id", entityID, "error", err.Error())
	}
}

func firstClientIP(r *http.Request) string {
	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value == "" {
			continue
		}
		if i := strings.IndexByte(value, ','); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		return value
	}
	return r.RemoteAddr
}

func backupFilename(label string, now time.Time) string {
	label = strings.ToLower(strings.TrimSpace(label))
	var b strings.Builder
	lastDash := false
	for _, r := range label {
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
		if b.Len() >= 48 {
			break
		}
	}
	suffix := strings.Trim(b.String(), "-")
	name := "pangolite-" + now.Format("20060102-150405")
	if suffix != "" {
		name += "-" + suffix
	}
	return name + ".db"
}

func sqliteStringLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func ensureInsideDir(dir, path string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return err
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("ruta fuera del directorio permitido")
	}
	return nil
}
