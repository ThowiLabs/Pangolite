package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type Store struct {
	path string
	db   *sql.DB
}

func NewStore(path string) (*Store, error) {
	if err := EnsureDirForFile(path); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("abrir SQLite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{path: path, db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			force_password_change INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id_hash TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			csrf_token TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			last_seen TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`,
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			slug TEXT NOT NULL UNIQUE,
			notes TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS managed_domains (
			id TEXT PRIMARY KEY,
			domain TEXT NOT NULL UNIQUE,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_managed_domains_domain_unique ON managed_domains(lower(domain))`,
		`CREATE TABLE IF NOT EXISTS app_settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			token_hash TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_seen TEXT,
			os TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT '',
			hostname TEXT NOT NULL DEFAULT '',
			public_ip TEXT NOT NULL DEFAULT '',
			private_ip TEXT NOT NULL DEFAULT '',
			version TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS resources (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			name TEXT NOT NULL,
			mode TEXT NOT NULL,
			domain TEXT,
			path_prefix TEXT,
			public_port INTEGER,
			tunnel_port INTEGER,
			backend_scheme TEXT,
			backend_host TEXT NOT NULL,
			backend_port INTEGER NOT NULL,
			origin_type TEXT NOT NULL DEFAULT 'local',
			agent_id TEXT REFERENCES agents(id) ON DELETE SET NULL,
			tls INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			disabled_response_mode TEXT NOT NULL DEFAULT '403',
			disabled_status_code INTEGER NOT NULL DEFAULT 403,
			disabled_html TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_resources_http_unique ON resources(domain, path_prefix) WHERE enabled = 1 AND mode = 'http'`,
		`DROP INDEX IF EXISTS idx_resources_port_unique`,
		`CREATE INDEX IF NOT EXISTS idx_resources_port_lookup ON resources(mode, public_port) WHERE COALESCE(public_port,0) > 0 AND mode IN ('tcp','udp')`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			action TEXT NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id TEXT NOT NULL DEFAULT '',
			project_id TEXT NOT NULL DEFAULT '',
			username TEXT NOT NULL DEFAULT '',
			remote_ip TEXT NOT NULL DEFAULT '',
			metadata TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_created_at ON audit_events(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_events_project_id ON audit_events(project_id)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrar SQLite: %w", err)
		}
	}
	if err := s.ensureColumn(ctx, "agents", "project_id", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
		return err
	}
	for _, col := range []struct{ name, def string }{
		{"os", "TEXT NOT NULL DEFAULT ''"},
		{"arch", "TEXT NOT NULL DEFAULT ''"},
		{"hostname", "TEXT NOT NULL DEFAULT ''"},
		{"public_ip", "TEXT NOT NULL DEFAULT ''"},
		{"private_ip", "TEXT NOT NULL DEFAULT ''"},
		{"version", "TEXT NOT NULL DEFAULT ''"},
		{"last_error", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.ensureColumn(ctx, "agents", col.name, col.def); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "resources", "project_id", "TEXT NOT NULL DEFAULT 'default'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "resources", "origin_type", "TEXT NOT NULL DEFAULT 'local'"); err != nil {
		return err
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE resources SET origin_type = CASE WHEN COALESCE(agent_id,'') <> '' THEN 'agent' ELSE 'local' END WHERE origin_type IS NULL OR origin_type = ''`)
	if err := s.ensureLegacyProject(ctx); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_resources_project_id ON resources(project_id)`); err != nil {
		return fmt.Errorf("crear indice resources.project_id: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_agents_project_id ON agents(project_id)`); err != nil {
		return fmt.Errorf("crear indice agents.project_id: %w", err)
	}
	if err := s.ensureColumn(ctx, "resources", "disabled_response_mode", "TEXT NOT NULL DEFAULT '403'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "resources", "disabled_status_code", "INTEGER NOT NULL DEFAULT 403"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "resources", "disabled_html", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "resources", "tunnel_port", "INTEGER"); err != nil {
		return err
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return fmt.Errorf("revisar columnas de %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("leer columnas de %s: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition); err != nil {
		return fmt.Errorf("agregar columna %s.%s: %w", table, column, err)
	}
	return nil
}

func (s *Store) ensureLegacyProject(ctx context.Context) error {
	var orphanAgents, orphanResources, defaultAgents, defaultResources int
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agents WHERE project_id IS NULL OR project_id = ''`).Scan(&orphanAgents)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources WHERE project_id IS NULL OR project_id = ''`).Scan(&orphanResources)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agents WHERE project_id = 'default'`).Scan(&defaultAgents)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM resources WHERE project_id = 'default'`).Scan(&defaultResources)
	if orphanAgents+orphanResources+defaultAgents+defaultResources == 0 {
		return nil
	}
	now := formatTime(time.Now().UTC())
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO projects(id,name,slug,notes,enabled,created_at,updated_at) VALUES('default','General','general','Proyecto legado para recursos existentes antes del onboarding.',1,?,?)`, now, now)
	if err != nil {
		return fmt.Errorf("crear proyecto legado: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `UPDATE agents SET project_id = 'default' WHERE project_id IS NULL OR project_id = ''`)
	_, _ = s.db.ExecContext(ctx, `UPDATE resources SET project_id = 'default' WHERE project_id IS NULL OR project_id = ''`)
	return nil
}

func (s *Store) EnsureManagedDomain(domain string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" || domain == "pangolite.localhost" || strings.HasSuffix(domain, ".localhost") {
		return nil
	}
	d := ManagedDomain{Domain: domain, Enabled: true}
	d.Normalize(time.Now().UTC())
	if err := d.Validate(); err != nil {
		return nil
	}
	if d.ID == "" {
		id, err := randomID()
		if err != nil {
			return err
		}
		d.ID = id
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO managed_domains(id,domain,enabled,created_at,updated_at) VALUES(?,?,?,?,?)`, d.ID, d.Domain, boolInt(d.Enabled), formatTime(d.CreatedAt), formatTime(d.UpdatedAt))
	if err != nil {
		return fmt.Errorf("asegurar dominio administrado: %w", err)
	}
	return nil
}

