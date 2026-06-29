package app

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

const smtpDialTimeout = 8 * time.Second

type mailMessage struct {
	ToEmail string
	Subject string
	Text    string
}

func validateSMTPConnectivity(settings AppSettings) error {
	settings.Normalize()
	if err := settings.ValidateSMTP(true); err != nil {
		return err
	}
	client, conn, err := smtpClient(settings)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer client.Close()
	if strings.TrimSpace(settings.SMTPUsername) != "" {
		if err := client.Auth(smtp.PlainAuth("", settings.SMTPUsername, settings.SMTPPassword, settings.SMTPHost)); err != nil {
			return fmt.Errorf("autenticacion SMTP fallida: %w", err)
		}
	}
	return nil
}

func sendSMTPMail(settings AppSettings, msg mailMessage) error {
	settings.Normalize()
	if err := settings.ValidateSMTP(true); err != nil {
		return err
	}
	if err := ValidateEmailAddress(msg.ToEmail); err != nil {
		return err
	}
	if strings.TrimSpace(msg.Subject) == "" || strings.TrimSpace(msg.Text) == "" {
		return fmt.Errorf("mensaje SMTP incompleto")
	}
	client, conn, err := smtpClient(settings)
	if err != nil {
		return err
	}
	defer conn.Close()
	defer client.Close()
	if strings.TrimSpace(settings.SMTPUsername) != "" {
		if err := client.Auth(smtp.PlainAuth("", settings.SMTPUsername, settings.SMTPPassword, settings.SMTPHost)); err != nil {
			return fmt.Errorf("autenticacion SMTP fallida: %w", err)
		}
	}
	from := mail.Address{Name: settings.SMTPFromName, Address: settings.SMTPFromEmail}
	to := mail.Address{Address: strings.ToLower(strings.TrimSpace(msg.ToEmail))}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("remitente SMTP rechazado: %w", err)
	}
	if err := client.Rcpt(to.Address); err != nil {
		return fmt.Errorf("destinatario SMTP rechazado: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("abrir cuerpo SMTP: %w", err)
	}
	_, err = wc.Write(buildRFC822Message(from, to, msg.Subject, msg.Text))
	closeErr := wc.Close()
	if err != nil {
		return fmt.Errorf("enviar cuerpo SMTP: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("finalizar correo SMTP: %w", closeErr)
	}
	return client.Quit()
}

func smtpClient(settings AppSettings) (*smtp.Client, net.Conn, error) {
	addr := net.JoinHostPort(settings.SMTPHost, fmt.Sprint(settings.SMTPPort))
	dialer := &net.Dialer{Timeout: smtpDialTimeout}
	var conn net.Conn
	var err error
	if settings.SMTPSecurity == SMTPSecurityTLS {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: settings.SMTPHost, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("conectar SMTP: %w", err)
	}
	client, err := smtp.NewClient(conn, settings.SMTPHost)
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("iniciar SMTP: %w", err)
	}
	if settings.SMTPSecurity == SMTPSecurityStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			_ = conn.Close()
			return nil, nil, fmt.Errorf("el servidor SMTP no anuncia STARTTLS")
		}
		if err := client.StartTLS(&tls.Config{ServerName: settings.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
			_ = client.Close()
			_ = conn.Close()
			return nil, nil, fmt.Errorf("activar STARTTLS: %w", err)
		}
	}
	return client, conn, nil
}

func buildRFC822Message(from, to mail.Address, subject, text string) []byte {
	var b bytes.Buffer
	headers := map[string]string{
		"From":         from.String(),
		"To":           to.String(),
		"Subject":      mime.QEncoding.Encode("utf-8", strings.TrimSpace(subject)),
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=utf-8",
	}
	for _, k := range []string{"From", "To", "Subject", "MIME-Version", "Content-Type"} {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(headers[k])
		b.WriteString("\r\n")
	}
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(text, "\n", "\r\n"))
	b.WriteString("\r\n")
	return b.Bytes()
}
