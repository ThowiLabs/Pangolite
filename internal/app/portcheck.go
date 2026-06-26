package app

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

func TCPPortAvailable(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("puerto publico %d fuera de rango", port)
	}
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("el puerto TCP publico %d ya esta en uso en este servidor o no se puede abrir: %w", port, err)
	}
	return ln.Close()
}

func UDPPortAvailable(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("puerto publico %d fuera de rango", port)
	}
	pc, err := net.ListenPacket("udp", ":"+strconv.Itoa(port))
	if err != nil {
		return fmt.Errorf("el puerto UDP publico %d ya esta en uso en este servidor o no se puede abrir: %w", port, err)
	}
	return pc.Close()
}

func ListenPortFromAddr(addr string) int {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return 0
	}
	_, portText, err := net.SplitHostPort(addr)
	if err != nil {
		if i := strings.LastIndex(addr, ":"); i >= 0 && i+1 < len(addr) {
			portText = addr[i+1:]
		}
	}
	port, _ := strconv.Atoi(portText)
	return port
}
