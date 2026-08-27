package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestPublicL4TCPBackendUpdateKeepsExistingConnection(t *testing.T) {
	backendA, portA := startTaggedTCPBackend(t, "A")
	defer backendA.Close()
	backendB, portB := startTaggedTCPBackend(t, "B")
	defer backendB.Close()

	publicPort := freeTCPPort(t)
	manager := NewPublicL4Manager(NewTunnelHub(8), nil, 32, 8, 32)
	defer manager.Close()

	resource := Resource{ID: "tcp-hot", Name: "TCP hot", Mode: ModeTCP, PublicPort: publicPort, BackendHost: "127.0.0.1", BackendPort: portA, OriginType: OriginLocal, Enabled: true}
	if err := manager.Sync([]Resource{resource}); err != nil {
		t.Fatal(err)
	}

	existing := dialTCP(t, publicPort)
	defer existing.Close()
	assertTaggedTCPRoundTrip(t, existing, "uno", "A:uno")

	resource.BackendPort = portB
	if err := manager.Sync([]Resource{resource}); err != nil {
		t.Fatal(err)
	}
	assertTaggedTCPRoundTrip(t, existing, "dos", "A:dos")

	fresh := dialTCP(t, publicPort)
	defer fresh.Close()
	assertTaggedTCPRoundTrip(t, fresh, "tres", "B:tres")

	if err := manager.Sync(nil); err != nil {
		t.Fatal(err)
	}
	assertTaggedTCPRoundTrip(t, existing, "cuatro", "A:cuatro")
	if _, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(publicPort)), 200*time.Millisecond); err == nil {
		t.Fatal("el listener retirado no debe aceptar conexiones nuevas")
	}
}

func TestPublicL4SyncFailureKeepsExistingListeners(t *testing.T) {
	backend, backendPort := startTaggedTCPBackend(t, "OK")
	defer backend.Close()
	portA := freeTCPPort(t)
	manager := NewPublicL4Manager(NewTunnelHub(8), nil, 32, 8, 32)
	defer manager.Close()

	r1 := Resource{ID: "tcp-one", Name: "uno", Mode: ModeTCP, PublicPort: portA, BackendHost: "127.0.0.1", BackendPort: backendPort, OriginType: OriginLocal, Enabled: true}
	if err := manager.Sync([]Resource{r1}); err != nil {
		t.Fatal(err)
	}

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	portB := occupied.Addr().(*net.TCPAddr).Port
	r2 := Resource{ID: "tcp-two", Name: "dos", Mode: ModeTCP, PublicPort: portB, BackendHost: "127.0.0.1", BackendPort: backendPort, OriginType: OriginLocal, Enabled: true}
	if err := manager.Sync([]Resource{r1, r2}); err == nil {
		t.Fatal("se esperaba error al reservar el segundo puerto")
	}

	conn := dialTCP(t, portA)
	defer conn.Close()
	assertTaggedTCPRoundTrip(t, conn, "sigue", "OK:sigue")
}

func TestPublicL4UDPProxyLocal(t *testing.T) {
	backend, backendPort := startTaggedUDPBackend(t, "UDP")
	defer backend.Close()
	publicPort := freeUDPPort(t)
	manager := NewPublicL4Manager(NewTunnelHub(8), nil, 32, 8, 32)
	defer manager.Close()
	resource := Resource{ID: "udp-local", Name: "udp", Mode: ModeUDP, PublicPort: publicPort, BackendHost: "127.0.0.1", BackendPort: backendPort, OriginType: OriginLocal, Enabled: true}
	if err := manager.Sync([]Resource{resource}); err != nil {
		t.Fatal(err)
	}

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: publicPort})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("hola")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(buf[:n]); got != "UDP:hola" {
		t.Fatalf("respuesta UDP=%q", got)
	}
}

func TestPublicL4TCPRemoteUsesTunnelHubDirectly(t *testing.T) {
	hub := NewTunnelHub(8)
	hub.streamAttachTimeout = 2 * time.Second
	publicPort := freeTCPPort(t)
	manager := NewPublicL4Manager(hub, nil, 32, 8, 32)
	defer manager.Close()
	resource := Resource{ID: "tcp-remote", Name: "ssh", Mode: ModeTCP, PublicPort: publicPort, BackendHost: "127.0.0.1", BackendPort: 22, OriginType: OriginAgent, AgentID: "agent01", Enabled: true}
	if err := manager.Sync([]Resource{resource}); err != nil {
		t.Fatal(err)
	}

	agentDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		job, ok, err := hub.PollStream(ctx, "agent01")
		if err != nil || !ok {
			agentDone <- fmt.Errorf("poll stream: %v", err)
			return
		}
		if job.ResourceID != resource.ID || job.TargetPort != 22 {
			agentDone <- fmt.Errorf("job remoto inesperado: %#v", job)
			return
		}
		sess, ok := hub.AttachStream(job.ID, "agent01")
		if !ok {
			agentDone <- fmt.Errorf("no se pudo adjuntar stream")
			return
		}
		buf := make([]byte, 4)
		if _, err := io.ReadFull(sess.ClientConn, buf); err != nil {
			agentDone <- err
			return
		}
		if string(buf) != "ping" {
			agentDone <- fmt.Errorf("payload=%q", buf)
			return
		}
		if _, err := sess.ClientConn.Write([]byte("pong")); err != nil {
			agentDone <- err
			return
		}
		hub.CompleteStream(job.ID)
		agentDone <- nil
	}()

	conn := dialTCP(t, publicPort)
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "pong" {
		t.Fatalf("respuesta remota=%q", buf)
	}
	if err := <-agentDone; err != nil {
		t.Fatal(err)
	}
}