func (s *Store) LoadAppSettings(c Config) AppSettings {
	settings := AppSettings{
		DashboardDomain:  strings.TrimSpace(c.DashboardDomain),
		LetsEncryptEmail: strings.TrimSpace(c.LetsEncryptEmail),
	}
	if v, ok := s.getSetting("dashboard_domain"); ok {
		settings.DashboardDomain = v
	}
	if v, ok := s.getSetting("lets_encrypt_email"); ok {
		settings.LetsEncryptEmail = v
	}
	settings.Normalize()
	return settings
}

func (s *Store) EffectiveConfig(c Config) Config {
	settings := s.LoadAppSettings(c)
	c.DashboardDomain = settings.DashboardDomain
	c.LetsEncryptEmail = settings.LetsEncryptEmail
	return c
}

func (s *Store) SaveAppSettings(settings AppSettings) (AppSettings, error) {
	settings.Normalize()
	if err := settings.Validate(); err != nil {
		return AppSettings{}, err
	}
	now := formatTime(time.Now().UTC())
	tx, err := s.db.Begin()
	if err != nil {
		return AppSettings{}, fmt.Errorf("abrir transaccion de ajustes: %w", err)
	}
	defer tx.Rollback()
	pairs := map[string]string{
		"dashboard_domain":   settings.DashboardDomain,
		"lets_encrypt_email": settings.LetsEncryptEmail,
	}
	for key, value := range pairs {
		if _, err := tx.Exec(`INSERT INTO app_settings(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, now); err != nil {
			return AppSettings{}, fmt.Errorf("guardar ajuste %s: %w", key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return AppSettings{}, fmt.Errorf("confirmar ajustes: %w", err)
	}
	if settings.DashboardDomain != "" {
		_ = s.EnsureManagedDomain(settings.DashboardDomain)
	}
	return settings, nil
}

func (s *Store) getSetting(key string) (string, bool) {
	var value string
	if err := s.db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value); err != nil {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func (s *Store) BootstrapAdmin(username, passwordFile string) (created bool, tempPassword string, err error) {
	username = NormalizeUsername(username)
	if err := ValidateUsername(username); err != nil {
		return false, "", err
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return false, "", fmt.Errorf("contar usuarios: %w", err)
	}
	if total > 0 {
		return false, "", nil
	}
	tempPassword, err = newSecret(18)
	if err != nil {
		return false, "", err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		return false, "", fmt.Errorf("hashear password inicial: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := s.db.Exec(`INSERT INTO users(username,password_hash,force_password_change,created_at,updated_at) VALUES(?,?,?,?,?)`, username, string(hash), 1, now, now); err != nil {
		return false, "", fmt.Errorf("crear admin inicial: %w", err)
	}
	if passwordFile != "" {
		if err := EnsureDirForFile(passwordFile); err != nil {
			return false, "", err
		}
		content := fmt.Sprintf("usuario=%s\npassword=%s\n", username, tempPassword)
		if err := os.WriteFile(passwordFile, []byte(content), 0o600); err != nil {
			return false, "", fmt.Errorf("escribir password temporal: %w", err)
		}
	}
	return true, tempPassword, nil
}

func (s *Store) AuthenticateUser(username, password string) (User, bool) {
	username = NormalizeUsername(username)
	if username == "" || password == "" {
		return User{}, false
	}
	user, err := s.UserByUsername(username)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoOHI33oXb6uOCSBTCDi1i7N4tStcWmYOO"), []byte(password))
		return User{}, false
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return User{}, false
	}
	return user, true
}

func (s *Store) VerifyUserPassword(userID int64, password string) bool {
	if password == "" {
		return false
	}
	user, err := s.UserByID(userID)
	if err != nil {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$7EqJtq98hPqEX7fNZaFWoOHI33oXb6uOCSBTCDi1i7N4tStcWmYOO"), []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) == nil
}

func (s *Store) UserByUsername(username string) (User, error) {
	username = NormalizeUsername(username)
	return scanUser(s.db.QueryRow(`SELECT id, username, password_hash, force_password_change, created_at, updated_at FROM users WHERE username = ?`, username))
}

func (s *Store) UserByID(id int64) (User, error) {
	return scanUser(s.db.QueryRow(`SELECT id, username, password_hash, force_password_change, created_at, updated_at FROM users WHERE id = ?`, id))
}

func (s *Store) ChangePassword(userID int64, currentPassword, newPassword string, requireCurrent bool) error {
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	user, err := s.UserByID(userID)
	if err != nil {
		return errors.New("usuario no encontrado")
	}
	if requireCurrent && bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return errors.New("contraseña actual incorrecta")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashear nueva contraseña: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.Exec(`UPDATE users SET password_hash = ?, force_password_change = 0, updated_at = ? WHERE id = ?`, string(hash), now, userID)
	return err
}

func (s *Store) CreateSession(userID int64, ttl time.Duration) (rawID string, session Session, err error) {
	rawID, err = newSecret(32)
	if err != nil {
		return "", Session{}, err
	}
	csrf, err := newSecret(32)
	if err != nil {
		return "", Session{}, err
	}
	now := time.Now().UTC()
	session = Session{IDHash: hashToken(rawID), UserID: userID, CSRFToken: csrf, CreatedAt: now, LastSeen: now, ExpiresAt: now.Add(ttl)}
	_, err = s.db.Exec(`INSERT INTO sessions(id_hash,user_id,csrf_token,expires_at,created_at,last_seen) VALUES(?,?,?,?,?,?)`, session.IDHash, session.UserID, session.CSRFToken, formatTime(session.ExpiresAt), formatTime(session.CreatedAt), formatTime(session.LastSeen))
	if err != nil {
		return "", Session{}, fmt.Errorf("crear sesion: %w", err)
	}
	return rawID, session, nil
}

func (s *Store) SessionWithUser(rawID string) (Session, User, bool) {
	if rawID == "" {
		return Session{}, User{}, false
	}
	now := time.Now().UTC()
	row := s.db.QueryRow(`SELECT s.id_hash, s.user_id, s.csrf_token, s.expires_at, s.created_at, s.last_seen,
		u.id, u.username, u.password_hash, u.force_password_change, u.created_at, u.updated_at
		FROM sessions s JOIN users u ON u.id = s.user_id WHERE s.id_hash = ?`, hashToken(rawID))
	var sess Session
	var user User
	var expiresAt, sessCreated, lastSeen, userCreated, userUpdated string
	var force int
	if err := row.Scan(&sess.IDHash, &sess.UserID, &sess.CSRFToken, &expiresAt, &sessCreated, &lastSeen, &user.ID, &user.Username, &user.PasswordHash, &force, &userCreated, &userUpdated); err != nil {
		return Session{}, User{}, false
	}
	sess.ExpiresAt = parseTime(expiresAt)
	if !sess.ExpiresAt.After(now) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE id_hash = ?`, sess.IDHash)
		return Session{}, User{}, false
	}
	sess.CreatedAt = parseTime(sessCreated)
	sess.LastSeen = parseTime(lastSeen)
	user.ForcePasswordChange = force == 1
	user.CreatedAt = parseTime(userCreated)
	user.UpdatedAt = parseTime(userUpdated)
	if now.Sub(sess.LastSeen) > time.Minute {
		_, _ = s.db.Exec(`UPDATE sessions SET last_seen = ? WHERE id_hash = ?`, formatTime(now), sess.IDHash)
	}
	return sess, user, true
}

