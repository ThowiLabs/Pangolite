package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	AgentPollTimeout        = 25 * time.Second
	AgentStreamModeTerminal = "terminal"
)

type AgentJob struct {
	ID           string      `json:"id"`
	Kind         string      `json:"kind,omitempty"`
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

type AgentStreamJob struct {
	ID         string `json:"id"`
	ResourceID string `json:"resourceId"`
	Mode       string `json:"mode"`
	TargetHost string `json:"targetHost"`
	TargetPort int    `json:"targetPort"`
	Shell      string `json:"shell,omitempty"`
	Cols       int    `json:"cols,omitempty"`
	Rows       int    `json:"rows,omitempty"`
}

type StreamSession struct {
	ID         string
	AgentID    string
	ClientConn net.Conn
	Attached   chan struct{}
	Done       chan struct{}
	CreatedAt  time.Time
}

type TunnelHub struct {
	mu           sync.Mutex
	queues       map[string]chan AgentJob
	streamQueues map[string]chan AgentStreamJob
	pending      map[string]chan AgentResponse
	streams      map[string]*StreamSession
	maxQ         int
}

func NewTunnelHub(maxQueue int) *TunnelHub {
	if maxQueue < 1 {
		maxQueue = 32
	}
	return &TunnelHub{queues: map[string]chan AgentJob{}, streamQueues: map[string]chan AgentStreamJob{}, pending: map[string]chan AgentResponse{}, streams: map[string]*StreamSession{}, maxQ: maxQueue}
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

func (h *TunnelHub) PollStream(ctx context.Context, agentID string) (AgentStreamJob, bool, error) {
	q := h.streamQueue(agentID)
	select {
	case job := <-q:
		return job, true, nil
	case <-ctx.Done():
		return AgentStreamJob{}, false, ctx.Err()
	}
}

func (h *TunnelHub) SubmitStream(ctx context.Context, agentID string, job AgentStreamJob, clientConn net.Conn) error {
	if agentID == "" || job.ID == "" {
		return errors.New("stream invalido")
	}
	q := h.streamQueue(agentID)
	sess := &StreamSession{ID: job.ID, AgentID: agentID, ClientConn: clientConn, Attached: make(chan struct{}), Done: make(chan struct{}), CreatedAt: time.Now().UTC()}
	h.mu.Lock()
	if _, exists := h.streams[job.ID]; exists {
		h.mu.Unlock()
		return errors.New("stream duplicado")
	}
	h.streams[job.ID] = sess
	h.mu.Unlock()
	defer h.deleteStream(job.ID)

	select {
	case q <- job:
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("cola de streams llena o cliente sin consumo activo")
	}

	select {
	case <-sess.Attached:
	case <-ctx.Done():
		return ctx.Err()
	}

	<-sess.Done
	return nil
}

func (h *TunnelHub) AttachStream(streamID, agentID string) (*StreamSession, bool) {
	streamID = strings.TrimSpace(streamID)
	agentID = strings.TrimSpace(agentID)
	h.mu.Lock()
	defer h.mu.Unlock()
	sess, ok := h.streams[streamID]
	if !ok || sess.AgentID != agentID {
		return nil, false
	}
	select {
	case <-sess.Attached:
	default:
		close(sess.Attached)
	}
	return sess, true
}

func (h *TunnelHub) CompleteStream(streamID string) {
	h.mu.Lock()
	sess, ok := h.streams[streamID]
	h.mu.Unlock()
	if !ok {
		return
	}
	select {
	case <-sess.Done:
	default:
		close(sess.Done)
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

func (h *TunnelHub) streamQueue(agentID string) chan AgentStreamJob {
	agentID = strings.TrimSpace(agentID)
	h.mu.Lock()
	defer h.mu.Unlock()
	q, ok := h.streamQueues[agentID]
	if !ok {
		q = make(chan AgentStreamJob, h.maxQ)
		h.streamQueues[agentID] = q
	}
	return q
}

func (h *TunnelHub) deletePending(jobID string) {
	h.mu.Lock()
	delete(h.pending, jobID)
	h.mu.Unlock()
}

func (h *TunnelHub) deleteStream(streamID string) {
	h.mu.Lock()
	delete(h.streams, streamID)
	h.mu.Unlock()
}
