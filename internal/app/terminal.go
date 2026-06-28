package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"nhooyr.io/websocket"
)

type terminalStartOptions struct {
	Shell string
	Cols  int
	Rows  int
}

type terminalProcess struct {
	rw     io.ReadWriteCloser
	resize func(cols, rows int) error
}

func (p *terminalProcess) Read(b []byte) (int, error) {
	if p == nil || p.rw == nil {
		return 0, io.ErrClosedPipe
	}
	return p.rw.Read(b)
}

func (p *terminalProcess) Write(b []byte) (int, error) {
	if p == nil || p.rw == nil {
		return 0, io.ErrClosedPipe
	}
	return p.rw.Write(b)
}

func (p *terminalProcess) Close() error {
	if p == nil || p.rw == nil {
		return nil
	}
	return p.rw.Close()
}

func (p *terminalProcess) Resize(cols, rows int) error {
	if p == nil || p.resize == nil {
		return nil
	}
	return p.resize(cols, rows)
}

type terminalControlMessage struct {
	PangoliteTerminal bool   `json:"pangoliteTerminal,omitempty"`
	Type              string `json:"type"`
	Cols              int    `json:"cols"`
	Rows              int    `json:"rows"`
}

var terminalControlPrefix = []byte("\x00PANGOLITE-TERMINAL-CONTROL ")

func (s *Server) localTerminalSocket(w http.ResponseWriter, r *http.Request) {
	rs, ok := s.authorizeTerminalWebSocket(w, r)
	if !ok {
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		if s.log != nil {
			s.log.Warn("websocket de terminal local rechazado", "error", err.Error())
		}
		return
	}
	defer ws.Close(websocket.StatusNormalClosure, "")

	cols, rows := terminalSizeFromRequest(r)
	term, err := startTerminalProcess(r.Context(), terminalStartOptions{Cols: cols, Rows: rows})
	if err != nil {
		_ = ws.Write(r.Context(), websocket.MessageText, []byte("No se pudo iniciar la terminal local: "+err.Error()+"\r\n"))
		return
	}
	defer term.Close()
	if s.log != nil {
		s.log.Info("terminal local abierta", "user", rs.User.Username, "remote", r.RemoteAddr)
	}
	s.recordAudit(r, rs, "terminal.open", "terminal", "local", "", map[string]any{"target": "local"})
	if err := bridgeWebSocketTerminalProcess(r.Context(), ws, term, true); err != nil && s.log != nil {
		s.log.Debug("terminal local cerrada", "user", rs.User.Username, "error", err.Error())
	}
}

func (s *Server) agentTerminalSocket(w http.ResponseWriter, r *http.Request) {
	rs, ok := s.authorizeTerminalWebSocket(w, r)
	if !ok {
		return
	}
	agentID := strings.TrimSpace(r.PathValue("id"))
	if agentID == "" {
		writeError(w, http.StatusBadRequest, "cliente requerido")
		return
	}
	agent, err := s.store.AgentByID(agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "cliente de sistema no encontrado")
		return
	}
	if !agent.Enabled {
		writeError(w, http.StatusForbidden, "cliente de sistema inactivo")
		return
	}
	if !agent.Online {
		writeError(w, http.StatusBadRequest, "cliente de sistema offline; espera a que vuelva a conectarse")
		return
	}
	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		if s.log != nil {
			s.log.Warn("websocket de terminal remota rechazado", "agent", agentID, "error", err.Error())
		}
		return
	}
	defer ws.Close(websocket.StatusNormalClosure, "")

	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	streamID, err := randomID()
	if err != nil {
		_ = ws.Write(r.Context(), websocket.MessageText, []byte("No se pudo crear la sesión remota\r\n"))
		return
	}
	cols, rows := terminalSizeFromRequest(r)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	job := AgentStreamJob{ID: streamID, Mode: AgentStreamModeTerminal, Cols: cols, Rows: rows}
	errCh := make(chan error, 2)
	go func() {
		errCh <- s.hub.SubmitStream(ctx, agentID, job, left)
	}()
	go func() {
		errCh <- bridgeWebSocketRemoteTerminal(ctx, ws, right, true)
	}()
	if s.log != nil {
		s.log.Info("terminal remota solicitada", "user", rs.User.Username, "agent", agentID, "name", agent.Name)
	}
	s.recordAudit(r, rs, "terminal.open", "agent", agentID, agent.ProjectID, map[string]any{"target": agent.Name, "os": agent.OS, "hostname": agent.Hostname})
	err = <-errCh
	cancel()
	_ = left.Close()
	_ = right.Close()
	if err != nil && !errors.Is(err, context.Canceled) && s.log != nil {
		s.log.Debug("terminal remota cerrada", "agent", agentID, "error", err.Error())
	}
}

