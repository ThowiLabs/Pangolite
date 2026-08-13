package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const (
	remoteBridgeHost         = "127.0.0.1"
	remoteUDPReplyTimeout    = 5 * time.Second
	remoteUDPPacketMaxSize   = 65535
	streamBufferSize         = 32 * 1024
	streamWebSocketReadLimit = 64 * 1024
	streamKeepAliveInterval  = 30 * time.Second
	streamKeepAliveTimeout   = 20 * time.Second
	streamTCPKeepAlivePeriod = 30 * time.Second
)

type BridgeManager struct {
	mu          sync.Mutex
	hub         *TunnelHub
	log         *slog.Logger
	listeners   map[string]io.Closer
	streamSlots chan struct{}
}

func NewBridgeManager(hub *TunnelHub, logger *slog.Logger, maxConcurrent int) *BridgeManager {
	if maxConcurrent < 1 {
		maxConcurrent = 16
	}
	return &BridgeManager{hub: hub, log: logger, listeners: map[string]io.Closer{}, streamSlots: make(chan struct{}, maxConcurrent)}
}

func (m *BridgeManager) Sync(resources []Resource) error {
	if m == nil {
		return nil
	}
	wanted := map[string]Resource{}
	for _, r := range resources {
		if !r.Enabled || !r.UsesAgent() || (r.Mode != ModeTCP && r.Mode != ModeUDP) || r.TunnelPort <= 0 {
			continue
		}
		wanted[bridgeKey(r)] = r
	}

	m.mu.Lock()
	for key, closer := range m.listeners {
		if _, ok := wanted[key]; !ok {
			_ = closer.Close()
			delete(m.listeners, key)
		}
	}
	for key, r := range wanted {
		if _, ok := m.listeners[key]; ok {
			continue
		}
		closer, err := m.startLocked(r)
		if err != nil {
			m.mu.Unlock()
			return err
		}
		m.listeners[key] = closer
	}
	m.mu.Unlock()
	return nil
}

func (m *BridgeManager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, closer := range m.listeners {
		_ = closer.Close()
		delete(m.listeners, key)
	}
}

func (m *BridgeManager) startLocked(r Resource) (io.Closer, error) {
	addr := r.BridgeAddress()
	switch r.Mode {
	case ModeTCP:
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("abrir puente TCP interno %s para %s: %w", addr, r.Name, err)
		}
		go m.acceptTCP(r, ln)
		if m.log != nil {
			m.log.Info("puente TCP remoto activo", "resource", r.ID, "listen", addr, "agent", r.AgentID)
		}
		return ln, nil
	case ModeUDP:
		pc, err := net.ListenPacket("udp", addr)
		if err != nil {
			return nil, fmt.Errorf("abrir puente UDP interno %s para %s: %w", addr, r.Name, err)
		}
		go m.acceptUDP(r, pc)
		if m.log != nil {
			m.log.Info("puente UDP remoto activo", "resource", r.ID, "listen", addr, "agent", r.AgentID)
		}
		return pc, nil
	default:
		return nil, fmt.Errorf("modo no soportado para puente remoto: %s", r.Mode)
	}
}

func (m *BridgeManager) acceptTCP(r Resource, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if !isClosedNetworkError(err) && m.log != nil {
				m.log.Warn("aceptar TCP remoto fallo", "resource", r.ID, "error", err.Error())
			}
			return
		}
		select {
		case m.streamSlots <- struct{}{}:
			go func() {
				defer func() { <-m.streamSlots }()
				m.handleTCPConn(r, conn)
			}()
		default:
			_ = conn.Close()
			if m.log != nil {
				m.log.Warn("limite de streams TCP remotos alcanzado", "resource", r.ID, "agent", r.AgentID, "limit", cap(m.streamSlots))
			}
		}
	}
}

func (m *BridgeManager) handleTCPConn(r Resource, conn net.Conn) {
	defer conn.Close()
	streamID, err := randomID()
	if err != nil {
		return
	}
	configureTCPStreamConn(conn)
	job := AgentStreamJob{ID: streamID, ResourceID: r.ID, Mode: ModeTCP, TargetHost: r.BackendHost, TargetPort: r.BackendPort}
	if err := m.hub.SubmitStream(context.Background(), r.AgentID, job, conn); err != nil && m.log != nil {
		m.log.Warn("stream TCP remoto fallo", "resource", r.ID, "agent", r.AgentID, "error", err.Error())
	}
}

