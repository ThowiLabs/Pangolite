package app

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
)

const (
	remoteUDPReplyTimeout    = 5 * time.Second
	remoteUDPPacketMaxSize   = 65535
	streamBufferSize         = 32 * 1024
	streamWebSocketReadLimit = 64 * 1024
	streamKeepAliveInterval  = 30 * time.Second
	streamKeepAliveTimeout   = 20 * time.Second
	streamTCPKeepAlivePeriod = 30 * time.Second
)

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

func isClosedNetworkError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "use of closed network connection") || strings.Contains(s, "network connection closed")
}
