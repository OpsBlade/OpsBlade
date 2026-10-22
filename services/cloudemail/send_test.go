// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudemail

import (
	"bufio"
	"net"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSMTP is a minimal SMTP server. Each reply field defaults to the success
// code for that command; a test overrides one via the setup function passed to
// newFakeSMTP to exercise an error path.
type fakeSMTP struct {
	ln       net.Listener
	greeting string
	starttls bool // advertise STARTTLS and then refuse it
	auth     string
	mail     string
	rcpt     string
	data     string // reply to the DATA command
	dataEnd  string // reply after the message body

	mu     sync.Mutex
	authed bool
	from   string
	rcpts  []string
	body   string
}

func newFakeSMTP(t *testing.T, setup func(f *fakeSMTP)) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f := &fakeSMTP{
		ln:       ln,
		greeting: "220 fake ESMTP",
		auth:     "235 ok",
		mail:     "250 ok",
		rcpt:     "250 ok",
		data:     "354 go ahead",
		dataEnd:  "250 queued",
	}
	if setup != nil {
		setup(f)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn)
		}
	}()
	return f
}

func (f *fakeSMTP) client() *CloudEmail {
	_, port, _ := net.SplitHostPort(f.ln.Addr().String())
	var p int
	for _, c := range port {
		p = p*10 + int(c-'0')
	}
	return &CloudEmail{Config: EmailConfig{Host: "127.0.0.1", Port: p, User: "u", Password: "p"}}
}

func (f *fakeSMTP) serve(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	reply := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
	reply(f.greeting)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			if f.starttls {
				reply("250-fake")
				reply("250 STARTTLS")
			} else {
				reply("250 fake")
			}
		case strings.HasPrefix(upper, "STARTTLS"):
			reply("454 TLS not available")
		case strings.HasPrefix(upper, "AUTH"):
			f.mu.Lock()
			f.authed = true
			f.mu.Unlock()
			reply(f.auth)
		case strings.HasPrefix(upper, "MAIL FROM:"):
			f.mu.Lock()
			f.from = strings.Trim(line[len("MAIL FROM:"):], "<> ")
			f.mu.Unlock()
			reply(f.mail)
		case strings.HasPrefix(upper, "RCPT TO:"):
			f.mu.Lock()
			f.rcpts = append(f.rcpts, strings.Trim(line[len("RCPT TO:"):], "<> "))
			f.mu.Unlock()
			reply(f.rcpt)
		case upper == "DATA":
			reply(f.data)
			if !strings.HasPrefix(f.data, "354") {
				continue
			}
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			f.mu.Lock()
			f.body = b.String()
			f.mu.Unlock()
			reply(f.dataEnd)
		case upper == "QUIT":
			reply("221 bye")
			return
		default:
			reply("250 ok")
		}
	}
}

func (f *fakeSMTP) received() (bool, string, []string, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authed, f.from, append([]string(nil), f.rcpts...), f.body
}

func TestSend_Delivers(t *testing.T) {
	f := newFakeSMTP(t, nil)
	e := f.client()
	err := e.Send("OpsBlade <noreply@example.com>", []string{"Ops Team <ops@example.com>", "b@example.com"}, "Report", "line1\nline2")
	require.NoError(t, err)

	authed, from, rcpts, body := f.received()
	assert.True(t, authed, "PLAIN auth is used when SMTP_USER is set")
	assert.Equal(t, "noreply@example.com", from, "envelope sender is the bare address")
	assert.Equal(t, []string{"ops@example.com", "b@example.com"}, rcpts, "envelope recipients are bare addresses")
	assert.Contains(t, body, `From: "OpsBlade" <noreply@example.com>`+"\r\n", "headers keep the display name")
	assert.Contains(t, body, `To: "Ops Team" <ops@example.com>, <b@example.com>`+"\r\n")
	assert.Contains(t, body, "Subject: Report\r\n")
	assert.Contains(t, body, "\r\n\r\nline1\r\nline2\r\n")
}

func TestSend_NoAuthWithoutUser(t *testing.T) {
	f := newFakeSMTP(t, nil)
	e := f.client()
	e.Config.User = ""
	require.NoError(t, e.Send("a@example.com", []string{"b@example.com"}, "s", "b"))
	authed, _, _, _ := f.received()
	assert.False(t, authed)
}

func TestSend_ServerErrors(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(f *fakeSMTP)
		wantErr string
	}{
		{"bad greeting", func(f *fakeSMTP) { f.greeting = "554 go away" }, "SMTP handshake with"},
		{"starttls refused", func(f *fakeSMTP) { f.starttls = true }, "SMTP STARTTLS failed"},
		{"auth rejected", func(f *fakeSMTP) { f.auth = "535 no" }, "SMTP authentication failed"},
		{"sender rejected", func(f *fakeSMTP) { f.mail = "550 no" }, "SMTP MAIL FROM failed"},
		{"recipient rejected", func(f *fakeSMTP) { f.rcpt = "550 no" }, "SMTP RCPT TO b@example.com failed"},
		{"data rejected", func(f *fakeSMTP) { f.data = "554 no" }, "SMTP DATA failed"},
		{"message rejected", func(f *fakeSMTP) { f.dataEnd = "554 no" }, "SMTP DATA close failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakeSMTP(t, tc.setup)
			err := f.client().Send("a@example.com", []string{"b@example.com"}, "s", "b")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}