func (s *Store) DeleteSession(rawID string) {
	if rawID == "" {
		return
	}
	_, _ = s.db.Exec(`DELETE FROM sessions WHERE id_hash = ?`, hashToken(rawID))
}

func (s *Store) DeleteExpiredSessions() {
	_, _ = s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, formatTime(time.Now().UTC()))
}

func (s *Store) ListProjects() []Project {
	rows, err := s.db.Query(`SELECT id,name,slug,notes,enabled,created_at,updated_at FROM projects ORDER BY created_at ASC`)
	if err != nil {
		return []Project{}
	}
	defer rows.Close()
	out := []Project{}
	for rows.Next() {
		p, err := scanProjectRows(rows)
		if err == nil {
			out = append(out, p)
		}
	}
	return out
}

func (s *Store) ProjectByID(id string) (Project, error) {
	id = strings.TrimSpace(id)
	if !idRe.MatchString(id) {
		return Project{}, errors.New("id de proyecto invalido")
	}
	row := s.db.QueryRow(`SELECT id,name,slug,notes,enabled,created_at,updated_at FROM projects WHERE id = ?`, id)
	p, err := scanProjectRows(row)
	if err != nil {
		return Project{}, errors.New("proyecto no encontrado")
	}
	return p, nil
}

func (s *Store) ProjectExists(id string) bool {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM projects WHERE id = ? AND enabled = 1`, strings.TrimSpace(id)).Scan(&count)
	return count > 0
}

func (s *Store) AddProject(p Project) (Project, error) {
	now := time.Now().UTC()
	p.Normalize(now)
	if p.ID == "" {
		id, err := randomID()
		if err != nil {
			return Project{}, err
		}
		p.ID = id
	}
	p.Enabled = true
	if err := p.Validate(); err != nil {
		return Project{}, err
	}
	var existingName int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM projects WHERE lower(name) = lower(?)`, p.Name).Scan(&existingName); err != nil {
		return Project{}, fmt.Errorf("validar nombre de proyecto: %w", err)
	}
	if existingName > 0 {
		return Project{}, errors.New("ya existe un proyecto con ese nombre")
	}
	_, err := s.db.Exec(`INSERT INTO projects(id,name,slug,notes,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, p.ID, p.Name, p.Slug, p.Notes, boolInt(p.Enabled), formatTime(p.CreatedAt), formatTime(p.UpdatedAt))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Project{}, errors.New("ya existe un proyecto con ese nombre o slug")
		}
		return Project{}, fmt.Errorf("crear proyecto: %w", err)
	}
	return p, nil
}

func (s *Store) UpdateProject(id, name, notes string, enabled bool) (Project, error) {
	p, err := s.ProjectByID(id)
	if err != nil {
		return Project{}, err
	}
	p.Name = name
	p.Notes = notes
	p.Enabled = enabled
	p.Normalize(time.Now().UTC())
	if err := p.Validate(); err != nil {
		return Project{}, err
	}
	var existingName int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM projects WHERE lower(name) = lower(?) AND id <> ?`, p.Name, p.ID).Scan(&existingName); err != nil {
		return Project{}, fmt.Errorf("validar nombre de proyecto: %w", err)
	}
	if existingName > 0 {
		return Project{}, errors.New("ya existe otro proyecto con ese nombre")
	}
	_, err = s.db.Exec(`UPDATE projects SET name = ?, notes = ?, enabled = ?, updated_at = ? WHERE id = ?`, p.Name, p.Notes, boolInt(p.Enabled), formatTime(p.UpdatedAt), p.ID)
	if err != nil {
		return Project{}, fmt.Errorf("actualizar proyecto: %w", err)
	}
	return p, nil
}

func (s *Store) ProjectStats() map[string]map[string]int {
	stats := map[string]map[string]int{}
	for _, p := range s.ListProjects() {
		stats[p.ID] = map[string]int{"resources": 0, "agents": 0, "activeResources": 0}
	}
	rows, err := s.db.Query(`SELECT project_id, COUNT(*), SUM(CASE WHEN enabled = 1 THEN 1 ELSE 0 END) FROM resources GROUP BY project_id`)
	if err == nil {
		for rows.Next() {
			var projectID string
			var total, active int
			if rows.Scan(&projectID, &total, &active) == nil {
				if stats[projectID] == nil {
					stats[projectID] = map[string]int{}
				}
				stats[projectID]["resources"] = total
				stats[projectID]["activeResources"] = active
			}
		}
		_ = rows.Close()
	}
	rows, err = s.db.Query(`SELECT project_id, COUNT(*) FROM agents GROUP BY project_id`)
	if err == nil {
		for rows.Next() {
			var projectID string
			var total int
			if rows.Scan(&projectID, &total) == nil {
				if stats[projectID] == nil {
					stats[projectID] = map[string]int{}
				}
				stats[projectID]["agents"] = total
			}
		}
		_ = rows.Close()
	}
	return stats
}

func (s *Store) ProjectDependencyCounts(projectID string) (resources int, agents int, err error) {
	projectID = strings.TrimSpace(projectID)
	if !idRe.MatchString(projectID) {
		return 0, 0, errors.New("id de proyecto invalido")
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM resources WHERE project_id = ?`, projectID).Scan(&resources); err != nil {
		return 0, 0, fmt.Errorf("contar recursos del proyecto: %w", err)
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM agents WHERE project_id = ?`, projectID).Scan(&agents); err != nil {
		return 0, 0, fmt.Errorf("contar clientes de sistema del proyecto: %w", err)
	}
	return resources, agents, nil
}

