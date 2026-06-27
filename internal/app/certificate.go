package app

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CertificateStatus struct {
	Domain     string    `json:"domain"`
	SSLEnabled bool      `json:"sslEnabled"`
	Issued     bool      `json:"issued"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	Source     string    `json:"source,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt,omitempty"`
}

type traefikACMEStorage map[string]struct {
	Certificates []traefikACMECertificate `json:"Certificates"`
}

type traefikACMECertificate struct {
	Domain struct {
		Main string   `json:"main"`
		SANs []string `json:"sans"`
	} `json:"domain"`
	Certificate string `json:"certificate"`
}

func ResolveCertificateStatus(c Config, domain string, sslEnabled bool) CertificateStatus {
	domain = strings.ToLower(strings.TrimSpace(domain))
	status := CertificateStatus{Domain: domain, SSLEnabled: sslEnabled}
	if domain == "" {
		status.Status = "missing_domain"
		status.Message = "Sin dominio para revisar SSL."
		return status
	}
	if !sslEnabled {
		status.Status = "disabled"
		status.Message = "SSL desactivado para este dominio. Si ya existe un certificado ACME, se conserva en Traefik."
		return status
	}
	if !ACMEEnabled(c) {
		status.Status = "acme_disabled"
		status.Message = "SSL solicitado, pero falta configurar un correo ACME real en Ajustes."
		return status
	}

	path := filepath.Join(c.TraefikDir, "acme.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		status.Status = "pending"
		status.Message = "SSL configurado. Traefik generará el certificado al recibir tráfico válido por HTTP/HTTPS."
		return status
	}
	if err != nil {
		status.Status = "unavailable"
		status.Message = fmt.Sprintf("No se pudo leer el almacenamiento ACME: %v", err)
		return status
	}
	if len(strings.TrimSpace(string(data))) == 0 || strings.TrimSpace(string(data)) == "{}" {
		status.Status = "pending"
		status.Message = "SSL configurado. El certificado aún no aparece en el almacenamiento ACME."
		return status
	}

	var storage traefikACMEStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		status.Status = "unavailable"
		status.Message = fmt.Sprintf("El almacenamiento ACME no se pudo interpretar: %v", err)
		return status
	}
	for resolver, bucket := range storage {
		for _, cert := range bucket.Certificates {
			if !certificateCoversDomain(cert, domain) {
				continue
			}
			status.Status = "issued"
			status.Issued = true
			status.Source = resolver
			status.Message = "Certificado SSL generado y encontrado en Traefik."
			if expires := certificateExpiry(cert.Certificate); !expires.IsZero() {
				status.ExpiresAt = expires
			}
			return status
		}
	}
	status.Status = "pending"
	status.Message = "SSL configurado. El certificado todavía no está generado o está en proceso."
	return status
}

func certificateCoversDomain(cert traefikACMECertificate, domain string) bool {
	if domainMatchesCertificateName(domain, cert.Domain.Main) {
		return true
	}
	for _, san := range cert.Domain.SANs {
		if domainMatchesCertificateName(domain, san) {
			return true
		}
	}
	return false
}

func domainMatchesCertificateName(domain, certName string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	certName = strings.ToLower(strings.TrimSpace(certName))
	if domain == "" || certName == "" {
		return false
	}
	if domain == certName {
		return true
	}
	if strings.HasPrefix(certName, "*.") {
		base := strings.TrimPrefix(certName, "*.")
		if strings.HasSuffix(domain, "."+base) {
			left := strings.TrimSuffix(domain, "."+base)
			return left != "" && !strings.Contains(left, ".")
		}
	}
	return false
}

func certificateExpiry(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	candidates := [][]byte{[]byte(raw)}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		candidates = append(candidates, decoded)
	}
	for _, candidate := range candidates {
		for {
			block, rest := pem.Decode(candidate)
			if block == nil {
				break
			}
			if block.Type == "CERTIFICATE" {
				if parsed, err := x509.ParseCertificate(block.Bytes); err == nil {
					return parsed.NotAfter.UTC()
				}
			}
			candidate = rest
		}
	}
	return time.Time{}
}
