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
)

const (
	publicL4DialTimeout       = 5 * time.Second
	publicL4UDPIdleTimeout    = 30 * time.Second
	publicL4UDPMaxSessions    = 4096
	publicL4UDPRemoteTimeout  = 10 * time.Second
	publicL4RetryInterval     = 10 * time.Second
	publicL4DefaultTCPMax     = 512
	publicL4DefaultUDPWorkers = 256
)

type PublicL4Manager struct {
	mu               sync.Mutex
	hub              *TunnelHub
	log              *slog.Logger
	listeners        map[string]*publicL4Listener
	reservations     map[string]*PublicL4Reservation
	tcpSlots         chan struct{}
	agentStreamSlots chan struct{}
	udpRemoteSlots   chan struct{}
}

type PublicL4Reservation struct {
	manager  *PublicL4Manager
	key      string
	listener *publicL4Listener
	done     bool
}

type publicL4Listener struct {
	mu       sync.RWMutex
	resource Resource
	mode     string
	port     int
	closer   io.Closer
	udp      *publicUDPState
}

type publicUDPState struct {
	manager  *PublicL4Manager
	listener *publicL4Listener
	pc       net.PacketConn

	mu       sync.Mutex
	sessions map[string]*publicUDPSession
	closed   bool
}

type publicUDPSession struct {
	key     string
	client  net.Addr
	backend *net.UDPConn
	state   *publicUDPState
	close   sync.Once
}

func NewPublicL4Manager(hub *TunnelHub, logger *slog.Logger, maxTCP, maxAgentStreams, maxRemoteUDP int) *PublicL4Manager {
	if maxTCP < 1 {
		maxTCP = publicL4DefaultTCPMax
	}
	if maxAgentStreams < 1 {
		maxAgentStreams = 16
	}
	if maxRemoteUDP < 1 {
		maxRemoteUDP = publicL4DefaultUDPWorkers
	}
	return &PublicL4Manager{
		hub:              hub,
		log:              logger,
		listeners:        map[string]*publicL4Listener{},
		reservations:     map[string]*PublicL4Reservation{},
		tcpSlots:         make(chan struct{}, maxTCP),
		agentStreamSlots: make(chan struct{}, maxAgentStreams),
		udpRemoteSlots:   make(chan struct{}, maxRemoteUDP),
	}
}

// Reserve mantiene abierto un socket nuevo sin empezar a aceptar trafico. Se
// usa antes de persistir altas/cambios de puerto para eliminar la carrera entre
// "puerto disponible" y el INSERT/UPDATE de SQLite. Si el listener ya existe,
// no hace falta reserva y devuelve nil.
func (m *PublicL4Manager) Reserve(r Resource) (*PublicL4Reservation, error) {
	if m == nil || !r.Enabled || (r.Mode != ModeTCP && r.Mode != ModeUDP) || r.PublicPort <= 0 {
		return nil, nil
	}
	key := publicL4Key(r.Mode, r.PublicPort)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.listeners[key]; ok {
		return nil, nil
	}
	if _, ok := m.reservations[key]; ok {
		return nil, fmt.Errorf("el puerto publico %d/%s esta siendo reservado por otra operacion", r.PublicPort, strings.ToLower(r.Mode))
	}
	listener, err := m.openListener(r)
	if err != nil {
		return nil, err
	}
	reservation := &PublicL4Reservation{manager: m, key: key, listener: listener}
	m.reservations[key] = reservation
	return reservation, nil
}

// Commit publica el socket reservado. No realiza I/O que pueda fallar: el bind
// real ya ocurrio en Reserve. El Resource definitivo se asigna justo antes de
// comenzar a aceptar trafico.
func (r *PublicL4Reservation) Commit(resource Resource) {
	if r == nil || r.manager == nil {
		return
	}
	m := r.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.done {
		return
	}
	current, ok := m.reservations[r.key]
	if !ok || current != r {
		r.done = true
		return
	}
	delete(m.reservations, r.key)
	r.listener.setResource(resource)
	if existing, exists := m.listeners[r.key]; exists {
		// Es defensivo: Sync no adopta reservas pendientes, pero si otro camino
		// ya creo el listener conservamos el activo y liberamos esta reserva.
		_ = r.listener.closer.Close()
		existing.setResource(resource)
	} else {
		m.listeners[r.key] = r.listener
		m.startListener(r.listener)
	}
	r.done = true
}

