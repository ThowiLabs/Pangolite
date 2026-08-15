package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

const terminalUsageTargetLocal = "local"

type TerminalUsage struct {
	Target          string    `json:"target"`
	ConnectionCount int64     `json:"connectionCount"`
	LastConnectedAt time.Time `json:"lastConnectedAt"`
	LastDir         string    `json:"lastDir,omitempty"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func normalizeTerminalUsageTarget(target string) (string, bool) {
	target = strings.TrimSpace(target)
	if target == terminalUsageTargetLocal {
		return target, true
	}
	if strings.HasPrefix(target, "agent:") {
		agentID := strings.TrimSpace(strings.TrimPrefix(target, "agent:"))
		if agentID != "" && len(agentID) <= 256 {
			return "agent:" + agentID, true
		}
	}
	return "", false
}

func (s *Store) migrateTerminalUsage(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS terminal_usage (
        user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
        target TEXT NOT NULL,
        connection_count INTEGER NOT NULL DEFAULT 0,
        last_connected_at TEXT NOT NULL DEFAULT '',
        last_dir TEXT NOT NULL DEFAULT '',
        updated_at TEXT NOT NULL,
        PRIMARY KEY(user_id, target)
    )`)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_terminal_usage_user_rank ON terminal_usage(user_id, connection_count DESC, last_connected_at DESC)`)
	return err
}

func (s *Store) RecordTerminalConnection(userID int64, target string) error {
	target, ok := normalizeTerminalUsageTarget(target)
	if !ok || userID <= 0 {
		return errors.New("destino o usuario de terminal invalido")
	}
	now := formatTime(time.Now().UTC())
	_, err := s.db.Exec(`INSERT INTO terminal_usage(user_id,target,connection_count,last_connected_at,last_dir,updated_at)
        VALUES(?,?,1,?,'',?)
        ON CONFLICT(user_id,target) DO UPDATE SET
            connection_count = terminal_usage.connection_count + 1,
            last_connected_at = excluded.last_connected_at,
            updated_at = excluded.updated_at`, userID, target, now, now)
	return err
}

func (s *Store) UpdateTerminalLastDir(userID int64, target, dir string) error {
	target, ok := normalizeTerminalUsageTarget(target)
	dir = strings.TrimSpace(dir)
	if !ok || userID <= 0 || dir == "" || len(dir) > 4096 {
		return errors.New("destino, usuario o directorio de terminal invalido")
	}
	now := formatTime(time.Now().UTC())
	_, err := s.db.Exec(`INSERT INTO terminal_usage(user_id,target,connection_count,last_connected_at,last_dir,updated_at)
        VALUES(?,?,0,'',?,?)
        ON CONFLICT(user_id,target) DO UPDATE SET
            last_dir = excluded.last_dir,
            updated_at = excluded.updated_at`, userID, target, dir, now)
	return err
}

func (s *Store) TerminalUsageByTarget(userID int64, target string) (TerminalUsage, error) {
	target, ok := normalizeTerminalUsageTarget(target)
	if !ok || userID <= 0 {
		return TerminalUsage{}, errors.New("destino o usuario de terminal invalido")
	}
	var usage TerminalUsage
	var lastConnected, updated string
	err := s.db.QueryRow(`SELECT target,connection_count,last_connected_at,last_dir,updated_at FROM terminal_usage WHERE user_id = ? AND target = ?`, userID, target).
		Scan(&usage.Target, &usage.ConnectionCount, &lastConnected, &usage.LastDir, &updated)
	if err != nil {
		return TerminalUsage{}, err
	}
	usage.LastConnectedAt = parseTime(lastConnected)
	usage.UpdatedAt = parseTime(updated)
	return usage, nil
}

func (s *Store) ListTerminalUsage(userID int64) []TerminalUsage {
	if userID <= 0 {
		return nil
	}
	rows, err := s.db.Query(`SELECT target,connection_count,last_connected_at,last_dir,updated_at
        FROM terminal_usage WHERE user_id = ?
        ORDER BY connection_count DESC, last_connected_at DESC, target ASC`, userID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := make([]TerminalUsage, 0)
	for rows.Next() {
		var usage TerminalUsage
		var lastConnected, updated string
		if err := rows.Scan(&usage.Target, &usage.ConnectionCount, &lastConnected, &usage.LastDir, &updated); err != nil {
			return out
		}
		usage.LastConnectedAt = parseTime(lastConnected)
		usage.UpdatedAt = parseTime(updated)
		out = append(out, usage)
	}
	return out
}

func terminalUsageMap(usages []TerminalUsage) map[string]TerminalUsage {
	out := make(map[string]TerminalUsage, len(usages))
	for _, usage := range usages {
		if target, ok := normalizeTerminalUsageTarget(usage.Target); ok {
			usage.Target = target
			out[target] = usage
		}
	}
	return out
}

func (s *Server) terminalState(w http.ResponseWriter, r *http.Request, rs requestSession) {
	usage := terminalUsageMap(s.store.ListTerminalUsage(rs.User.ID))
	writeJSON(w, http.StatusOK, map[string]any{
		"usage":       usage,
		"version":     NormalizedVersion(),
		"versionCode": NormalizedVersionCode(),
	})
}
