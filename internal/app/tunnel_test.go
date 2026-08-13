package app

import (
	"context"
	"errors"
	"net"
	"strings"
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

func TestTunnelHubRemoveAgentReleasesQueues(t *testing.T) {
	hub := NewTunnelHub(4)
	_ = hub.queue("agent-a")
	_ = hub.streamQueue("agent-a")
	if len(hub.queues) != 1 || len(hub.streamQueues) != 1 {
		t.Fatalf("colas iniciales inesperadas: %d/%d", len(hub.queues), len(hub.streamQueues))
	}
	hub.RemoveAgent("agent-a")
	if len(hub.queues) != 0 || len(hub.streamQueues) != 0 {
		t.Fatalf("colas no liberadas: %d/%d", len(hub.queues), len(hub.streamQueues))
	}
}

func TestTunnelHubAttachedStreamOutlivesAttachTimeout(t *testing.T) {
	hub := NewTunnelHub(2)
	hub.streamAttachTimeout = 40 * time.Millisecond
	client, peer := net.Pipe()
	defer client.Close()
	defer peer.Close()

	errCh := make(chan error, 1)
	job := AgentStreamJob{ID: "stream-largo", Mode: ModeTCP}
	go func() {
		errCh <- hub.SubmitStream(context.Background(), "agent-1", job, client)
	}()

	pollCtx, pollCancel := context.WithTimeout(context.Background(), time.Second)
	defer pollCancel()
	if _, ok, err := hub.PollStream(pollCtx, "agent-1"); err != nil || !ok {
		t.Fatalf("no se pudo obtener stream: ok=%v err=%v", ok, err)
	}
	if _, ok := hub.AttachStream(job.ID, "agent-1"); !ok {
		t.Fatal("no se pudo adjuntar stream")
	}

	time.Sleep(3 * hub.streamAttachTimeout)
	select {
	case err := <-errCh:
		t.Fatalf("el stream termino por el timeout de adjunto: %v", err)
	default:
	}

	hub.CompleteStream(job.ID)
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("SubmitStream termino con error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SubmitStream no termino despues de completar el stream")
	}
}

func TestTunnelHubAttachTimeoutOnlyAppliesBeforeHandshake(t *testing.T) {
	hub := NewTunnelHub(2)
	hub.streamAttachTimeout = 30 * time.Millisecond
	client, peer := net.Pipe()
	defer client.Close()
	defer peer.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- hub.SubmitStream(context.Background(), "agent-1", AgentStreamJob{ID: "sin-adjunto", Mode: ModeTCP}, client)
	}()

	pollCtx, pollCancel := context.WithTimeout(context.Background(), time.Second)
	defer pollCancel()
	if _, ok, err := hub.PollStream(pollCtx, "agent-1"); err != nil || !ok {
		t.Fatalf("no se pudo obtener stream: ok=%v err=%v", ok, err)
	}
	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "timeout esperando conexion") {
			t.Fatalf("error inesperado: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("el timeout de adjunto no libero el stream")
	}
}

func TestTunnelHubTracksTerminalUploadCapability(t *testing.T) {
	hub := NewTunnelHub(2)
	hub.UpdateAgentCapabilities("agent-terminal", AgentCapabilityHTTPStreamV1+","+AgentCapabilityTerminalUploadV1)
	if !hub.AgentSupports("agent-terminal", AgentCapabilityTerminalUploadV1) {
		t.Fatal("capacidad de subida por terminal no registrada")
	}
}

func TestTunnelHubAgentCapabilitiesExpire(t *testing.T) {
	hub := NewTunnelHub(2)
	hub.UpdateAgentCapabilities("agent-1", "tcp-stream-v1, "+AgentCapabilityHTTPStreamV1)
	if !hub.AgentSupports("agent-1", AgentCapabilityHTTPStreamV1) {
		t.Fatal("capacidad HTTP streaming no registrada")
	}

	hub.mu.Lock()
	state := hub.capabilities["agent-1"]
	state.SeenAt = time.Now().Add(-agentCapabilityTTL - time.Second)
	hub.capabilities["agent-1"] = state
	hub.mu.Unlock()
	if hub.AgentSupports("agent-1", AgentCapabilityHTTPStreamV1) {
		t.Fatal("capacidad expirada siguio activa")
	}
}

func TestTunnelHubEmptyCapabilitiesDowngradesAgentImmediately(t *testing.T) {
	hub := NewTunnelHub(2)
	hub.UpdateAgentCapabilities("agent-1", AgentCapabilityHTTPStreamV1)
	if !hub.AgentSupports("agent-1", AgentCapabilityHTTPStreamV1) {
		t.Fatal("capacidad no registrada")
	}
	hub.UpdateAgentCapabilities("agent-1", "")
	if hub.AgentSupports("agent-1", AgentCapabilityHTTPStreamV1) {
		t.Fatal("un agente sin cabecera de capacidades conservo un protocolo incompatible")
	}
}