// Abort libera una reserva que no llego a persistirse. Es idempotente y seguro
// usarlo con defer incluso despues de Commit.
func (r *PublicL4Reservation) Abort() {
	if r == nil || r.manager == nil {
		return
	}
	m := r.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.done {
		return
	}
	if current, ok := m.reservations[r.key]; ok && current == r {
		delete(m.reservations, r.key)
		_ = r.listener.closer.Close()
	}
	r.done = true
}

// Sync reconcilia el conjunto deseado aislando fallos por puerto: un socket
// ocupado no impide activar otros recursos independientes. Las mutaciones CRUD
// que necesitan abrir un puerto nuevo usan Reserve antes de persistir, por lo
// que su cambio individual si mantiene semantica transaccional DB/socket.
func (m *PublicL4Manager) Sync(resources []Resource) error {
	if m == nil {
		return nil
	}
	wanted := make(map[string]Resource)
	for _, r := range resources {
		if !r.Enabled || (r.Mode != ModeTCP && r.Mode != ModeUDP) || r.PublicPort <= 0 {
			continue
		}
		wanted[publicL4Key(r.Mode, r.PublicPort)] = r
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Una mutacion puede haber reservado el socket y estar entre el commit de
	// SQLite y Commit(). El reconciliador no debe cerrar el listener anterior en
	// esa ventana; la siguiente sincronizacion aplicara el conjunto completo.
	for key := range wanted {
		if _, pending := m.reservations[key]; pending {
			return nil
		}
	}

	staged := make(map[string]*publicL4Listener)
	var syncErrors []error
	for key, r := range wanted {
		if _, ok := m.listeners[key]; ok {
			continue
		}
		listener, err := m.openListener(r)
		if err != nil {
			syncErrors = append(syncErrors, err)
			continue
		}
		staged[key] = listener
	}

	// Un backend puede cambiar sin tocar el socket. Las conexiones ya aceptadas
	// conservan la copia de Resource con la que nacieron; las nuevas leen esta.
	for key, r := range wanted {
		if current, ok := m.listeners[key]; ok {
			current.setResource(r)
		}
	}

	for key, listener := range staged {
		m.listeners[key] = listener
		m.startListener(listener)
	}

	for key, listener := range m.listeners {
		if _, ok := wanted[key]; ok {
			continue
		}
		_ = listener.closer.Close()
		delete(m.listeners, key)
		if m.log != nil {
			m.log.Info("listener publico L4 retirado", "mode", strings.ToUpper(listener.mode), "port", listener.port)
		}
	}
	return errors.Join(syncErrors...)
}

func (m *PublicL4Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, listener := range m.listeners {
		_ = listener.closer.Close()
		delete(m.listeners, key)
	}
	for key, reservation := range m.reservations {
		_ = reservation.listener.closer.Close()
		reservation.done = true
		delete(m.reservations, key)
	}
}

func (m *PublicL4Manager) Listening(mode string, port int) bool {
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.listeners[publicL4Key(mode, port)]
	return ok
}

func (m *PublicL4Manager) openListener(r Resource) (*publicL4Listener, error) {
	addr := ":" + strconv.Itoa(r.PublicPort)
	listener := &publicL4Listener{resource: r, mode: r.Mode, port: r.PublicPort}
	switch r.Mode {
	case ModeTCP:
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("abrir listener TCP publico %s para %s: %w", addr, r.Name, err)
		}
		listener.closer = ln
	case ModeUDP:
		pc, err := net.ListenPacket("udp", addr)
		if err != nil {
			return nil, fmt.Errorf("abrir listener UDP publico %s para %s: %w", addr, r.Name, err)
		}
		state := &publicUDPState{manager: m, listener: listener, pc: pc, sessions: map[string]*publicUDPSession{}}
		listener.closer = &udpListenerCloser{state: state}
		listener.udp = state
	default:
		return nil, fmt.Errorf("modo L4 no soportado: %s", r.Mode)
	}
	return listener, nil
}

func (m *PublicL4Manager) startListener(listener *publicL4Listener) {
	if listener == nil {
		return
	}
	if m.log != nil {
		r := listener.getResource()
		m.log.Info("listener publico L4 activo", "resource", r.ID, "mode", strings.ToUpper(listener.mode), "port", listener.port, "origin", r.OriginType, "agent", r.AgentID)
	}
	switch listener.mode {
	case ModeTCP:
		go m.acceptTCP(listener, listener.closer.(net.Listener))
	case ModeUDP:
		go listener.udp.serve()
	}
}

