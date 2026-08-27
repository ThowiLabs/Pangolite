package app

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateWindow struct {
	Count   int
	ResetAt time.Time
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	entries map[string]rateWindow
}

func newFixedWindowLimiter() *fixedWindowLimiter {
	return &fixedWindowLimiter{entries: make(map[string]rateWindow)}
}

func (l *fixedWindowLimiter) Allow(key string, limit int, window time.Duration) (time.Time, bool) {
	if l == nil || limit <= 0 || window <= 0 {
		return time.Time{}, true
	}
	now := time.Now().UTC()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) > 4096 {
		for k, v := range l.entries {
			if !v.ResetAt.After(now) {
				delete(l.entries, k)
			}
		}
	}
	state, exists := l.entries[key]
	if !exists && len(l.entries) >= 8192 {
		return now.Add(window), false
	}
	if state.ResetAt.IsZero() || !state.ResetAt.After(now) {
		state = rateWindow{ResetAt: now.Add(window)}
	}
	if state.Count >= limit {
		l.entries[key] = state
		return state.ResetAt, false
	}
	state.Count++
	l.entries[key] = state
	return state.ResetAt, true
}

func parseCIDRs(raw string) []*net.IPNet {
	var out []*net.IPNet
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if ip := net.ParseIP(part); ip != nil {
			if ip.To4() != nil {
				part += "/32"
			} else {
				part += "/128"
			}
		}
		if _, network, err := net.ParseCIDR(part); err == nil {
			out = append(out, network)
		}
	}
	return out
}

func ipInNetworks(ip net.IP, networks []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, network := range networks {
		if network != nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func remoteIP(r *http.Request) net.IP {
	host := strings.TrimSpace(r.RemoteAddr)
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		host = parsed
	}
	return net.ParseIP(strings.Trim(host, "[]"))
}

func (s *Server) clientIP(r *http.Request) string {
	peer := remoteIP(r)
	if peer == nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	if ipInNetworks(peer, s.trustedProxyNetworks) {
		if raw := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); raw != "" {
			values := strings.Split(raw, ",")
			for i := len(values) - 1; i >= 0; i-- {
				ip := net.ParseIP(strings.TrimSpace(values[i]))
				if ip != nil && !ipInNetworks(ip, s.trustedProxyNetworks) {
					return ip.String()
				}
			}
		}
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	return peer.String()
}

func adminNetworkForIP(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		return (&net.IPNet{IP: v4.Mask(net.CIDRMask(24, 32)), Mask: net.CIDRMask(24, 32)}).String()
	}
	return (&net.IPNet{IP: ip.Mask(net.CIDRMask(64, 128)), Mask: net.CIDRMask(64, 128)}).String()
}

func (s *Server) adminIPAllowed(raw string) bool {
	mode := strings.ToLower(strings.TrimSpace(s.config.AdminAccessMode))
	if mode == "" || mode == "off" {
		return true
	}
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return false
	}
	if ipInNetworks(ip, s.adminAllowedNetworks) {
		return true
	}
	s.adminNetworksMu.RLock()
	learned := s.learnedAdminNetworks
	allowed := ipInNetworks(ip, learned)
	hasLearned := len(learned) > 0
	s.adminNetworksMu.RUnlock()
	if !hasLearned && mode == "learn" {
		return true
	}
	return allowed
}

func (s *Server) adminLoginIPAllowed(raw string) bool {
	mode := strings.ToLower(strings.TrimSpace(s.config.AdminAccessMode))
	if mode == "" || mode == "off" {
		return true
	}
	if net.ParseIP(strings.TrimSpace(raw)) == nil {
		return false
	}
	if mode == "learn" {
		return true
	}
	return s.adminIPAllowed(raw)
}

func (s *Server) sessionIPBindingEnabled() bool {
	mode := strings.ToLower(strings.TrimSpace(s.config.AdminAccessMode))
	return mode == "learn" || mode == "allowlist"
}

func (s *Server) adminSessionIPAllowed(raw string) bool {
	mode := strings.ToLower(strings.TrimSpace(s.config.AdminAccessMode))
	if mode == "" || mode == "off" || mode == "learn" {
		return true
	}
	return s.adminIPAllowed(raw)
}

func sameClientIP(expected, actual string) bool {
	expectedIP := net.ParseIP(strings.TrimSpace(expected))
	actualIP := net.ParseIP(strings.TrimSpace(actual))
	if expectedIP == nil || actualIP == nil {
		return false
	}
	return expectedIP.Equal(actualIP)
}

func (s *Server) rememberAdminIP(raw, username string) {
	if strings.ToLower(strings.TrimSpace(s.config.AdminAccessMode)) != "learn" {
		return
	}
	network := adminNetworkForIP(raw)
	if network == "" {
		return
	}
	if err := s.store.AddTrustedAdminNetwork(network, username); err != nil {
		if s.log != nil {
			s.log.Warn("no se pudo recordar red administrativa", "network", network, "error", err.Error())
		}
		return
	}
	learned := parseCIDRs(strings.Join(s.store.ListTrustedAdminNetworks(), ","))
	s.adminNetworksMu.Lock()
	s.learnedAdminNetworks = learned
	s.adminNetworksMu.Unlock()
}
