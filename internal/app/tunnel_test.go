package app

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestTunnelHubRejectsDuplicateStreamAttachment(t *testing.T) {
	hub := NewTunnelHub(2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, peer := net.Pipe()
	defer client.Close()
	defer peer.Close()

	errCh := make(chan error, 1)
	job := AgentStreamJob{ID: "stream-duplicado", Mode: AgentStreamModeTerminal}
	go func() {
		errCh <- hub.SubmitStream(ctx, "agent-1", job, client)
	}()

	polled, ok, err := hub.PollStream(ctx, "agent-1")
	if err != nil || !ok || polled.ID != job.ID {
		t.Fatalf("poll stream = %+v ok=%v err=%v", polled, ok, err)
	}
	if _, ok := hub.AttachStream(job.ID, "agent-1"); !ok {
		t.Fatal("primer adjunto rechazado")
	}
	if _, ok := hub.AttachStream(job.ID, "agent-1"); ok {
		t.Fatal("segundo adjunto duplicado aceptado")
	}

	hub.CompleteStream(job.ID)
	if err := <-errCh; err != nil {
		t.Fatalf("SubmitStream termino con error: %v", err)
	}
}

func TestTunnelHubSubmitStreamStopsWhenContextCancelsAfterAttach(t *testing.T) {
	hub := NewTunnelHub(2)
	ctx, cancel := context.WithCancel(context.Background())
	client, peer := net.Pipe()
	defer client.Close()
	defer peer.Close()

	errCh := make(chan error, 1)
	job := AgentStreamJob{ID: "stream-cancelado", Mode: AgentStreamModeTerminal}
	go func() {
		errCh <- hub.SubmitStream(ctx, "agent-1", job, client)
	}()

	pollCtx, pollCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer pollCancel()
	if _, ok, err := hub.PollStream(pollCtx, "agent-1"); err != nil || !ok {
		t.Fatalf("no se pudo obtener stream: ok=%v err=%v", ok, err)
	}
	if _, ok := hub.AttachStream(job.ID, "agent-1"); !ok {
		t.Fatal("no se pudo adjuntar stream")
	}

	cancel()
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SubmitStream quedo bloqueado despues de cancelar el contexto")
	}
}
