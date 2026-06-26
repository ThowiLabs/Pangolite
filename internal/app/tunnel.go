package app

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	AgentPollTimeout = 25 * time.Second
)

type AgentJob struct {
	ID           string      `json:"id"`
	ResourceID   string      `json:"resourceId"`
	Method       string      `json:"method"`
	Path         string      `json:"path"`
	RawQuery     string      `json:"rawQuery,omitempty"`
	Header       http.Header `json:"header,omitempty"`
	Body         []byte      `json:"body,omitempty"`
	TargetScheme string      `json:"targetScheme"`
	TargetHost   string      `json:"targetHost"`
	TargetPort   int         `json:"targetPort"`
}

type AgentResponse struct {
	JobID      string      `json:"jobId"`
	StatusCode int         `json:"statusCode"`
	Header     http.Header `json:"header,omitempty"`
	Body       []byte      `json:"body,omitempty"`
	Error      string      `json:"error,omitempty"`
}

type TunnelHub struct {
	mu      sync.Mutex
	queues  map[string]chan AgentJob
	pending map[string]chan AgentResponse
	maxQ    int
}

func NewTunnelHub(maxQueue int) *TunnelHub {
	if maxQueue < 1 {
		maxQueue = 32
	}
	return &TunnelHub{queues: map[string]chan AgentJob{}, pending: map[string]chan AgentResponse{}, maxQ: maxQueue}
}

func (h *TunnelHub) Poll(ctx context.Context, agentID string) (AgentJob, bool, error) {
	q := h.queue(agentID)
	select {
	case job := <-q:
		return job, true, nil
	case <-ctx.Done():
		return AgentJob{}, false, ctx.Err()
	}
}

func (h *TunnelHub) Submit(ctx context.Context, agentID string, job AgentJob) (AgentResponse, error) {
	if agentID == "" || job.ID == "" {
		return AgentResponse{}, errors.New("job invalido")
	}
	q := h.queue(agentID)
	respCh := make(chan AgentResponse, 1)

	h.mu.Lock()
	if _, exists := h.pending[job.ID]; exists {
		h.mu.Unlock()
		return AgentResponse{}, errors.New("job duplicado")
	}
	h.pending[job.ID] = respCh
	h.mu.Unlock()
	defer h.deletePending(job.ID)

	select {
	case q <- job:
	case <-ctx.Done():
		return AgentResponse{}, ctx.Err()
	default:
		return AgentResponse{}, errors.New("cola del agente llena o agente sin consumo activo")
	}

	select {
	case resp := <-respCh:
		return resp, nil
	case <-ctx.Done():
		return AgentResponse{}, ctx.Err()
	}
}

func (h *TunnelHub) Complete(jobID string, resp AgentResponse) bool {
	h.mu.Lock()
	ch, ok := h.pending[jobID]
	h.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- resp:
		return true
	default:
		return false
	}
}

func (h *TunnelHub) queue(agentID string) chan AgentJob {
	agentID = strings.TrimSpace(agentID)
	h.mu.Lock()
	defer h.mu.Unlock()
	q, ok := h.queues[agentID]
	if !ok {
		q = make(chan AgentJob, h.maxQ)
		h.queues[agentID] = q
	}
	return q
}

func (h *TunnelHub) deletePending(jobID string) {
	h.mu.Lock()
	delete(h.pending, jobID)
	h.mu.Unlock()
}
