// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudemail

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_ReadsEnvironment(t *testing.T) {
	t.Setenv("SMTP_HOST", "mail.example.com")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USER", "u")
	t.Setenv("SMTP_PASS", "p")

	e, err := New()
	require.NoError(t, err)
	assert.Equal(t, "mail.example.com", e.Config.Host)
	assert.Equal(t, 2525, e.Config.Port)
	assert.Equal(t, "u", e.Config.User)
	assert.Equal(t, "p", e.Config.Password)
}

func TestNew_DefaultPort(t *testing.T) {
	t.Setenv("SMTP_HOST", "mail.example.com")
	t.Setenv("SMTP_PORT", "")

	e, err := New()
	require.NoError(t, err)
	assert.Equal(t, defaultPort, e.Config.Port)
}

func TestNew_MissingHost(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	_, err := New()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SMTP_HOST")
}

func TestNew_BadPort(t *testing.T) {
	t.Setenv("SMTP_HOST", "mail.example.com")
	for _, p := range []string{"abc", "0", "70000"} {
		t.Setenv("SMTP_PORT", p)
		_, err := New()
		assert.Error(t, err, p)
	}
}

func TestNew_MissingEnvFile(t *testing.T) {
	_, err := New(WithEnvironment("/nonexistent/.env"))
	assert.Error(t, err)
}

func TestSend_ValidatesAddresses(t *testing.T) {
	e := &CloudEmail{Config: EmailConfig{Host: "localhost", Port: 1}}
	assert.Error(t, e.Send("a@example.com", nil, "s", "b"))
	assert.Error(t, e.Send("not an address", []string{"a@example.com"}, "s", "b"))
	assert.Error(t, e.Send("a@example.com", []string{"nope"}, "s", "b"))
}

func TestSend_AcceptsDisplayNames(t *testing.T) {
	// Validation must accept the "Name <addr>" form on both sides. Delivery is not
	// attempted here (port 1 refuses), so a connection error proves parsing passed.
	e := &CloudEmail{Config: EmailConfig{Host: "localhost", Port: 1}}
	err := e.Send("OpsBlade <noreply@example.com>", []string{"Ops Team <ops@example.com>"}, "s", "b")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "invalid from address")
	assert.NotContains(t, err.Error(), "invalid recipient address")
}

func TestMessage_Format(t *testing.T) {
	msg := Message("from@example.com", []string{"a@example.com", "b@example.com"}, "Hello\r\nBcc: x", "line1\nline2")
	assert.True(t, strings.HasPrefix(msg, "From: from@example.com\r\n"))
	assert.Contains(t, msg, "To: a@example.com, b@example.com\r\n")
	assert.Contains(t, msg, "Subject: Hello  Bcc: x\r\n", "header injection must be neutralised")
	assert.Contains(t, msg, "\r\n\r\nline1\r\nline2\r\n")
	assert.NotContains(t, msg, "\nBcc:")

	msg = Message("OpsBlade <noreply@example.com>", []string{"Ops Team <ops@example.com>"}, "s", "b")
	assert.True(t, strings.HasPrefix(msg, "From: OpsBlade <noreply@example.com>\r\n"))
	assert.Contains(t, msg, "To: Ops Team <ops@example.com>\r\n")
}