func (l *publicL4Listener) getResource() Resource {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.resource
}

func (l *publicL4Listener) setResource(r Resource) {
	l.mu.Lock()
	l.resource = r
	l.mu.Unlock()
}

func (m *PublicL4Manager) acceptTCP(listener *publicL4Listener, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if !isClosedNetworkError(err) && m.log != nil {
				m.log.Warn("aceptar TCP publico fallo", "port", listener.port, "error", err.Error())
			}
			return
		}
		select {
		case m.tcpSlots <- struct{}{}:
			r := listener.getResource()
			go func() {
				defer func() { <-m.tcpSlots }()
				m.handleTCPConn(r, conn)
			}()
		default:
			_ = conn.Close()
			if m.log != nil {
				m.log.Debug("limite global de conexiones TCP publicas alcanzado", "port", listener.port, "limit", cap(m.tcpSlots))
			}
		}
	}
}

func (m *PublicL4Manager) handleTCPConn(r Resource, client net.Conn) {
	defer client.Close()
	configureTCPStreamConn(client)
	if r.UsesAgent() {
		select {
		case m.agentStreamSlots <- struct{}{}:
			defer func() { <-m.agentStreamSlots }()
		default:
			if m.log != nil {
				m.log.Debug("limite de streams TCP remotos alcanzado", "resource", r.ID, "agent", r.AgentID, "limit", cap(m.agentStreamSlots))
			}
			return
		}
		streamID, err := randomID()
		if err != nil {
			return
		}
		job := AgentStreamJob{ID: streamID, ResourceID: r.ID, Mode: ModeTCP, TargetHost: r.BackendHost, TargetPort: r.BackendPort}
		if err := m.hub.SubmitStream(context.Background(), r.AgentID, job, client); err != nil && m.log != nil {
			m.log.Warn("stream TCP publico remoto fallo", "resource", r.ID, "agent", r.AgentID, "error", err.Error())
		}
		return
	}

	dialer := net.Dialer{Timeout: publicL4DialTimeout, KeepAlive: streamTCPKeepAlivePeriod}
	backend, err := dialer.Dial("tcp", net.JoinHostPort(r.BackendHost, strconv.Itoa(r.BackendPort)))
	if err != nil {
		if m.log != nil {
			m.log.Warn("conexion TCP a backend fallo", "resource", r.ID, "backend", net.JoinHostPort(r.BackendHost, strconv.Itoa(r.BackendPort)), "error", err.Error())
		}
		return
	}
	defer backend.Close()
	configureTCPStreamConn(backend)
	if err := proxyTCPDuplex(client, backend); err != nil && m.log != nil {
		m.log.Debug("conexion TCP finalizada con error", "resource", r.ID, "error", err.Error())
	}
}

func proxyTCPDuplex(a, b net.Conn) error {
	type copyResult struct{ err error }
	done := make(chan copyResult, 2)
	copyOne := func(dst, src net.Conn) {
		buf := make([]byte, streamBufferSize)
		_, err := io.CopyBuffer(dst, src, buf)
		closeWrite(dst)
		closeRead(src)
		if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
			err = nil
		}
		done <- copyResult{err: err}
	}
	go copyOne(a, b)
	go copyOne(b, a)
	first := <-done
	if first.err != nil {
		// Un error real en una direccion puede dejar la copia opuesta bloqueada
		// indefinidamente. El cierre normal usa half-close; ante error forzamos
		// ambos extremos para liberar goroutines y slots de concurrencia.
		_ = a.Close()
		_ = b.Close()
	}
	second := <-done
	if first.err != nil {
		return first.err
	}
	return second.err
}

func closeWrite(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.CloseWrite()
	}
}

func closeRead(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.CloseRead()
	}
}

func (s *publicUDPState) serve() {
	buf := make([]byte, remoteUDPPacketMaxSize)
	for {
		n, client, err := s.pc.ReadFrom(buf)
		if err != nil {
			if !isClosedNetworkError(err) && s.manager.log != nil {
				s.manager.log.Warn("leer UDP publico fallo", "port", s.listener.port, "error", err.Error())
			}
			return
		}
		packet := append([]byte(nil), buf[:n]...)
		r := s.listener.getResource()
		if r.UsesAgent() {
			s.forwardRemote(r, client, packet)
			continue
		}
		s.forwardLocal(r, client, packet)
	}
}

