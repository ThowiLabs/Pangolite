package app

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                  string
	DataPath              string
	TraefikDir            string
	DashboardDomain       string
	LetsEncryptEmail      string
	PublicIP              string
	BootstrapTraefik      bool
	InsecureDev           bool
	InitialAdminUser      string
	InitialPasswordFile   string
	SessionDays           int
	CookieSecureOverride  string
	AutoTraefik           bool
	LogPath               string
	BackupDir             string
	SuspensionTemplateDir string
	BackupIntervalHours   int
	BackupRetentionDays   int
}

func LoadConfigFromEnv() Config {
	return Config{
		Addr:                  env("PANGOLITE_ADDR", "0.0.0.0:2424"),
		DataPath:              env("PANGOLITE_DATA", "/opt/pangolite/data/pangolite.db"),
		TraefikDir:            env("PANGOLITE_TRAEFIK_DIR", "/etc/traefik"),
		DashboardDomain:       env("PANGOLITE_DASHBOARD_DOMAIN", ""),
		LetsEncryptEmail:      env("PANGOLITE_LETSENCRYPT_EMAIL", ""),
		PublicIP:              env("PANGOLITE_PUBLIC_IP", ""),
		BootstrapTraefik:      os.Getenv("PANGOLITE_BOOTSTRAP_TRAEFIK") == "1",
		InsecureDev:           os.Getenv("PANGOLITE_INSECURE_DEV") == "1",
		InitialAdminUser:      env("PANGOLITE_INITIAL_ADMIN_USER", "admin"),
		InitialPasswordFile:   env("PANGOLITE_INITIAL_PASSWORD_FILE", ""),
		SessionDays:           envInt("PANGOLITE_SESSION_DAYS", 30),
		CookieSecureOverride:  strings.TrimSpace(os.Getenv("PANGOLITE_COOKIE_SECURE")),
		AutoTraefik:           env("PANGOLITE_AUTO_TRAEFIK", "1") != "0",
		LogPath:               env("PANGOLITE_LOG_FILE", ""),
		BackupDir:             env("PANGOLITE_BACKUP_DIR", ""),
		SuspensionTemplateDir: env("PANGOLITE_SUSPENSION_TEMPLATE_DIR", ""),
		BackupIntervalHours:   envInt("PANGOLITE_BACKUP_INTERVAL_HOURS", 24),
		BackupRetentionDays:   envInt("PANGOLITE_BACKUP_RETENTION_DAYS", 14),
	}
}

func (c *Config) ResolveBootstrapPaths() {
	base := filepath.Dir(c.DataPath)
	if base == "." || base == "" {
		base = "."
	}
	if strings.TrimSpace(c.InitialPasswordFile) == "" {
		c.InitialPasswordFile = filepath.Join(base, "admin-password.txt")
	}
	if strings.TrimSpace(c.LogPath) == "" {
		c.LogPath = filepath.Join(base, "pangolite.log")
	}
	if strings.TrimSpace(c.BackupDir) == "" {
		c.BackupDir = filepath.Join(base, "backups")
	}
	if strings.TrimSpace(c.SuspensionTemplateDir) == "" {
		c.SuspensionTemplateDir = filepath.Join(base, "templates", "suspension")
	}
}

func (c Config) ValidateForServe() error {
	if c.Addr == "" {
		return errors.New("PANGOLITE_ADDR requerido")
	}
	if c.DataPath == "" {
		return errors.New("PANGOLITE_DATA requerido")
	}
	if c.InitialAdminUser == "" {
		return errors.New("PANGOLITE_INITIAL_ADMIN_USER requerido")
	}
	if c.SessionDays < 1 || c.SessionDays > 365 {
		return errors.New("PANGOLITE_SESSION_DAYS debe estar entre 1 y 365")
	}
	return nil
}

func (c Config) ValidateForRender() error {
	if c.TraefikDir == "" {
		return errors.New("PANGOLITE_TRAEFIK_DIR requerido")
	}
	if c.DashboardDomain != "" && c.LetsEncryptEmail == "" {
		return errors.New("PANGOLITE_LETSENCRYPT_EMAIL requerido cuando configuras dominio del panel")
	}
	return nil
}

func ApplyCommonFlags(fs *flag.FlagSet, c *Config) {
	fs.StringVar(&c.Addr, "addr", c.Addr, "direccion interna del panel")
	fs.StringVar(&c.DataPath, "data", c.DataPath, "ruta de la base SQLite")
	fs.StringVar(&c.TraefikDir, "traefik-dir", c.TraefikDir, "directorio de configuracion de Traefik")
	fs.StringVar(&c.DashboardDomain, "dashboard-domain", c.DashboardDomain, "dominio del panel")
	fs.StringVar(&c.LetsEncryptEmail, "email", c.LetsEncryptEmail, "correo para Let's Encrypt")
	fs.StringVar(&c.PublicIP, "public-ip", c.PublicIP, "IP publica del servidor para validar DNS")
	fs.StringVar(&c.InitialAdminUser, "initial-admin-user", c.InitialAdminUser, "usuario admin inicial")
	fs.StringVar(&c.InitialPasswordFile, "initial-password-file", c.InitialPasswordFile, "archivo donde se guarda la password temporal inicial")
	fs.StringVar(&c.LogPath, "log-file", c.LogPath, "archivo de logs rotativo")
	fs.StringVar(&c.BackupDir, "backup-dir", c.BackupDir, "directorio de respaldos SQLite")
	fs.StringVar(&c.SuspensionTemplateDir, "suspension-template-dir", c.SuspensionTemplateDir, "directorio de plantillas HTML de suspension")
	fs.IntVar(&c.BackupIntervalHours, "backup-interval-hours", c.BackupIntervalHours, "intervalo de respaldos automaticos en horas; 0 desactiva")
	fs.IntVar(&c.BackupRetentionDays, "backup-retention-days", c.BackupRetentionDays, "dias de retencion para respaldos automaticos; 0 conserva todo")
}

func newSecret(bytesLen int) (string, error) {
	b := make([]byte, bytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generar secreto: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func EnsureDirForFile(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func PrintServeConfig(c Config) string {
	mode := "seguro"
	if c.InsecureDev {
		mode = "desarrollo-inseguro"
	}
	return fmt.Sprintf("addr=%s db=%s log=%s backups=%s templates=%s backup_interval_hours=%d backup_retention_days=%d mode=%s session_days=%d public_ip=%s", c.Addr, c.DataPath, c.LogPath, c.BackupDir, c.SuspensionTemplateDir, c.BackupIntervalHours, c.BackupRetentionDays, mode, c.SessionDays, c.PublicIP)
}

func sessionDuration(c Config) time.Duration {
	return time.Duration(c.SessionDays) * 24 * time.Hour
}