func (s *Store) DeleteProjectIfEmpty(projectID string) error {
	projectID = strings.TrimSpace(projectID)
	if !idRe.MatchString(projectID) {
		return errors.New("id de proyecto invalido")
	}
	resources, agents, err := s.ProjectDependencyCounts(projectID)
	if err != nil {
		return err
	}
	if resources > 0 || agents > 0 {
		return fmt.Errorf("no se puede eliminar: primero elimina %d recurso(s) y %d cliente(s) de sistema del proyecto", resources, agents)
	}
	res, err := s.db.Exec(`DELETE FROM projects WHERE id = ?`, projectID)
	if err != nil {
		return fmt.Errorf("eliminar proyecto: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("proyecto no encontrado")
	}
	return nil
}

func (s *Store) ListManagedDomains() []ManagedDomain {
	rows, err := s.db.Query(`SELECT id,domain,enabled,created_at,updated_at FROM managed_domains ORDER BY domain ASC`)
	if err != nil {
		return []ManagedDomain{}
	}
	defer rows.Close()
	out := []ManagedDomain{}
	for rows.Next() {
		d, err := scanManagedDomainRows(rows)
		if err == nil {
			out = append(out, d)
		}
	}
	return out
}

func (s *Store) AddManagedDomain(domain string) (ManagedDomain, error) {
	now := time.Now().UTC()
	d := ManagedDomain{Domain: domain, Enabled: true}
	d.Normalize(now)
	if d.ID == "" {
		id, err := randomID()
		if err != nil {
			return ManagedDomain{}, err
		}
		d.ID = id
	}
	if err := d.Validate(); err != nil {
		return ManagedDomain{}, err
	}
	var existing int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM managed_domains WHERE lower(domain) = lower(?)`, d.Domain).Scan(&existing); err != nil {
		return ManagedDomain{}, fmt.Errorf("validar dominio: %w", err)
	}
	if existing > 0 {
		return ManagedDomain{}, errors.New("ya existe ese dominio administrado")
	}
	_, err := s.db.Exec(`INSERT INTO managed_domains(id,domain,enabled,created_at,updated_at) VALUES(?,?,?,?,?)`, d.ID, d.Domain, boolInt(d.Enabled), formatTime(d.CreatedAt), formatTime(d.UpdatedAt))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return ManagedDomain{}, errors.New("ya existe ese dominio administrado")
		}
		return ManagedDomain{}, fmt.Errorf("crear dominio administrado: %w", err)
	}
	return d, nil
}

func (s *Store) DeleteManagedDomain(id string) error {
	id = strings.TrimSpace(id)
	if !idRe.MatchString(id) {
		return errors.New("id de dominio invalido")
	}
	res, err := s.db.Exec(`DELETE FROM managed_domains WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("eliminar dominio: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("dominio no encontrado")
	}
	return nil
}

const resourceSelectSQL = `SELECT id,project_id,name,mode,COALESCE(domain,''),COALESCE(path_prefix,''),COALESCE(public_port,0),COALESCE(tunnel_port,0),COALESCE(backend_scheme,''),backend_host,backend_port,COALESCE(origin_type,'local'),COALESCE(agent_id,''),tls,enabled,COALESCE(disabled_response_mode,'403'),COALESCE(disabled_status_code,403),COALESCE(disabled_html,''),created_at,updated_at FROM resources`

func (s *Store) ListResources() []Resource {
	rows, err := s.db.Query(resourceSelectSQL + ` ORDER BY created_at ASC`)
	if err != nil {
		return []Resource{}
	}
	defer rows.Close()
	out := []Resource{}
	for rows.Next() {
		r, err := scanResourceRows(rows)
		if err == nil {
			out = append(out, r)
		}
	}
	return out
}

func (s *Store) ListResourcesByProject(projectID string) []Resource {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return s.ListResources()
	}
	rows, err := s.db.Query(resourceSelectSQL+` WHERE project_id = ? ORDER BY created_at ASC`, projectID)
	if err != nil {
		return []Resource{}
	}
	defer rows.Close()
	out := []Resource{}
	for rows.Next() {
		r, err := scanResourceRows(rows)
		if err == nil {
			out = append(out, r)
		}
	}
	return out
}

func (s *Store) ResourceByID(id string) (Resource, error) {
	id = strings.TrimSpace(id)
	if !idRe.MatchString(id) {
		return Resource{}, errors.New("id de recurso invalido")
	}
	r, err := scanResourceRows(s.db.QueryRow(resourceSelectSQL+` WHERE id = ?`, id))
	if err != nil {
		return Resource{}, errors.New("recurso no encontrado")
	}
	return r, nil
}

func (s *Store) ResourcePublicPortExists(mode string, port int) (bool, error) {
	return s.ResourcePublicPortExistsExcept(mode, port, "")
}

func (s *Store) ResourcePublicPortExistsExcept(mode string, port int, excludeID string) (bool, error) {
	conflict, err := s.ResourcePublicPortConflictExcept(mode, port, excludeID)
	return conflict.ID != "", err
}

func (s *Store) ResourcePublicPortConflictExcept(mode string, port int, excludeID string) (Resource, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	excludeID = strings.TrimSpace(excludeID)
	if mode != ModeTCP && mode != ModeUDP {
		return Resource{}, nil
	}
	if port <= 0 {
		return Resource{}, nil
	}
	query := resourceSelectSQL + ` WHERE mode = ? AND COALESCE(public_port,0) = ?`
	args := []any{mode, port}
	if excludeID != "" {
		query += ` AND id <> ?`
		args = append(args, excludeID)
	}
	query += ` ORDER BY updated_at DESC LIMIT 1`
	row := s.db.QueryRow(query, args...)
	res, err := scanResourceRows(row)
	if err == nil {
		return res, nil
	}
	if errors.Is(err, sql.ErrNoRows) || strings.Contains(err.Error(), "no rows") {
		return Resource{}, nil
	}
	return Resource{}, fmt.Errorf("validar puerto publico: %w", err)
}