func (s *publicUDPState) forwardRemote(r Resource, client net.Addr, packet []byte) {
	client = cloneNetAddr(client)
	select {
	case s.manager.udpRemoteSlots <- struct{}{}:
		go func() {
			defer func() { <-s.manager.udpRemoteSlots }()
			jobID, err := randomID()
			if err != nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), publicL4UDPRemoteTimeout)
			defer cancel()
			resp, err := s.manager.hub.Submit(ctx, r.AgentID, AgentJob{ID: jobID, Kind: ModeUDP, ResourceID: r.ID, Body: packet, TargetHost: r.BackendHost, TargetPort: r.BackendPort})
			if err != nil {
				if s.manager.log != nil {
					s.manager.log.Debug("datagrama UDP publico remoto fallo", "resource", r.ID, "agent", r.AgentID, "error", err.Error())
				}
				return
			}
			if len(resp.Body) > 0 {
				_, _ = s.pc.WriteTo(resp.Body, client)
			}
		}()
	default:
		// UDP no tiene backpressure fiable. Al alcanzar el limite se descarta el
		// datagrama antes que permitir crecimiento ilimitado de goroutines/memoria.
	}
}

func (s *publicUDPState) forwardLocal(r Resource, client net.Addr, packet []byte) {
	key := client.String()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	session := s.sessions[key]
	if session == nil {
		if len(s.sessions) >= publicL4UDPMaxSessions {
			s.mu.Unlock()
			return
		}
		backendAddr, err := net.ResolveUDPAddr("udp", net.JoinHostPort(r.BackendHost, strconv.Itoa(r.BackendPort)))
		if err != nil {
			s.mu.Unlock()
			return
		}
		backend, err := net.DialUDP("udp", nil, backendAddr)
		if err != nil {
			s.mu.Unlock()
			if s.manager.log != nil {
				s.manager.log.Debug("abrir sesion UDP a backend fallo", "resource", r.ID, "backend", backendAddr.String(), "error", err.Error())
			}
			return
		}
		session = &publicUDPSession{key: key, client: cloneNetAddr(client), backend: backend, state: s}
		s.sessions[key] = session
		go session.readReplies()
	}
	backend := session.backend
	s.mu.Unlock()

	_ = backend.SetReadDeadline(time.Now().Add(publicL4UDPIdleTimeout))
	if _, err := backend.Write(packet); err != nil {
		session.shutdown()
	}
}

func (s *publicUDPSession) readReplies() {
	buf := make([]byte, remoteUDPPacketMaxSize)
	for {
		n, err := s.backend.Read(buf)
		if err != nil {
			s.shutdown()
			return
		}
		if n > 0 {
			_ = s.backend.SetReadDeadline(time.Now().Add(publicL4UDPIdleTimeout))
			if _, err := s.state.pc.WriteTo(buf[:n], s.client); err != nil {
				s.shutdown()
				return
			}
		}
	}
}

func (s *publicUDPSession) shutdown() {
	s.close.Do(func() {
		_ = s.backend.Close()
		s.state.mu.Lock()
		if current := s.state.sessions[s.key]; current == s {
			delete(s.state.sessions, s.key)
		}
		s.state.mu.Unlock()
	})
}

type udpListenerCloser struct{ state *publicUDPState }

func (c *udpListenerCloser) Close() error {
	if c == nil || c.state == nil {
		return nil
	}
	c.state.mu.Lock()
	if c.state.closed {
		c.state.mu.Unlock()
		return nil
	}
	c.state.closed = true
	sessions := make([]*publicUDPSession, 0, len(c.state.sessions))
	for _, session := range c.state.sessions {
		sessions = append(sessions, session)
	}
	c.state.mu.Unlock()
	for _, session := range sessions {
		session.shutdown()
	}
	return c.state.pc.Close()
}

func publicL4Key(mode string, port int) string {
	return strings.ToLower(strings.TrimSpace(mode)) + ":" + strconv.Itoa(port)
}

func cloneNetAddr(addr net.Addr) net.Addr {
	switch a := addr.(type) {
	case *net.UDPAddr:
		ip := append(net.IP(nil), a.IP...)
		return &net.UDPAddr{IP: ip, Port: a.Port, Zone: a.Zone}
	default:
		return addr
	}
}
