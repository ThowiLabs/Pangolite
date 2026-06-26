package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type NetworkInfo struct {
	PublicIP           string   `json:"publicIp"`
	InterfaceIPs       []string `json:"interfaceIps"`
	DashboardDomain    string   `json:"dashboardDomain,omitempty"`
	DashboardDomainIPs []string `json:"dashboardDomainIps,omitempty"`
	DNSMatchesServer   bool     `json:"dnsMatchesServer"`
	Message            string   `json:"message"`
}

func DetectNetworkInfo(publicIPOverride, dashboardDomain string) NetworkInfo {
	info := NetworkInfo{
		PublicIP:        strings.TrimSpace(publicIPOverride),
		InterfaceIPs:    localInterfaceIPs(),
		DashboardDomain: strings.ToLower(strings.TrimSpace(dashboardDomain)),
	}
	if info.PublicIP == "" {
		if ip, err := fetchPublicIP(context.Background()); err == nil {
			info.PublicIP = ip
		}
	}
	if info.DashboardDomain != "" {
		ips, err := lookupDomainIPs(info.DashboardDomain)
		if err == nil {
			info.DashboardDomainIPs = ips
			info.DNSMatchesServer = dnsMatches(info)
		}
	}
	if info.PublicIP == "" {
		info.Message = "no se pudo detectar IP publica; define PANGOLITE_PUBLIC_IP"
	} else if info.DashboardDomain != "" && !info.DNSMatchesServer {
		info.Message = "el dominio del panel no apunta a la IP detectada del servidor"
	} else {
		info.Message = "ok"
	}
	return info
}

func ValidateDashboardDomainDNS(domain, publicIPOverride string) (NetworkInfo, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return DetectNetworkInfo(publicIPOverride, ""), nil
	}
	info := DetectNetworkInfo(publicIPOverride, domain)
	if info.PublicIP == "" {
		return info, errors.New("no se pudo detectar la IP publica del servidor; configura PANGOLITE_PUBLIC_IP en /opt/pangolite/pangolite.env")
	}
	if len(info.DashboardDomainIPs) == 0 {
		return info, fmt.Errorf("el dominio %s no tiene registros A/AAAA visibles", domain)
	}
	if !info.DNSMatchesServer {
		return info, fmt.Errorf("el dominio %s resuelve a %s, pero la IP detectada del servidor es %s", domain, strings.Join(info.DashboardDomainIPs, ", "), info.PublicIP)
	}
	return info, nil
}

func dnsMatches(info NetworkInfo) bool {
	candidates := map[string]bool{}
	if info.PublicIP != "" {
		candidates[info.PublicIP] = true
	}
	for _, ip := range info.InterfaceIPs {
		candidates[ip] = true
	}
	for _, ip := range info.DashboardDomainIPs {
		if candidates[ip] {
			return true
		}
	}
	return false
}

func localInterfaceIPs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	out := []string{}
	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		text := ip.String()
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	return out
}

func lookupDomainIPs(domain string) ([]string, error) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	out := []string{}
	for _, ip := range ips {
		text := ip.String()
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	return out, nil
}

func fetchPublicIP(parent context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range []string{"https://api.ipify.org", "https://ifconfig.me/ip", "https://icanhazip.com"} {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}
		res, err := client.Do(req)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(io.LimitReader(res.Body, 128))
		_ = res.Body.Close()
		if err != nil || res.StatusCode < 200 || res.StatusCode > 299 {
			continue
		}
		ip := strings.TrimSpace(string(body))
		if parsed := net.ParseIP(ip); parsed != nil {
			return parsed.String(), nil
		}
	}
	return "", errors.New("no se pudo detectar IP publica")
}