func (s *Store) AddResource(r Resource) (Resource, error) {
	now := time.Now().UTC()
	r.Normalize(now)
	if r.ID == "" {
		id, err := randomID()
		if err != nil {
			return Resource{}, err
		}
		r.ID = id
	}
	if err := r.Validate(); err != nil {
		return Resource{}, err
	}
	if !s.ProjectExists(r.ProjectID) {
		return Resource{}, errors.New("projectId no existe")
	}
	if r.OriginType == OriginAgent && !s.AgentBelongsToProject(r.AgentID, r.ProjectID) {
		return Resource{}, errors.New("agentId no existe en este proyecto")
	}
	if err := s.prepareTunnelPort(&r, ""); err != nil {
		return Resource{}, err
	}
	if r.Mode == ModeHTTP {
		var existing int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM resources WHERE mode = 'http' AND domain = ? AND path_prefix = ?`, r.Domain, r.PathPrefix).Scan(&existing); err != nil {
			return Resource{}, fmt.Errorf("validar ruta HTTP: %w", err)
		}
		if existing > 0 {
			return Resource{}, errors.New("ya existe un recurso con el mismo dominio/path")
		}
	}
	if r.Mode == ModeTCP || r.Mode == ModeUDP {
		conflict, err := s.ResourcePublicPortConflictExcept(r.Mode, r.PublicPort, r.ID)
		if err != nil {
			return Resource{}, err
		}
		if conflict.ID != "" {
			return Resource{}, fmt.Errorf("ya existe un recurso %s usando el puerto publico %d: %s (%s)", strings.ToUpper(r.Mode), r.PublicPort, conflict.Name, conflict.ID)
		}
	}
	_, err := s.db.Exec(`INSERT INTO resources(id,project_id,name,mode,domain,path_prefix,public_port,tunnel_port,backend_scheme,backend_host,backend_port,origin_type,agent_id,tls,enabled,disabled_response_mode,disabled_status_code,disabled_html,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, r.ID, r.ProjectID, r.Name, r.Mode, nullableString(r.Domain), nullableString(r.PathPrefix), nullableInt(r.PublicPort), nullableInt(r.TunnelPort), nullableString(r.BackendScheme), r.BackendHost, r.BackendPort, r.OriginType, nullableString(r.AgentID), boolInt(r.TLS), boolInt(r.Enabled), r.DisabledResponseMode, r.DisabledStatusCode, r.DisabledHTML, formatTime(r.CreatedAt), formatTime(r.UpdatedAt))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Resource{}, errors.New("ya existe un recurso con el mismo dominio/path o puerto publico")
		}
		return Resource{}, fmt.Errorf("crear recurso: %w", err)
	}
	return r, nil
}

func (s *Store) UpdateResource(id string, next Resource) (Resource, error) {
	id = strings.TrimSpace(id)
	current, err := s.ResourceByID(id)
	if err != nil {
		return Resource{}, err
	}
	now := time.Now().UTC()
	next.ID = current.ID
	next.CreatedAt = current.CreatedAt
	if strings.TrimSpace(next.ProjectID) == "" {
		next.ProjectID = current.ProjectID
	}
	if next.DisabledResponseMode == "" {
		next.DisabledResponseMode = current.DisabledResponseMode
	}
	if next.DisabledStatusCode == 0 {
		next.DisabledStatusCode = current.DisabledStatusCode
	}
	if next.DisabledHTML == "" && current.DisabledHTML != "" && next.DisabledResponseMode == current.DisabledResponseMode {
		next.DisabledHTML = current.DisabledHTML
	}
	next.Normalize(now)
	if err := next.Validate(); err != nil {
		return Resource{}, err
	}
	if !s.ProjectExists(next.ProjectID) {
		return Resource{}, errors.New("projectId no existe")
	}
	if next.OriginType == OriginAgent && !s.AgentBelongsToProject(next.AgentID, next.ProjectID) {
		return Resource{}, errors.New("agentId no existe en este proyecto")
	}
	if next.OriginType == current.OriginType && next.Mode == current.Mode && next.AgentID == current.AgentID && current.TunnelPort > 0 {
		next.TunnelPort = current.TunnelPort
	}
	if err := s.prepareTunnelPort(&next, next.ID); err != nil {
		return Resource{}, err
	}
	if next.Mode == ModeHTTP {
		var existing int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM resources WHERE mode = 'http' AND domain = ? AND path_prefix = ? AND id <> ?`, next.Domain, next.PathPrefix, next.ID).Scan(&existing); err != nil {
			return Resource{}, fmt.Errorf("validar ruta HTTP: %w", err)
		}
		if existing > 0 {
			return Resource{}, errors.New("ya existe un recurso con el mismo dominio/path")
		}
	}
	if next.Mode == ModeTCP || next.Mode == ModeUDP {
		conflict, err := s.ResourcePublicPortConflictExcept(next.Mode, next.PublicPort, next.ID)
		if err != nil {
			return Resource{}, err
		}
		if conflict.ID != "" {
			return Resource{}, fmt.Errorf("ya existe un recurso %s usando el puerto publico %d: %s (%s)", strings.ToUpper(next.Mode), next.PublicPort, conflict.Name, conflict.ID)
		}
	}
	_, err = s.db.Exec(`UPDATE resources SET project_id = ?, name = ?, mode = ?, domain = ?, path_prefix = ?, public_port = ?, tunnel_port = ?, backend_scheme = ?, backend_host = ?, backend_port = ?, origin_type = ?, agent_id = ?, tls = ?, enabled = ?, disabled_response_mode = ?, disabled_status_code = ?, disabled_html = ?, updated_at = ? WHERE id = ?`, next.ProjectID, next.Name, next.Mode, nullableString(next.Domain), nullableString(next.PathPrefix), nullableInt(next.PublicPort), nullableInt(next.TunnelPort), nullableString(next.BackendScheme), next.BackendHost, next.BackendPort, next.OriginType, nullableString(next.AgentID), boolInt(next.TLS), boolInt(next.Enabled), next.DisabledResponseMode, next.DisabledStatusCode, next.DisabledHTML, formatTime(next.UpdatedAt), next.ID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return Resource{}, errors.New("ya existe un recurso con el mismo dominio/path o puerto publico")
		}
		return Resource{}, fmt.Errorf("actualizar recurso: %w", err)
	}
	return next, nil
}

func (s *Store) prepareTunnelPort(r *Resource, excludeID string) error {
	if r == nil {
		return nil
	}
	if !r.UsesAgent() || (r.Mode != ModeTCP && r.Mode != ModeUDP) {
		r.TunnelPort = 0
		return nil
	}
	if r.TunnelPort > 0 {
		return nil
	}
	used := map[int]bool{}
	rows, err := s.db.Query(`SELECT COALESCE(tunnel_port,0) FROM resources WHERE COALESCE(tunnel_port,0) > 0 AND id <> ?`, strings.TrimSpace(excludeID))
	if err == nil {
		for rows.Next() {
			var p int
			if rows.Scan(&p) == nil && p > 0 {
				used[p] = true
			}
		}
		_ = rows.Close()
	}
	for port := 42000; port <= 49999; port++ {
		if used[port] {
			continue
		}
		if r.Mode == ModeTCP {
			if err := TCPPortAvailable(port); err != nil {
				continue
			}
		} else {
			if err := UDPPortAvailable(port); err != nil {
				continue
			}
		}
		r.TunnelPort = port
		return nil
	}
	return errors.New("no hay puertos internos disponibles para el puente del cliente NAT")
}

func (s *Store) DeleteResource(id string) error {
	res, err := s.db.Exec(`DELETE FROM resources WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("recurso no encontrado")
	}
	return nil
}