func TestPublicL4UDPRemoteUsesTunnelHubDirectly(t *testing.T) {
	hub := NewTunnelHub(8)
	publicPort := freeUDPPort(t)
	manager := NewPublicL4Manager(hub, nil, 32, 8, 32)
	defer manager.Close()
	resource := Resource{ID: "udp-remote", Name: "dns", Mode: ModeUDP, PublicPort: publicPort, BackendHost: "127.0.0.1", BackendPort: 53, OriginType: OriginAgent, AgentID: "agent01", Enabled: true}
	if err := manager.Sync([]Resource{resource}); err != nil {
		t.Fatal(err)
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		job, ok, err := hub.Poll(ctx, "agent01")
		if err != nil || !ok {
			return
		}
		hub.Complete(job.ID, AgentResponse{JobID: job.ID, StatusCode: 200, Body: append([]byte("R:"), job.Body...)})
	}()

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: publicPort})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(buf[:n]); got != "R:q" {
		t.Fatalf("respuesta UDP remota=%q", got)
	}
}

func startTaggedTCPBackend(t *testing.T, tag string) (net.Listener, int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 128)
				for {
					n, err := c.Read(buf)
					if n > 0 {
						_, _ = c.Write(append([]byte(tag+":"), buf[:n]...))
					}
					if err != nil {
						return
					}
				}
			}(conn)
		}
	}()
	return ln, ln.Addr().(*net.TCPAddr).Port
}

func startTaggedUDPBackend(t *testing.T, tag string) (net.PacketConn, int) {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		buf := make([]byte, 65535)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			payload := append([]byte(tag+":"), buf[:n]...)
			_, _ = pc.WriteTo(payload, addr)
		}
	}()
	return pc, pc.LocalAddr().(*net.UDPAddr).Port
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func freeUDPPort(t *testing.T) int {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	_ = pc.Close()
	return port
}

func dialTCP(t *testing.T, port int) net.Conn {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
	return conn
}

func assertTaggedTCPRoundTrip(t *testing.T, conn net.Conn, payload, want string) {
	t.Helper()
	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, len(want))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatal(err)
	}
	if got := string(buf); got != want {
		t.Fatalf("respuesta=%q want=%q", got, want)
	}
}

func TestPublicL4ReservationHoldsPortUntilCommitOrAbort(t *testing.T) {
	m := NewPublicL4Manager(NewTunnelHub(8), slog.New(slog.NewTextHandler(io.Discard, nil)), 16, 16, 16)
	defer m.Close()

	port := freeTCPPort(t)
	resource := Resource{ID: "reserve", Name: "reserve", Mode: ModeTCP, PublicPort: port, Enabled: true, BackendHost: "127.0.0.1", BackendPort: 9}
	reservation, err := m.Reserve(resource)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if reservation == nil {
		t.Fatal("se esperaba una reserva nueva")
	}
	if m.Listening(ModeTCP, port) {
		t.Fatal("la reserva no debe aceptar trafico antes de Commit")
	}
	if ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port))); err == nil {
		_ = ln.Close()
		t.Fatal("la reserva debe mantener el puerto ocupado")
	}

	reservation.Commit(resource)
	if !m.Listening(ModeTCP, port) {
		t.Fatal("Commit debe publicar el listener reservado")
	}
	reservation.Abort() // idempotente despues de Commit.

	port2 := freeTCPPort(t)
	resource.PublicPort = port2
	reservation2, err := m.Reserve(resource)
	if err != nil {
		t.Fatalf("reserve 2: %v", err)
	}
	reservation2.Abort()
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port2)))
	if err != nil {
		t.Fatalf("Abort debe liberar el puerto: %v", err)
	}
	_ = ln.Close()
}

func TestPublicL4SyncIsolatesPortFailures(t *testing.T) {
	manager := NewPublicL4Manager(NewTunnelHub(8), nil, 16, 16, 16)
	defer manager.Close()

	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	occupiedPort := occupied.Addr().(*net.TCPAddr).Port
	freePort := freeTCPPort(t)

	resources := []Resource{
		{ID: "blocked", Name: "blocked", Mode: ModeTCP, PublicPort: occupiedPort, Enabled: true, BackendHost: "127.0.0.1", BackendPort: 9},
		{ID: "healthy", Name: "healthy", Mode: ModeTCP, PublicPort: freePort, Enabled: true, BackendHost: "127.0.0.1", BackendPort: 9},
	}
	if err := manager.Sync(resources); err == nil {
		t.Fatal("se esperaba error por el puerto ocupado")
	}
	if !manager.Listening(ModeTCP, freePort) {
		t.Fatal("un puerto ocupado no debe impedir activar listeners independientes")
	}
	if manager.Listening(ModeTCP, occupiedPort) {
		t.Fatal("el puerto ocupado no debe marcarse como listener activo")
	}
}