func (m *BridgeManager) acceptUDP(r Resource, pc net.PacketConn) {
	buf := make([]byte, remoteUDPPacketMaxSize)
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			if !isClosedNetworkError(err) && m.log != nil {
				m.log.Warn("leer UDP remoto fallo", "resource", r.ID, "error", err.Error())
			}
			return
		}
		packet := append([]byte(nil), buf[:n]...)
		go m.handleUDPPacket(r, pc, addr, packet)
	}
}

func (m *BridgeManager) handleUDPPacket(r Resource, pc net.PacketConn, addr net.Addr, packet []byte) {
	jobID, err := randomID()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := m.hub.Submit(ctx, r.AgentID, AgentJob{ID: jobID, Kind: ModeUDP, ResourceID: r.ID, Body: packet, TargetHost: r.BackendHost, TargetPort: r.BackendPort})
	if err != nil {
		if m.log != nil {
			m.log.Warn("datagrama UDP remoto fallo", "resource", r.ID, "agent", r.AgentID, "error", err.Error())
		}
		return
	}
	if len(resp.Body) > 0 {
		_, _ = pc.WriteTo(resp.Body, addr)
	}
}

func bridgeWebSocketNetConn(ctx context.Context, ws *websocket.Conn, conn net.Conn) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	ws.SetReadLimit(streamWebSocketReadLimit)
	errc := make(chan error, 3)
	report := func(err error) {
		select {
		case errc <- err:
		case <-ctx.Done():
		}
	}

	go func() {
		buf := make([]byte, streamBufferSize)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				if werr := ws.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					report(werr)
					return
				}
			}
			if err != nil {
				report(err)
				return
			}
		}
	}()
	go func() {
		for {
			typ, data, err := ws.Read(ctx)
			if err != nil {
				report(err)
				return
			}
			if typ != websocket.MessageBinary && typ != websocket.MessageText {
				continue
			}
			if len(data) > 0 {
				if err := writeFull(conn, data); err != nil {
					report(err)
					return
				}
			}
		}
	}()
	go func() {
		ticker := time.NewTicker(streamKeepAliveInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, pingCancel := context.WithTimeout(ctx, streamKeepAliveTimeout)
				err := ws.Ping(pingCtx)
				pingCancel()
				if err != nil {
					report(err)
					return
				}
			}
		}
	}()

	err := <-errc
	cancel()
	_ = conn.Close()
	_ = ws.CloseNow()
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || websocket.CloseStatus(err) == websocket.StatusNormalClosure || websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return nil
	}
	return err
}

func writeFull(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if n > 0 {
			data = data[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func configureTCPStreamConn(conn net.Conn) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcp.SetNoDelay(true)
	_ = tcp.SetKeepAlive(true)
	_ = tcp.SetKeepAlivePeriod(streamTCPKeepAlivePeriod)
}

func runUDPAgentJob(ctx context.Context, job AgentJob) AgentResponse {
	addr := net.JoinHostPort(job.TargetHost, strconv.Itoa(job.TargetPort))
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "udp", addr)
	if err != nil {
		return AgentResponse{JobID: job.ID, StatusCode: 502, Error: err.Error()}
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(remoteUDPReplyTimeout))
	if _, err := conn.Write(job.Body); err != nil {
		return AgentResponse{JobID: job.ID, StatusCode: 502, Error: err.Error()}
	}
	buf := make([]byte, remoteUDPPacketMaxSize)
	n, err := conn.Read(buf)
	if err != nil {
		return AgentResponse{JobID: job.ID, StatusCode: 504, Error: err.Error()}
	}
	return AgentResponse{JobID: job.ID, StatusCode: 200, Body: append([]byte(nil), buf[:n]...)}
}

func bridgeKey(r Resource) string {
	return r.Mode + ":" + r.ID + ":" + strconv.Itoa(r.TunnelPort)
}

func isClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "use of closed network connection") || strings.Contains(s, "network connection closed")
}