func (s *Store) UpdateResourceControl(id string, enabled bool, responseMode string, statusCode int, html string) (Resource, error) {
	id = strings.TrimSpace(id)
	if !idRe.MatchString(id) {
		return Resource{}, errors.New("id de recurso invalido")
	}
	row := s.db.QueryRow(resourceSelectSQL+` WHERE id = ?`, id)
	r, err := scanResourceRows(row)
	if err != nil {
		return Resource{}, errors.New("recurso no encontrado")
	}
	r.Enabled = enabled
	r.DisabledResponseMode = responseMode
	r.DisabledStatusCode = statusCode
	r.DisabledHTML = html
	r.Normalize(time.Now().UTC())
	if err := r.Validate(); err != nil {
		return Resource{}, err
	}
	_, err = s.db.Exec(`UPDATE resources SET enabled = ?, disabled_response_mode = ?, disabled_status_code = ?, disabled_html = ?, updated_at = ? WHERE id = ?`, boolInt(r.Enabled), r.DisabledResponseMode, r.DisabledStatusCode, r.DisabledHTML, formatTime(r.UpdatedAt), r.ID)
	if err != nil {
		return Resource{}, fmt.Errorf("actualizar control del recurso: %w", err)
	}
	return r, nil
}

func (s *Store) ListAgents() []AgentPublic {
	return s.ListAgentsByProject("")
}

func (s *Store) ListAgentsByProject(projectID string) []AgentPublic {
	query := `SELECT a.project_id,a.id,a.name,a.enabled,a.created_at,a.updated_at,COALESCE(a.last_seen,''),COALESCE(a.os,''),COALESCE(a.arch,''),COALESCE(a.hostname,''),COALESCE(a.public_ip,''),COALESCE(a.private_ip,''),COALESCE(a.version,''),COALESCE(a.last_error,''),COALESCE((SELECT COUNT(*) FROM resources r WHERE r.agent_id = a.id),0) FROM agents a`
	args := []any{}
	if strings.TrimSpace(projectID) != "" {
		query += ` WHERE a.project_id = ?`
		args = append(args, strings.TrimSpace(projectID))
	}
	query += ` ORDER BY a.created_at ASC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return []AgentPublic{}
	}
	defer rows.Close()
	out := []AgentPublic{}
	now := time.Now().UTC()
	for rows.Next() {
		var a AgentPublic
		var enabled int
		var createdAt, updatedAt, lastSeen string
		if err := rows.Scan(&a.ProjectID, &a.ID, &a.Name, &enabled, &createdAt, &updatedAt, &lastSeen, &a.OS, &a.Arch, &a.Hostname, &a.PublicIP, &a.PrivateIP, &a.Version, &a.LastError, &a.ResourceCount); err != nil {
			continue
		}
		a.Enabled = enabled == 1
		a.CreatedAt = parseTime(createdAt)
		a.UpdatedAt = parseTime(updatedAt)
		a.LastSeen = parseTime(lastSeen)
		a.Online = a.Enabled && !a.LastSeen.IsZero() && now.Sub(a.LastSeen) <= 45*time.Second
		out = append(out, a)
	}
	return out
}

func (s *Store) AgentByID(id string) (AgentPublic, error) {
	id = strings.TrimSpace(id)
	if !idRe.MatchString(id) {
		return AgentPublic{}, errors.New("id de agente invalido")
	}
	query := `SELECT a.project_id,a.id,a.name,a.enabled,a.created_at,a.updated_at,COALESCE(a.last_seen,''),COALESCE(a.os,''),COALESCE(a.arch,''),COALESCE(a.hostname,''),COALESCE(a.public_ip,''),COALESCE(a.private_ip,''),COALESCE(a.version,''),COALESCE(a.last_error,''),COALESCE((SELECT COUNT(*) FROM resources r WHERE r.agent_id = a.id),0) FROM agents a WHERE a.id = ?`
	row := s.db.QueryRow(query, id)
	var a AgentPublic
	var enabled int
	var createdAt, updatedAt, lastSeen string
	if err := row.Scan(&a.ProjectID, &a.ID, &a.Name, &enabled, &createdAt, &updatedAt, &lastSeen, &a.OS, &a.Arch, &a.Hostname, &a.PublicIP, &a.PrivateIP, &a.Version, &a.LastError, &a.ResourceCount); err != nil {
		return AgentPublic{}, errors.New("agente no encontrado")
	}
	a.Enabled = enabled == 1
	a.CreatedAt = parseTime(createdAt)
	a.UpdatedAt = parseTime(updatedAt)
	a.LastSeen = parseTime(lastSeen)
	a.Online = a.Enabled && !a.LastSeen.IsZero() && time.Since(a.LastSeen) <= 45*time.Second
	return a, nil
}

func (s *Store) ListResourcesByAgent(agentID string) []Resource {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return []Resource{}
	}
	rows, err := s.db.Query(resourceSelectSQL+` WHERE agent_id = ? ORDER BY created_at ASC`, agentID)
	if err != nil {
		return []Resource{}
	}
	defer rows.Close()
	out := []Resource{}
	for rows.Next() {
		r, err := scanResourceRows(rows)
		if err == nil {
			out = append(out, r)
		}
	}
	return out
}

func (s *Store) AddAgent(a Agent) (Agent, error) {
	now := time.Now().UTC()
	a.Normalize(now)
	if a.ID == "" {
		id, err := randomID()
		if err != nil {
			return Agent{}, err
		}
		a.ID = id
	}
	if a.Token == "" {
		token, err := newSecret(32)
		if err != nil {
			return Agent{}, err
		}
		a.Token = token
	}
	a.TokenHash = hashToken(a.Token)
	a.Enabled = true
	if !s.ProjectExists(a.ProjectID) {
		return Agent{}, errors.New("projectId no existe")
	}
	if err := a.Validate(); err != nil {
		return Agent{}, err
	}
	_, err := s.db.Exec(`INSERT INTO agents(id,project_id,name,token_hash,enabled,created_at,updated_at,last_seen) VALUES(?,?,?,?,?,?,?,NULL)`, a.ID, a.ProjectID, a.Name, a.TokenHash, boolInt(a.Enabled), formatTime(a.CreatedAt), formatTime(a.UpdatedAt))
	if err != nil {
		return Agent{}, fmt.Errorf("crear agente: %w", err)
	}
	return a, nil
}

func (s *Store) AgentExists(id string) bool {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM agents WHERE id = ? AND enabled = 1`, id).Scan(&count)
	return count > 0
}

