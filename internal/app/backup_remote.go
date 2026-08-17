package app

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const backupRemoteSettingKey = "backup_remote_config"

type BackupRemoteConfig struct {
	Enabled       bool   `json:"enabled"`
	AutoUpload    bool   `json:"autoUpload"`
	Provider      string `json:"provider"`
	Prefix        string `json:"prefix"`
	AllowInsecure bool   `json:"allowInsecure"`

	WebDAVURL       string `json:"webdavUrl"`
	WebDAVUsername  string `json:"webdavUsername"`
	WebDAVPassword  string `json:"webdavPassword,omitempty"`
	WebDAVSecretSet bool   `json:"webdavSecretSet"`

	S3Endpoint  string `json:"s3Endpoint"`
	S3Region    string `json:"s3Region"`
	S3Bucket    string `json:"s3Bucket"`
	S3AccessKey string `json:"s3AccessKey"`
	S3SecretKey string `json:"s3SecretKey,omitempty"`
	S3SecretSet bool   `json:"s3SecretSet"`
}

type BackupVerifyResult struct {
	Name          string    `json:"name"`
	Valid         bool      `json:"valid"`
	Integrity     string    `json:"integrity"`
	SchemaVersion int       `json:"schemaVersion"`
	Projects      int       `json:"projects"`
	Resources     int       `json:"resources"`
	Agents        int       `json:"agents"`
	CheckedAt     time.Time `json:"checkedAt"`
}

func (c *BackupRemoteConfig) Normalize() {
	c.Provider = strings.ToLower(strings.TrimSpace(c.Provider))
	c.Prefix = strings.Trim(strings.ReplaceAll(strings.TrimSpace(c.Prefix), "\\", "/"), "/")
	c.WebDAVURL = strings.TrimRight(strings.TrimSpace(c.WebDAVURL), "/")
	c.WebDAVUsername = strings.TrimSpace(c.WebDAVUsername)
	c.S3Endpoint = strings.TrimRight(strings.TrimSpace(c.S3Endpoint), "/")
	c.S3Region = strings.TrimSpace(c.S3Region)
	if c.S3Region == "" {
		c.S3Region = "us-east-1"
	}
	c.S3Bucket = strings.Trim(strings.TrimSpace(c.S3Bucket), "/")
	c.S3AccessKey = strings.TrimSpace(c.S3AccessKey)
}

func (c BackupRemoteConfig) Validate() error {
	if strings.Contains(c.Prefix, "..") || strings.ContainsRune(c.Prefix, '\x00') {
		return errors.New("prefijo remoto invalido")
	}
	if !c.Enabled && c.Provider == "" {
		return nil
	}
	switch c.Provider {
	case "webdav":
		if err := validateRemoteHTTPURL(c.WebDAVURL, c.AllowInsecure); err != nil {
			return fmt.Errorf("WebDAV: %w", err)
		}
	case "s3":
		if err := validateRemoteHTTPURL(c.S3Endpoint, c.AllowInsecure); err != nil {
			return fmt.Errorf("S3: %w", err)
		}
		if c.S3Region == "" || c.S3Bucket == "" || c.S3AccessKey == "" || c.S3SecretKey == "" {
			return errors.New("S3 requiere endpoint, region, bucket, access key y secret key")
		}
		if strings.Contains(c.S3Bucket, "/") || strings.Contains(c.S3Bucket, "..") {
			return errors.New("bucket S3 invalido")
		}
	case "":
		return errors.New("selecciona un destino remoto")
	default:
		return errors.New("proveedor remoto no soportado")
	}
	return nil
}

func validateRemoteHTTPURL(raw string, allowInsecure bool) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("URL HTTP/HTTPS invalida")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("la URL no debe incluir credenciales, query ni fragmento")
	}
	if u.Scheme != "https" && !allowInsecure {
		return errors.New("HTTP sin TLS requiere habilitar conexion insegura explicitamente")
	}
	return nil
}

