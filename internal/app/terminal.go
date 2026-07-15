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
	"sync"

	"nhooyr.io/websocket"
)

type terminalStartOptions struct {
	Shell string
	Cols  int
	Rows  int
}

type terminalProcess struct {
	rw        io.ReadWriteCloser
	resize    func(cols, rows int) error
	closeOnce sync.Once
	closeErr  error
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
	if p == nil {
		return nil
	}
	p.closeOnce.Do(func() {
		if p.rw != nil {
			p.closeErr = p.rw.Close()
		}
	})
	return p.closeErr
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

type terminalControlMode uint8

const (
	terminalControlJSON terminalControlMode = 1 << iota
	terminalControlFramed
)

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
	if err := bridgeWebSocketTerminalProcess(r.Context(), ws, term, terminalControlJSON); err != nil && s.log != nil {
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

func decodeTerminalControlJSON(data []byte) (terminalControlMessage, bool) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return terminalControlMessage{}, false
	}
	var msg terminalControlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return terminalControlMessage{}, false
	}
	if !msg.PangoliteTerminal {
		return terminalControlMessage{}, false
	}
	switch msg.Type {
	case "resize":
		return msg, true
	default:
		return terminalControlMessage{}, false
	}
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
			if fragmentLen := terminalControlPrefixFragmentLen(data); fragmentLen > 0 {
				plainLen := len(data) - fragmentLen
				if plainLen > 0 {
					payloads = append(payloads, append([]byte(nil), data[:plainLen]...))
				}
				f.buf = append(f.buf[:0], data[plainLen:]...)
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
		frameEnd := payloadStart + n
		if msg, ok := decodeTerminalControlJSON(data[payloadStart:frameEnd]); ok {
			applyTerminalControl(term, msg)
		} else {
			payloads = append(payloads, append([]byte(nil), data[:frameEnd]...))
		}
		data = data[frameEnd:]
	}
	return payloads
}

func terminalControlPrefixFragmentLen(data []byte) int {
	max := len(data)
	if max > len(terminalControlPrefix)-1 {
		max = len(terminalControlPrefix) - 1
	}
	for n := max; n > 0; n-- {
		if bytes.Equal(data[len(data)-n:], terminalControlPrefix[:n]) {
			return n
		}
	}
	return 0
}

func mergeTerminalEnv(base, overrides []string) []string {
	replacements := make(map[string]string, len(overrides))
	order := make([]string, 0, len(overrides))
	for _, entry := range overrides {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		if _, exists := replacements[key]; !exists {
			order = append(order, key)
		}
		replacements[key] = entry
	}
	out := make([]string, 0, len(base)+len(replacements))
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			if _, replaced := replacements[key]; replaced {
				continue
			}
		}
		out = append(out, entry)
	}
	for _, key := range order {
		out = append(out, replacements[key])
	}
	return out
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

func writeTerminalPayload(w io.Writer, data []byte) error {
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
							if err := writeTerminalPayload(conn, encoded); err != nil {
								errc <- err
								return
							}
						}
					}
					continue
				}
			}
			if len(data) > 0 {
				if err := writeTerminalPayload(conn, data); err != nil {
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

func bridgeWebSocketTerminalProcess(ctx context.Context, ws *websocket.Conn, term *terminalProcess, controlMode terminalControlMode) error {
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
			if typ == websocket.MessageText && controlMode&terminalControlJSON != 0 {
				if msg, ok := decodeTerminalControlJSON(data); ok {
					applyTerminalControl(term, msg)
					continue
				}
			}
			if controlMode&terminalControlFramed != 0 {
				for _, payload := range filter.Payloads(term, data) {
					if len(payload) == 0 {
						continue
					}
					if err := writeTerminalPayload(term, payload); err != nil {
						errCh <- err
						return
					}
				}
				continue
			}
			if len(data) > 0 {
				if err := writeTerminalPayload(term, data); err != nil {
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