func (s *Server) authorizeTerminalWebSocket(w http.ResponseWriter, r *http.Request) (requestSession, bool) {
	if !sameOriginWebSocket(r) {
		writeError(w, http.StatusForbidden, "origen no permitido")
		return requestSession{}, false
	}
	rs, ok := s.currentSession(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "sesion requerida")
		return requestSession{}, false
	}
	if rs.User.ForcePasswordChange {
		writeError(w, http.StatusForbidden, "debes cambiar la contraseña temporal antes de usar la terminal")
		return requestSession{}, false
	}
	return rs, true
}

func sameOriginWebSocket(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func terminalSizeFromRequest(r *http.Request) (int, int) {
	cols := intFromQuery(r, "cols", 80)
	rows := intFromQuery(r, "rows", 24)
	if cols < 20 || cols > 400 {
		cols = 80
	}
	if rows < 5 || rows > 120 {
		rows = 24
	}
	return cols, rows
}

func handleTerminalControlPayload(term *terminalProcess, data []byte) bool {
	if bytes.HasPrefix(data, terminalControlPrefix) {
		payload := bytes.TrimSpace(bytes.TrimPrefix(data, terminalControlPrefix))
		if len(payload) > 0 && payload[0] == '{' {
			if msg, ok := decodeTerminalControlJSON(payload); ok {
				applyTerminalControl(term, msg)
				return true
			}
		}
	}
	msg, ok := decodeTerminalControlJSON(data)
	if !ok {
		return false
	}
	applyTerminalControl(term, msg)
	return true
}

func decodeTerminalControlJSON(data []byte) (terminalControlMessage, bool) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return terminalControlMessage{}, false
	}
	var msg terminalControlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return terminalControlMessage{}, false
	}
	if !msg.PangoliteTerminal && msg.Type == "" {
		return terminalControlMessage{}, false
	}
	if msg.Type == "" {
		return terminalControlMessage{}, false
	}
	return msg, true
}

func applyTerminalControl(term *terminalProcess, msg terminalControlMessage) {
	switch msg.Type {
	case "resize":
		_ = term.Resize(msg.Cols, msg.Rows)
	}
}

func encodeTerminalControlMessage(msg terminalControlMessage) []byte {
	msg.PangoliteTerminal = true
	payload, err := json.Marshal(msg)
	if err != nil {
		return nil
	}
	out := make([]byte, 0, len(terminalControlPrefix)+16+len(payload))
	out = append(out, terminalControlPrefix...)
	out = strconv.AppendInt(out, int64(len(payload)), 10)
	out = append(out, '\n')
	out = append(out, payload...)
	return out
}

type terminalControlFilter struct {
	buf []byte
}

