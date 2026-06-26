package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type AgentClientConfig struct {
	ServerURL      string
	AgentID        string
	Token          string
	PollInterval   time.Duration
	RequestTimeout time.Duration
}

func (c AgentClientConfig) Validate() error {
	if strings.TrimSpace(c.ServerURL) == "" {
		return errors.New("server-url requerido")
	}
	if strings.TrimSpace(c.AgentID) == "" {
		return errors.New("agent-id requerido")
	}
	if strings.TrimSpace(c.Token) == "" {
		return errors.New("token requerido")
	}
	if _, err := url.ParseRequestURI(c.ServerURL); err != nil {
		return fmt.Errorf("server-url invalido: %w", err)
	}
	return nil
}

func RunAgent(ctx context.Context, cfg AgentClientConfig, logger *slog.Logger) error {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	base := strings.TrimRight(cfg.ServerURL, "/")
	client := &http.Client{}
	logger.Info("agente iniciado", "server", base, "agent", cfg.AgentID)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		job, ok, err := pollAgentJob(ctx, client, base, cfg)
		if err != nil {
			logger.Warn("poll fallo", "error", err.Error())
			sleepContext(ctx, 3*time.Second)
			continue
		}
		if !ok {
			sleepContext(ctx, cfg.PollInterval)
			continue
		}
		logger.Info("job recibido", "job", job.ID, "method", job.Method, "target", fmt.Sprintf("%s://%s:%d", job.TargetScheme, job.TargetHost, job.TargetPort))
		resp := executeAgentJob(ctx, client, job, cfg.RequestTimeout)
		if err := postAgentResponse(ctx, client, base, cfg, job.ID, resp); err != nil {
			logger.Warn("respuesta de job no enviada", "job", job.ID, "error", err.Error())
		}
	}
}

func pollAgentJob(ctx context.Context, client *http.Client, base string, cfg AgentClientConfig) (AgentJob, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/agent/poll", nil)
	if err != nil {
		return AgentJob{}, false, err
	}
	setAgentAuth(req, cfg)
	res, err := client.Do(req)
	if err != nil {
		return AgentJob{}, false, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return AgentJob{}, false, nil
	}
	if res.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return AgentJob{}, false, fmt.Errorf("estado %s: %s", res.Status, strings.TrimSpace(string(b)))
	}
	var job AgentJob
	if err := json.NewDecoder(res.Body).Decode(&job); err != nil {
		return AgentJob{}, false, err
	}
	if job.ID == "" {
		return AgentJob{}, false, errors.New("job sin id")
	}
	return job, true, nil
}

func executeAgentJob(ctx context.Context, client *http.Client, job AgentJob, requestTimeout time.Duration) AgentResponse {
	if job.TargetScheme != "http" && job.TargetScheme != "https" {
		return AgentResponse{JobID: job.ID, StatusCode: http.StatusBadGateway, Error: "scheme de backend no soportado"}
	}
	target := url.URL{Scheme: job.TargetScheme, Host: net.JoinHostPort(job.TargetHost, strconv.Itoa(job.TargetPort)), Path: job.Path, RawQuery: job.RawQuery}
	if requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, requestTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, job.Method, target.String(), bytes.NewReader(job.Body))
	if err != nil {
		return AgentResponse{JobID: job.ID, StatusCode: http.StatusBadGateway, Error: err.Error()}
	}
	copySafeHeader(req.Header, job.Header)
	res, err := client.Do(req)
	if err != nil {
		return AgentResponse{JobID: job.ID, StatusCode: http.StatusBadGateway, Error: err.Error()}
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return AgentResponse{JobID: job.ID, StatusCode: http.StatusBadGateway, Error: err.Error()}
	}
	return AgentResponse{JobID: job.ID, StatusCode: res.StatusCode, Header: cloneSafeHeader(res.Header), Body: body}
}

func postAgentResponse(ctx context.Context, client *http.Client, base string, cfg AgentClientConfig, jobID string, resp AgentResponse) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/agent/jobs/"+url.PathEscape(jobID)+"/response", bytes.NewReader(b))
	if err != nil {
		return err
	}
	setAgentAuth(req, cfg)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("estado %s: %s", res.Status, strings.TrimSpace(string(b)))
	}
	return nil
}

func setAgentAuth(req *http.Request, cfg AgentClientConfig) {
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("X-Pangolite-Agent", cfg.AgentID)
	req.Header.Set("User-Agent", "pangolite-agent/0.2")
}

func sleepContext(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