func (s *Store) LoadBackupRemoteConfig() BackupRemoteConfig {
	var c BackupRemoteConfig
	raw, ok := s.getSetting(backupRemoteSettingKey)
	if ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &c)
	}
	c.Normalize()
	c.WebDAVSecretSet = strings.TrimSpace(c.WebDAVPassword) != ""
	c.S3SecretSet = strings.TrimSpace(c.S3SecretKey) != ""
	return c
}

func (s *Store) SaveBackupRemoteConfig(c BackupRemoteConfig) (BackupRemoteConfig, error) {
	previous := s.LoadBackupRemoteConfig()
	c.Normalize()
	if c.Provider == "webdav" && strings.TrimSpace(c.WebDAVPassword) == "" {
		c.WebDAVPassword = previous.WebDAVPassword
	}
	if c.Provider == "s3" && strings.TrimSpace(c.S3SecretKey) == "" {
		c.S3SecretKey = previous.S3SecretKey
	}
	if err := c.Validate(); err != nil {
		return BackupRemoteConfig{}, err
	}
	c.WebDAVSecretSet = strings.TrimSpace(c.WebDAVPassword) != ""
	c.S3SecretSet = strings.TrimSpace(c.S3SecretKey) != ""
	b, err := json.Marshal(c)
	if err != nil {
		return BackupRemoteConfig{}, err
	}
	now := formatTime(time.Now().UTC())
	if _, err := s.db.Exec(`INSERT INTO app_settings(key,value,updated_at) VALUES(?,?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`, backupRemoteSettingKey, string(b), now); err != nil {
		return BackupRemoteConfig{}, fmt.Errorf("guardar destino remoto: %w", err)
	}
	return c, nil
}

func publicBackupRemoteConfig(c BackupRemoteConfig) BackupRemoteConfig {
	c.WebDAVSecretSet = strings.TrimSpace(c.WebDAVPassword) != ""
	c.S3SecretSet = strings.TrimSpace(c.S3SecretKey) != ""
	c.WebDAVPassword = ""
	c.S3SecretKey = ""
	return c
}

func (s *Store) VerifyBackup(ctx context.Context, backupDir, name string) (BackupVerifyResult, error) {
	pathName, err := BackupPath(backupDir, name)
	if err != nil {
		return BackupVerifyResult{}, err
	}
	src, err := os.Open(pathName)
	if err != nil {
		return BackupVerifyResult{}, err
	}
	defer src.Close()
	tmpDir, err := os.MkdirTemp("", "pangolite-backup-verify-")
	if err != nil {
		return BackupVerifyResult{}, err
	}
	defer os.RemoveAll(tmpDir)
	copyPath := filepath.Join(tmpDir, "verify.db")
	dst, err := os.OpenFile(copyPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return BackupVerifyResult{}, err
	}
	_, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		return BackupVerifyResult{}, copyErr
	}
	if closeErr != nil {
		return BackupVerifyResult{}, closeErr
	}
	verifyStore, err := NewStore(copyPath)
	if err != nil {
		return BackupVerifyResult{}, fmt.Errorf("el respaldo no puede abrirse/restaurarse: %w", err)
	}
	defer verifyStore.Close()
	var integrity string
	if err := verifyStore.db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return BackupVerifyResult{}, fmt.Errorf("integrity_check: %w", err)
	}
	version, err := verifyStore.SchemaVersion(ctx)
	if err != nil {
		return BackupVerifyResult{}, err
	}
	result := BackupVerifyResult{
		Name: name, Valid: strings.EqualFold(strings.TrimSpace(integrity), "ok"), Integrity: integrity,
		SchemaVersion: version, Projects: len(verifyStore.ListProjects()), Resources: len(verifyStore.ListResources()), Agents: verifyStore.CountAgents(), CheckedAt: time.Now().UTC(),
	}
	if !result.Valid {
		return result, errors.New("SQLite reporto integridad invalida")
	}
	return result, nil
}

