package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestTunnelHubSubmitPollComplete(t *testing.T) {
	hub := NewTunnelHub(2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		job, ok, err := hub.Poll(ctx, "agent-1")
		if err != nil || !ok || job.ID != "job-1" {
			t.Errorf("job inesperado: ok=%v err=%v job=%#v", ok, err, job)
			return
		}
		hub.Complete(job.ID, AgentResponse{JobID: job.ID, StatusCode: http.StatusTeapot, Body: []byte("ok")})
	}()

	resp, err := hub.Submit(ctx, "agent-1", AgentJob{ID: "job-1", Method: http.MethodGet, Path: "/"})
	if err != nil {
		t.Fatalf("submit fallo: %v", err)
	}
	if resp.StatusCode != http.StatusTeapot || string(resp.Body) != "ok" {
		t.Fatalf("respuesta inesperada: %#v", resp)
	}
}

func TestCloneSafeHeaderDropsHopHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Connection", "upgrade")
	h.Set("Authorization", "Bearer x")
	out := cloneSafeHeader(h)
	if out.Get("Connection") != "" {
		t.Fatal("Connection no debe reenviarse")
	}
	if out.Get("Authorization") == "" {
		t.Fatal("Authorization normal debe conservarse hacia backend; no se registra en logs")
	}
}

func TestTunnelHubLargePayloadWithoutHardLimit(t *testing.T) {
	hub := NewTunnelHub(2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	largeReq := make([]byte, 20<<20)
	largeResp := make([]byte, 24<<20)

	go func() {
		job, ok, err := hub.Poll(ctx, "agent-large")
		if err != nil || !ok || len(job.Body) != len(largeReq) {
			t.Errorf("payload de request inesperado: ok=%v err=%v size=%d", ok, err, len(job.Body))
			return
		}
		hub.Complete(job.ID, AgentResponse{JobID: job.ID, StatusCode: http.StatusOK, Body: largeResp})
	}()

	resp, err := hub.Submit(ctx, "agent-large", AgentJob{ID: "job-large", Method: http.MethodPost, Path: "/upload", Body: largeReq})
	if err != nil {
		t.Fatalf("submit con payload grande fallo: %v", err)
	}
	if len(resp.Body) != len(largeResp) {
		t.Fatalf("payload de response inesperado: %d", len(resp.Body))
	}
}

func TestCloneProxyRequestHeaderDropsInternalCookies(t *testing.T) {
	h := http.Header{}
	h.Set("Cookie", "pangolite_session=secret; app_session=ok; pangolite_resource_abc=secret2")
	h.Set("X-CSRF-Token", "csrf")
	h.Set("X-Pangolite-Agent", "agent")
	out := cloneProxyRequestHeader(h)
	if out.Get("X-CSRF-Token") != "" || out.Get("X-Pangolite-Agent") != "" {
		t.Fatal("headers internos no deben reenviarse a backends")
	}
	if got := out.Get("Cookie"); got != "app_session=ok" {
		t.Fatalf("cookies filtradas inesperadas: %q", got)
	}
}

func TestAgentClientConfigAllowsFallbackURL(t *testing.T) {
	cfg := AgentClientConfig{ServerURL: "https://panel.example.mx", FallbackURL: "http://203.0.113.10:2424", AgentID: "agent01", Token: "abcdefghijklmnopqrstuvwxyz123456"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.FallbackURL = "ftp://203.0.113.10"
	if err := cfg.Validate(); err == nil {
		t.Fatal("se esperaba rechazar fallback con scheme no soportado")
	}
}

func TestRewriteEnvLinesUpdatesServerEndpoint(t *testing.T) {
	out, seen := rewriteEnvLines("# cfg\nPANGOLITE_SERVER_URL='https://old.example.com'\nPANGOLITE_AGENT_ID=agent\n", map[string]string{
		"PANGOLITE_SERVER_URL":   "https://new.example.com",
		"PANGOLITE_FALLBACK_URL": "http://203.0.113.10:2424",
	})
	joined := strings.Join(out, "\n")
	if !seen["PANGOLITE_SERVER_URL"] {
		t.Fatal("server url debe marcarse como vista")
	}
	if strings.Contains(joined, "old.example.com") || !strings.Contains(joined, "PANGOLITE_SERVER_URL=https://new.example.com") {
		t.Fatalf("server url no fue actualizada: %s", joined)
	}
	if seen["PANGOLITE_FALLBACK_URL"] {
		t.Fatal("fallback no estaba en el archivo original")
	}
}

func TestAgentEndpointPersistsDiscoveredDomainInsteadOfFallbackIP(t *testing.T) {
	envPath := t.TempDir() + "/pangolite-client.env"
	if err := os.WriteFile(envPath, []byte("PANGOLITE_SERVER_URL=https://old.example.mx\nPANGOLITE_FALLBACK_URL=http://fallback.local:2424\n"), 0600); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "fallback.local:2424" && req.URL.Path == "/healthz" {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("ok")), Header: http.Header{}}, nil
		}
		return nil, errors.New("endpoint no disponible")
	})}
	m := newAgentEndpointManager(AgentClientConfig{ServerURL: "https://old.example.mx", FallbackURL: "http://fallback.local:2424", ConfigPath: envPath, AgentID: "agent01", Token: "abcdefghijklmnopqrstuvwxyz123456"}, client, nil)
	if !m.applyDiscovery(context.Background(), AgentDiscovery{ServerURL: "https://hircoir.duckdns.org", FallbackURL: "http://fallback.local:2424"}, true) {
		t.Fatal("se esperaba aplicar discovery")
	}
	b, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(b)
	if !strings.Contains(content, "PANGOLITE_SERVER_URL=https://hircoir.duckdns.org") {
		t.Fatalf("server url debe persistir el dominio principal descubierto, contenido: %s", content)
	}
	if strings.Contains(content, "PANGOLITE_SERVER_URL=http://fallback.local:2424") {
		t.Fatalf("server url no debe persistir la IP/URL fallback como principal: %s", content)
	}
}