func (s *Store) AgentBelongsToProject(id, projectID string) bool {
	var count int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM agents WHERE id = ? AND project_id = ? AND enabled = 1`, strings.TrimSpace(id), strings.TrimSpace(projectID)).Scan(&count)
	return count > 0
}

func (s *Store) DisableAgent(id string) error {
	id = strings.TrimSpace(id)
	if !idRe.MatchString(id) {
		return errors.New("id de agente invalido")
	}
	now := time.Now().UTC()
	res, err := s.db.Exec(`UPDATE agents SET enabled = 0, updated_at = ? WHERE id = ?`, formatTime(now), id)
	if err != nil {
		return fmt.Errorf("deshabilitar agente: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return errors.New("agente no encontrado")
	}
	return nil
}

func (s *Store) DeleteAgentAndResources(id string) (AgentPublic, []Resource, error) {
	id = strings.TrimSpace(id)
	if !idRe.MatchString(id) {
		return AgentPublic{}, nil, errors.New("id de agente invalido")
	}
	agent, err := s.AgentByID(id)
	if err != nil {
		return AgentPublic{}, nil, err
	}
	resources := s.ListResourcesByAgent(id)
	tx, err := s.db.Begin()
	if err != nil {
		return AgentPublic{}, nil, fmt.Errorf("iniciar eliminacion de cliente: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM resources WHERE agent_id = ?`, id); err != nil {
		_ = tx.Rollback()
		return AgentPublic{}, nil, fmt.Errorf("eliminar recursos vinculados al cliente: %w", err)
	}
	res, err := tx.Exec(`DELETE FROM agents WHERE id = ?`, id)
	if err != nil {
		_ = tx.Rollback()
		return AgentPublic{}, nil, fmt.Errorf("eliminar cliente: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		_ = tx.Rollback()
		return AgentPublic{}, nil, errors.New("agente no encontrado")
	}
	if err := tx.Commit(); err != nil {
		return AgentPublic{}, nil, fmt.Errorf("confirmar eliminacion de cliente: %w", err)
	}
	return agent, resources, nil
}

func (s *Store) RotateAgentToken(id string) (Agent, error) {
	id = strings.TrimSpace(id)
	if !idRe.MatchString(id) {
		return Agent{}, errors.New("id de agente invalido")
	}
	token, err := newSecret(32)
	if err != nil {
		return Agent{}, err
	}
	now := time.Now().UTC()
	res, err := s.db.Exec(`UPDATE agents SET token_hash = ?, enabled = 1, updated_at = ? WHERE id = ?`, hashToken(token), formatTime(now), id)
	if err != nil {
		return Agent{}, fmt.Errorf("rotar token de agente: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Agent{}, errors.New("agente no encontrado")
	}
	row := s.db.QueryRow(`SELECT project_id,id,name,token_hash,enabled,created_at,updated_at,COALESCE(last_seen,''),COALESCE(os,''),COALESCE(arch,''),COALESCE(hostname,''),COALESCE(public_ip,''),COALESCE(private_ip,''),COALESCE(version,''),COALESCE(last_error,'') FROM agents WHERE id = ?`, id)
	var a Agent
	var enabled int
	var createdAt, updatedAt, lastSeen string
	if err := row.Scan(&a.ProjectID, &a.ID, &a.Name, &a.TokenHash, &enabled, &createdAt, &updatedAt, &lastSeen, &a.OS, &a.Arch, &a.Hostname, &a.PublicIP, &a.PrivateIP, &a.Version, &a.LastError); err != nil {
		return Agent{}, errors.New("agente no encontrado")
	}
	a.Token = token
	a.Enabled = enabled == 1
	a.CreatedAt = parseTime(createdAt)
	a.UpdatedAt = parseTime(updatedAt)
	a.LastSeen = parseTime(lastSeen)
	return a, nil
}

func (s *Store) AuthenticateAgent(id, token string) (Agent, bool) {
	id = strings.TrimSpace(id)
	token = strings.TrimSpace(token)
	if id == "" || token == "" {
		return Agent{}, false
	}
	row := s.db.QueryRow(`SELECT project_id,id,name,token_hash,enabled,created_at,updated_at,COALESCE(last_seen,''),COALESCE(os,''),COALESCE(arch,''),COALESCE(hostname,''),COALESCE(public_ip,''),COALESCE(private_ip,''),COALESCE(version,''),COALESCE(last_error,'') FROM agents WHERE id = ?`, id)
	var a Agent
	var enabled int
	var createdAt, updatedAt, lastSeen string
	if err := row.Scan(&a.ProjectID, &a.ID, &a.Name, &a.TokenHash, &enabled, &createdAt, &updatedAt, &lastSeen, &a.OS, &a.Arch, &a.Hostname, &a.PublicIP, &a.PrivateIP, &a.Version, &a.LastError); err != nil {
		return Agent{}, false
	}
	if enabled != 1 || subtle.ConstantTimeCompare([]byte(a.TokenHash), []byte(hashToken(token))) != 1 {
		return Agent{}, false
	}
	now := time.Now().UTC()
	a.Enabled = true
	a.CreatedAt = parseTime(createdAt)
	a.UpdatedAt = parseTime(updatedAt)
	a.LastSeen = parseTime(lastSeen)
	if now.Sub(a.LastSeen) > 30*time.Second {
		_, _ = s.db.Exec(`UPDATE agents SET last_seen = ?, updated_at = ? WHERE id = ?`, formatTime(now), formatTime(now), a.ID)
		a.LastSeen = now
		a.UpdatedAt = now
	}
	return a, true
}

func (s *Store) TouchAgent(id string, hb AgentHeartbeat) {
	id = strings.TrimSpace(id)
	if !idRe.MatchString(id) {
		return
	}
	now := time.Now().UTC()
	hb.OS = cleanMeta(hb.OS, 32)
	hb.Arch = cleanMeta(hb.Arch, 32)
	hb.Hostname = cleanMeta(hb.Hostname, 128)
	hb.PublicIP = cleanIP(hb.PublicIP)
	hb.PrivateIP = cleanIP(hb.PrivateIP)
	hb.Version = cleanMeta(hb.Version, 64)
	hb.LastError = cleanMeta(hb.LastError, 240)
	_, _ = s.db.Exec(`UPDATE agents SET last_seen = ?, updated_at = ?, os = CASE WHEN ? <> '' THEN ? ELSE os END, arch = CASE WHEN ? <> '' THEN ? ELSE arch END, hostname = CASE WHEN ? <> '' THEN ? ELSE hostname END, public_ip = CASE WHEN ? <> '' THEN ? ELSE public_ip END, private_ip = CASE WHEN ? <> '' THEN ? ELSE private_ip END, version = CASE WHEN ? <> '' THEN ? ELSE version END, last_error = CASE WHEN ? <> '' THEN ? ELSE last_error END WHERE id = ?`, formatTime(now), formatTime(now), hb.OS, hb.OS, hb.Arch, hb.Arch, hb.Hostname, hb.Hostname, hb.PublicIP, hb.PublicIP, hb.PrivateIP, hb.PrivateIP, hb.Version, hb.Version, hb.LastError, hb.LastError, id)
}

func cleanMeta(v string, max int) string {
	v = strings.TrimSpace(v)
	v = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, v)
	if len(v) > max {
		v = v[:max]
	}
	return v
}