func (f *terminalControlFilter) Payloads(term *terminalProcess, data []byte) [][]byte {
	if len(f.buf) > 0 {
		data = append(append([]byte(nil), f.buf...), data...)
		f.buf = nil
	}
	var payloads [][]byte
	for len(data) > 0 {
		idx := bytes.Index(data, terminalControlPrefix)
		if idx < 0 {
			if terminalControlPrefixFragment(data) {
				f.buf = append(f.buf[:0], data...)
				return payloads
			}
			payloads = append(payloads, append([]byte(nil), data...))
			return payloads
		}
		if idx > 0 {
			payloads = append(payloads, append([]byte(nil), data[:idx]...))
			data = data[idx:]
		}
		if !bytes.HasPrefix(data, terminalControlPrefix) {
			continue
		}
		afterPrefix := data[len(terminalControlPrefix):]
		nl := bytes.IndexByte(afterPrefix, '\n')
		if nl < 0 {
			f.buf = append(f.buf[:0], data...)
			return payloads
		}
		lengthText := strings.TrimSpace(string(afterPrefix[:nl]))
		n, err := strconv.Atoi(lengthText)
		if err != nil || n < 0 || n > 64*1024 {
			payloads = append(payloads, append([]byte(nil), data[:1]...))
			data = data[1:]
			continue
		}
		payloadStart := len(terminalControlPrefix) + nl + 1
		if len(data) < payloadStart+n {
			f.buf = append(f.buf[:0], data...)
			return payloads
		}
		var msg terminalControlMessage
		if err := json.Unmarshal(data[payloadStart:payloadStart+n], &msg); err == nil {
			applyTerminalControl(term, msg)
		}
		data = data[payloadStart+n:]
	}
	return payloads
}

func terminalControlPrefixFragment(data []byte) bool {
	max := len(data)
	if max > len(terminalControlPrefix)-1 {
		max = len(terminalControlPrefix) - 1
	}
	for n := max; n > 0; n-- {
		if bytes.Equal(data[len(data)-n:], terminalControlPrefix[:n]) {
			return true
		}
	}
	return false
}

func intFromQuery(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n <= 0 {
		return fallback
	}
	return n
}

func bridgeWebSocketRemoteTerminal(ctx context.Context, ws *websocket.Conn, conn net.Conn, forwardControl bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errc := make(chan error, 2)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				if werr := ws.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					errc <- werr
					return
				}
			}
			if err != nil {
				errc <- err
				return
			}
		}
	}()
	go func() {
		for {
			typ, data, err := ws.Read(ctx)
			if err != nil {
				errc <- err
				return
			}
			if typ != websocket.MessageBinary && typ != websocket.MessageText {
				continue
			}
			if typ == websocket.MessageText {
				if msg, ok := decodeTerminalControlJSON(data); ok {
					if forwardControl {
						encoded := encodeTerminalControlMessage(msg)
						if len(encoded) > 0 {
							if _, err := conn.Write(encoded); err != nil {
								errc <- err
								return
							}
						}
					}
					continue
				}
			}
			if len(data) > 0 {
				if _, err := conn.Write(data); err != nil {
					errc <- err
					return
				}
			}
		}
	}()
	err := <-errc
	cancel()
	_ = ws.Close(websocket.StatusNormalClosure, "")
	_ = conn.Close()
	if errors.Is(err, io.EOF) || websocket.CloseStatus(err) == websocket.StatusNormalClosure || websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return nil
	}
	return err
}

func bridgeWebSocketTerminalProcess(ctx context.Context, ws *websocket.Conn, term *terminalProcess, allowControl bool) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errCh := make(chan error, 2)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := term.Read(buf)
			if n > 0 {
				if werr := ws.Write(ctx, websocket.MessageBinary, buf[:n]); werr != nil {
					errCh <- werr
					return
				}
			}
			if err != nil {
				errCh <- err
				return
			}
		}
	}()
	go func() {
		var filter terminalControlFilter
		for {
			typ, data, err := ws.Read(ctx)
			if err != nil {
				errCh <- err
				return
			}
			if typ != websocket.MessageBinary && typ != websocket.MessageText {
				continue
			}
			if allowControl {
				if handled := handleTerminalControlPayload(term, data); handled {
					continue
				}
				for _, payload := range filter.Payloads(term, data) {
					if len(payload) == 0 {
						continue
					}
					if _, err := term.Write(payload); err != nil {
						errCh <- err
						return
					}
				}
				continue
			}
			if len(data) > 0 {
				if _, err := term.Write(data); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()
	err := <-errCh
	cancel()
	_ = ws.Close(websocket.StatusNormalClosure, "")
	_ = term.Close()
	if errors.Is(err, io.EOF) || websocket.CloseStatus(err) == websocket.StatusNormalClosure || websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return nil
	}
	return err
}