func remoteObjectName(prefix, name string) string {
	name = path.Base(strings.TrimSpace(name))
	prefix = strings.Trim(strings.ReplaceAll(strings.TrimSpace(prefix), "\\", "/"), "/")
	if prefix == "" {
		return name
	}
	return prefix + "/" + name
}

func uploadBackupRemote(ctx context.Context, client *http.Client, c BackupRemoteConfig, localPath, name string) error {
	c.Normalize()
	if err := c.Validate(); err != nil {
		return err
	}
	if client == nil {
		// El contexto del llamador controla el tiempo total. Evita un timeout
		// adicional que pueda cortar respaldos grandes en enlaces lentos.
		client = &http.Client{}
	}
	switch c.Provider {
	case "webdav":
		return webdavUpload(ctx, client, c, localPath, remoteObjectName(c.Prefix, name))
	case "s3":
		return s3Upload(ctx, client, c, localPath, remoteObjectName(c.Prefix, name))
	default:
		return errors.New("proveedor remoto no soportado")
	}
}

func webdavUpload(ctx context.Context, client *http.Client, c BackupRemoteConfig, localPath, object string) error {
	base, err := url.Parse(c.WebDAVURL)
	if err != nil {
		return err
	}
	parts := strings.Split(path.Dir(object), "/")
	current := strings.TrimRight(base.Path, "/")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		current += "/" + part
		u := *base
		u.Path = current
		req, _ := http.NewRequestWithContext(ctx, "MKCOL", u.String(), nil)
		if c.WebDAVUsername != "" {
			req.SetBasicAuth(c.WebDAVUsername, c.WebDAVPassword)
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("WebDAV MKCOL %s: HTTP %d", current, resp.StatusCode)
		}
	}
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	u := *base
	u.Path = strings.TrimRight(base.Path, "/") + "/" + object
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), file)
	if err != nil {
		return err
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/vnd.sqlite3")
	if c.WebDAVUsername != "" {
		req.SetBasicAuth(c.WebDAVUsername, c.WebDAVPassword)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("WebDAV PUT: HTTP %d", resp.StatusCode)
	}
	return nil
}

func s3Upload(ctx context.Context, client *http.Client, c BackupRemoteConfig, localPath, object string) error {
	payloadHash, size, err := hashFile(localPath)
	if err != nil {
		return err
	}
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()
	endpoint, err := url.Parse(c.S3Endpoint)
	if err != nil {
		return err
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/" + c.S3Bucket + "/" + object
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint.String(), file)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/vnd.sqlite3")
	signS3Request(req, c, payloadHash, time.Now().UTC())
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("S3 PutObject: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func hashFile(name string) (string, int64, error) {
	f, err := os.Open(name)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func signS3Request(req *http.Request, c BackupRemoteConfig, payloadHash string, now time.Time) {
	amzDate := now.UTC().Format("20060102T150405Z")
	date := now.UTC().Format("20060102")
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalHeaders := "host:" + req.URL.Host + "\n" + "x-amz-content-sha256:" + payloadHash + "\n" + "x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := req.Method + "\n" + canonicalURI + "\n" + canonicalQuery(req.URL.Query()) + "\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash
	scope := date + "/" + c.S3Region + "/s3/aws4_request"
	h := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(h[:])
	kDate := hmacSHA256([]byte("AWS4"+c.S3SecretKey), date)
	kRegion := hmacSHA256(kDate, c.S3Region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	sig := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+c.S3AccessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+sig)
}

func canonicalQuery(values url.Values) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		vals := append([]string(nil), values[k]...)
		sort.Strings(vals)
		if len(vals) == 0 {
			pairs = append(pairs, url.QueryEscape(k)+"=")
			continue
		}
		for _, v := range vals {
			pairs = append(pairs, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.ReplaceAll(strings.Join(pairs, "&"), "+", "%20")
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(data))
	return h.Sum(nil)
}

func deleteBackupRemote(ctx context.Context, client *http.Client, c BackupRemoteConfig, name string) error {
	c.Normalize()
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	object := remoteObjectName(c.Prefix, name)
	switch c.Provider {
	case "webdav":
		base, err := url.Parse(c.WebDAVURL)
		if err != nil {
			return err
		}
		base.Path = strings.TrimRight(base.Path, "/") + "/" + object
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, base.String(), nil)
		if c.WebDAVUsername != "" {
			req.SetBasicAuth(c.WebDAVUsername, c.WebDAVPassword)
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("WebDAV DELETE: HTTP %d", resp.StatusCode)
		}
		return nil
	case "s3":
		endpoint, err := url.Parse(c.S3Endpoint)
		if err != nil {
			return err
		}
		endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/" + c.S3Bucket + "/" + object
		req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint.String(), nil)
		empty := sha256.Sum256(nil)
		signS3Request(req, c, hex.EncodeToString(empty[:]), time.Now().UTC())
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("S3 DeleteObject: HTTP %d", resp.StatusCode)
		}
		return nil
	}
	return errors.New("proveedor remoto no soportado")
}