func cleanIP(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.IndexByte(v, ','); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	if host, _, err := net.SplitHostPort(v); err == nil {
		v = host
	}
	if ip := net.ParseIP(v); ip != nil {
		return ip.String()
	}
	return ""
}

func (s *Store) FindHTTPPanelResource(host, path string) (Resource, bool) {
	host = strings.ToLower(strings.TrimSpace(host))
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	if path == "" {
		path = "/"
	}
	rows, err := s.db.Query(resourceSelectSQL+` WHERE mode = 'http' AND domain = ? AND (enabled = 0 OR COALESCE(origin_type,'local') = 'agent') ORDER BY length(COALESCE(path_prefix,'')) DESC`, host)
	if err != nil {
		return Resource{}, false
	}
	defer rows.Close()
	for rows.Next() {
		r, err := scanResourceRows(rows)
		if err != nil {
			continue
		}
		prefix := r.PathPrefix
		if prefix == "" {
			prefix = "/"
		}
		if prefix == "/" || strings.HasPrefix(path, prefix) {
			return r, true
		}
	}
	return Resource{}, false
}

func scanUser(row *sql.Row) (User, error) {
	var u User
	var force int
	var createdAt, updatedAt string
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &force, &createdAt, &updatedAt); err != nil {
		return User{}, err
	}
	u.ForcePasswordChange = force == 1
	u.CreatedAt = parseTime(createdAt)
	u.UpdatedAt = parseTime(updatedAt)
	return u, nil
}

type projectScanner interface {
	Scan(dest ...any) error
}

func scanProjectRows(row projectScanner) (Project, error) {
	var p Project
	var enabled int
	var createdAt, updatedAt string
	if err := row.Scan(&p.ID, &p.Name, &p.Slug, &p.Notes, &enabled, &createdAt, &updatedAt); err != nil {
		return Project{}, err
	}
	p.Enabled = enabled == 1
	p.CreatedAt = parseTime(createdAt)
	p.UpdatedAt = parseTime(updatedAt)
	return p, nil
}

type managedDomainScanner interface {
	Scan(dest ...any) error
}

func scanManagedDomainRows(row managedDomainScanner) (ManagedDomain, error) {
	var d ManagedDomain
	var enabled int
	var createdAt, updatedAt string
	if err := row.Scan(&d.ID, &d.Domain, &enabled, &createdAt, &updatedAt); err != nil {
		return ManagedDomain{}, err
	}
	d.Enabled = enabled == 1
	d.CreatedAt = parseTime(createdAt)
	d.UpdatedAt = parseTime(updatedAt)
	return d, nil
}

type resourceScanner interface {
	Scan(dest ...any) error
}

func scanResourceRows(row resourceScanner) (Resource, error) {
	var r Resource
	var tls, enabled int
	var createdAt, updatedAt string
	if err := row.Scan(&r.ID, &r.ProjectID, &r.Name, &r.Mode, &r.Domain, &r.PathPrefix, &r.PublicPort, &r.TunnelPort, &r.BackendScheme, &r.BackendHost, &r.BackendPort, &r.OriginType, &r.AgentID, &tls, &enabled, &r.DisabledResponseMode, &r.DisabledStatusCode, &r.DisabledHTML, &createdAt, &updatedAt); err != nil {
		return Resource{}, err
	}
	r.TLS = tls == 1
	r.Enabled = enabled == 1
	if r.OriginType == "" {
		if r.AgentID != "" {
			r.OriginType = OriginAgent
		} else {
			r.OriginType = OriginLocal
		}
	}
	if r.DisabledResponseMode == "" {
		r.DisabledResponseMode = DisabledResponse403
	}
	if r.DisabledStatusCode == 0 {
		r.DisabledStatusCode = 403
	}
	r.CreatedAt = parseTime(createdAt)
	r.UpdatedAt = parseTime(updatedAt)
	return r, nil
}

func randomID() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func nullableInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}
