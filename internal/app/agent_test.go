package app

import (
	"context"
	"net/http"
	"testing"
	"time"
)

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