func (s *Server) getBackupRemoteConfig(w http.ResponseWriter, _ *http.Request, _ requestSession) {
	writeJSON(w, http.StatusOK, publicBackupRemoteConfig(s.store.LoadBackupRemoteConfig()))
}
func (s *Server) saveBackupRemoteConfig(w http.ResponseWriter, r *http.Request, rs requestSession) {
	defer r.Body.Close()
	var c BackupRemoteConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&c); err != nil {
		writeError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	saved, err := s.store.SaveBackupRemoteConfig(c)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.recordAudit(r, rs, "backup.remote.configure", "backup", "remote", "", map[string]any{"provider": saved.Provider, "enabled": saved.Enabled, "autoUpload": saved.AutoUpload})
	writeJSON(w, http.StatusOK, publicBackupRemoteConfig(saved))
}
func (s *Server) verifyBackup(w http.ResponseWriter, r *http.Request, rs requestSession) {
	result, err := s.store.VerifyBackup(r.Context(), s.config.BackupDir, r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.recordAudit(r, rs, "backup.verify", "backup", result.Name, "", map[string]any{"schemaVersion": result.SchemaVersion, "projects": result.Projects, "resources": result.Resources, "agents": result.Agents})
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) uploadBackupRemote(w http.ResponseWriter, r *http.Request, rs requestSession) {
	c := s.store.LoadBackupRemoteConfig()
	if !c.Enabled {
		writeError(w, http.StatusBadRequest, "destino remoto desactivado")
		return
	}
	name := r.PathValue("name")
	local, err := BackupPath(s.config.BackupDir, name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	if err := uploadBackupRemote(ctx, nil, c, local, name); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	s.recordAudit(r, rs, "backup.remote.upload", "backup", name, "", map[string]any{"provider": c.Provider})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": name, "provider": c.Provider})
}
func (s *Server) testBackupRemote(w http.ResponseWriter, r *http.Request, rs requestSession) {
	c := s.store.LoadBackupRemoteConfig()
	if !c.Enabled {
		writeError(w, http.StatusBadRequest, "destino remoto desactivado")
		return
	}
	tmp, err := os.CreateTemp("", "pangolite-remote-test-*.db")
	if err != nil {
		writeError(w, 500, err.Error())
		return
	}
	name := filepath.Base(tmp.Name())
	_, _ = tmp.Write([]byte("pangolite remote test\n"))
	_ = tmp.Close()
	defer os.Remove(tmp.Name())
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if err := uploadBackupRemote(ctx, nil, c, tmp.Name(), name); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := deleteBackupRemote(ctx, nil, c, name); err != nil {
		writeError(w, http.StatusBadGateway, "se pudo escribir pero no limpiar la prueba remota: "+err.Error())
		return
	}
	s.recordAudit(r, rs, "backup.remote.test", "backup", "remote", "", map[string]any{"provider": c.Provider})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "provider": c.Provider})
}
