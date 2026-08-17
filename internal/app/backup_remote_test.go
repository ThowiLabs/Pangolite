package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestVerifyBackupRestoresTemporaryCopy(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "pangolite.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	p, err := store.AddProject(Project{Name: "Empresa"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.AddResource(Resource{ProjectID: p.ID, Name: "Panel", Mode: ModeTCP, PublicPort: 2301, BackendHost: "127.0.0.1", BackendPort: 22, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	backupDir := filepath.Join(dir, "backups")
	b, err := store.CreateBackup(context.Background(), backupDir, "verify")
	if err != nil {
		t.Fatal(err)
	}
	v, err := store.VerifyBackup(context.Background(), backupDir, b.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Valid || v.Integrity != "ok" || v.Projects < 1 || v.Resources != 1 {
		t.Fatalf("verificacion inesperada: %+v", v)
	}
}

func TestWebDAVUploadCreatesCollectionsAndStreams(t *testing.T) {
	var mu sync.Mutex
	methods := []string{}
	var putBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		methods = append(methods, r.Method)
		mu.Unlock()
		switch r.Method {
		case "MKCOL":
			w.WriteHeader(http.StatusCreated)
		case "PUT":
			b, _ := io.ReadAll(r.Body)
			putBody = string(b)
			w.WriteHeader(http.StatusCreated)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer ts.Close()
	f := filepath.Join(t.TempDir(), "pangolite-test.db")
	if err := os.WriteFile(f, []byte("sqlite-data"), 0600); err != nil {
		t.Fatal(err)
	}
	c := BackupRemoteConfig{Enabled: true, Provider: "webdav", Prefix: "empresa/backups", WebDAVURL: ts.URL, AllowInsecure: true}
	if err := uploadBackupRemote(context.Background(), ts.Client(), c, f, "pangolite-test.db"); err != nil {
		t.Fatal(err)
	}
	if putBody != "sqlite-data" {
		t.Fatalf("body inesperado %q", putBody)
	}
	if err := deleteBackupRemote(context.Background(), ts.Client(), c, "pangolite-test.db"); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(methods, ",")
	if !strings.Contains(joined, "MKCOL") || !strings.Contains(joined, "PUT") || !strings.Contains(joined, "DELETE") {
		t.Fatalf("metodos inesperados: %s", joined)
	}
}

func TestS3UploadUsesSigV4AndPathStyle(t *testing.T) {
	var auth, pathSeen, hashSeen string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		hashSeen = r.Header.Get("x-amz-content-sha256")
		pathSeen = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	f := filepath.Join(t.TempDir(), "pangolite-test.db")
	if err := os.WriteFile(f, []byte("sqlite-data"), 0600); err != nil {
		t.Fatal(err)
	}
	c := BackupRemoteConfig{Enabled: true, Provider: "s3", Prefix: "pangolite/backups", S3Endpoint: ts.URL + "/namespace", S3Region: "us-east-1", S3Bucket: "bucket", S3AccessKey: "AKID", S3SecretKey: "SECRET", AllowInsecure: true}
	if err := uploadBackupRemote(context.Background(), ts.Client(), c, f, "pangolite-test.db"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") || hashSeen == "" {
		t.Fatalf("firma ausente: auth=%q hash=%q", auth, hashSeen)
	}
	if pathSeen != "/namespace/bucket/pangolite/backups/pangolite-test.db" {
		t.Fatalf("ruta inesperada: %s", pathSeen)
	}
}

func TestS3SignatureMatchesReference(t *testing.T) {
	req, err := http.NewRequest(http.MethodPut, "https://s3.hf.co/namespace/bucket/pangolite/backups/x.db", nil)
	if err != nil {
		t.Fatal(err)
	}
	c := BackupRemoteConfig{S3Region: "us-east-1", S3AccessKey: "AKID", S3SecretKey: "SECRET"}
	payloadHash := "2e63ab81280987e8f16175a49baca369a7a989b9c6e3fbbd397bcea3114f7fd3"
	signS3Request(req, c, payloadHash, time.Date(2026, 8, 17, 18, 30, 0, 0, time.UTC))
	want := "AWS4-HMAC-SHA256 Credential=AKID/20260817/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature=b88a6e5f49de8bf1c3cba32459f802d16dc7ee55c30f127369f4691acdd8c2ef"
	if got := req.Header.Get("Authorization"); got != want {
		t.Fatalf("firma inesperada\ngot:  %s\nwant: %s", got, want)
	}
}

func TestBackupRemoteRejectsPlainHTTPByDefault(t *testing.T) {
	c := BackupRemoteConfig{Enabled: true, Provider: "webdav", WebDAVURL: "http://127.0.0.1/dav"}
	c.Normalize()
	if err := c.Validate(); err == nil {
		t.Fatal("HTTP sin TLS debe rechazarse salvo habilitacion explicita")
	}
}

func TestBackupRemoteConfigPreservesSecrets(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "pangolite.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	c := BackupRemoteConfig{Enabled: true, Provider: "s3", S3Endpoint: "https://s3.example.com", S3Region: "us-east-1", S3Bucket: "b", S3AccessKey: "a", S3SecretKey: "secret"}
	if _, err = store.SaveBackupRemoteConfig(c); err != nil {
		t.Fatal(err)
	}
	c.S3SecretKey = ""
	c.S3AccessKey = "a2"
	saved, err := store.SaveBackupRemoteConfig(c)
	if err != nil {
		t.Fatal(err)
	}
	if saved.S3SecretKey != "secret" || !saved.S3SecretSet {
		t.Fatal("el secret existente no se conservo")
	}
	if publicBackupRemoteConfig(saved).S3SecretKey != "" {
		t.Fatal("el secret no debe exponerse")
	}
}
