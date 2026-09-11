// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudemail

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// dialTimeout bounds the connection so an unreachable relay cannot stall the process
const dialTimeout = 30 * time.Second

// Send delivers a plain-text message. Recipients are validated as addresses before
// anything is sent. Addresses may carry a display name, as in "OpsBlade <ops@example.com>";
// the bare address is used on the SMTP envelope and the full form in the headers.
func (e *CloudEmail) Send(from string, to []string, subject, body string) error {
	if len(to) == 0 {
		return fmt.Errorf("no recipients")
	}
	fromAddr, err := mail.ParseAddress(from)
	if err != nil {
		return fmt.Errorf("invalid from address %q: %w", from, err)
	}
	toAddrs := make([]*mail.Address, 0, len(to))
	for _, addr := range to {
		parsed, err := mail.ParseAddress(addr)
		if err != nil {
			return fmt.Errorf("invalid recipient address %q: %w", addr, err)
		}
		toAddrs = append(toAddrs, parsed)
	}

	client, err := e.connect()
	if err != nil {
		return err
	}
	defer func(c *smtp.Client) {
		_ = c.Close()
	}(client)

	if e.Config.User != "" {
		auth := smtp.PlainAuth("", e.Config.User, e.Config.Password, e.Config.Host)
		if err = client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	// The envelope carries bare addresses; the headers keep any display name
	if err = client.Mail(fromAddr.Address); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}
	headerTo := make([]string, 0, len(toAddrs))
	for _, addr := range toAddrs {
		if err = client.Rcpt(addr.Address); err != nil {
			return fmt.Errorf("SMTP RCPT TO %s failed: %w", addr.Address, err)
		}
		headerTo = append(headerTo, addr.String())
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}
	if _, err = w.Write([]byte(Message(fromAddr.String(), headerTo, subject, body))); err != nil {
		return fmt.Errorf("SMTP write failed: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("SMTP DATA close failed: %w", err)
	}
	return client.Quit()
}

// connect opens the SMTP session, using implicit TLS on port 465 and STARTTLS
// elsewhere when the server offers it
func (e *CloudEmail) connect() (*smtp.Client, error) {
	addr := net.JoinHostPort(e.Config.Host, fmt.Sprint(e.Config.Port))
	tlsConfig := &tls.Config{ServerName: e.Config.Host, MinVersion: tls.VersionTLS12}

	conn, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("SMTP connect to %s failed: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(dialTimeout))

	if e.Config.Port == implicitTLSPort {
		conn = tls.Client(conn, tlsConfig)
	}

	client, err := smtp.NewClient(conn, e.Config.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("SMTP handshake with %s failed: %w", addr, err)
	}

	if e.Config.Port != implicitTLSPort {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err = client.StartTLS(tlsConfig); err != nil {
				_ = client.Close()
				return nil, fmt.Errorf("SMTP STARTTLS failed: %w", err)
			}
		}
	}
	return client, nil
}

// Message renders the RFC 5322 message. Header values have line breaks removed so a
// caller-supplied subject cannot inject headers.
func Message(from string, to []string, subject, body string) string {
	clean := func(s string) string {
		return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
	}
	var b strings.Builder
	b.WriteString("From: " + clean(from) + "\r\n")
	b.WriteString("To: " + clean(strings.Join(to, ", ")) + "\r\n")
	b.WriteString("Subject: " + clean(subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n"))
	if !strings.HasSuffix(body, "\n") {
		b.WriteString("\r\n")
	}
	return b.String()
}
