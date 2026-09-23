// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudslack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_ReadsWebhook(t *testing.T) {
	t.Setenv("SLACK_WEBHOOK", "https://hooks.example.com/default")
	t.Setenv("SLACK_WEBHOOK_OPS", "https://hooks.example.com/ops")

	cases := []struct {
		name    string
		options []Option
		want    string
		debug   bool
	}{
		{"default", nil, "https://hooks.example.com/default", false},
		{"suffix", []Option{WithEnvSuffix("_OPS")}, "https://hooks.example.com/ops", false},
		{"empty suffix is ignored", []Option{WithEnvSuffix("")}, "https://hooks.example.com/default", false},
		{"debug", []Option{WithDebug(true)}, "https://hooks.example.com/default", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := New(tc.options...)
			require.NoError(t, err)
			assert.Equal(t, tc.want, s.Config.Webhook)
			assert.Equal(t, tc.debug, s.Config.Debug)
		})
	}
}

func TestNew_EnvFile(t *testing.T) {
	t.Setenv("SLACK_WEBHOOK", "")
	require.NoError(t, os.Unsetenv("SLACK_WEBHOOK"))
	envFile := filepath.Join(t.TempDir(), "slack.env")
	require.NoError(t, os.WriteFile(envFile, []byte("SLACK_WEBHOOK=https://hooks.example.com/file\n"), 0o600))

	s, err := New(WithEnvironment(envFile))
	require.NoError(t, err)
	assert.Equal(t, envFile, s.Config.Env)
	assert.Equal(t, "https://hooks.example.com/file", s.Config.Webhook)

	_, err = New(WithEnvironment(filepath.Join(t.TempDir(), "missing.env")))
	assert.Error(t, err)

	s, err = New(WithEnvironment(""), WithEnvironment(envFile))
	require.NoError(t, err)
	assert.Equal(t, envFile, s.Config.Env, "an empty environment option is ignored")
}

func TestNew_MissingWebhook(t *testing.T) {
	t.Setenv("SLACK_WEBHOOK", "")
	_, err := New()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook is not configured")
}

func TestSendMessage(t *testing.T) {
	cases := []struct {
		name    string
		code    int
		body    string
		wantErr string
	}{
		{"accepted", http.StatusOK, "ok", ""},
		{"rejected with reason", http.StatusNotFound, "channel_not_found\n", "non-200 response from Slack: 404 Not Found (channel_not_found)"},
		{"rejected without reason", http.StatusInternalServerError, "", "non-200 response from Slack: 500 Internal Server Error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got SlackMessage
			var contentType string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				contentType = r.Header.Get("Content-Type")
				require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)

			s := &CloudSlack{Config: SlackConfig{Webhook: srv.URL}}
			err := s.SendMessage("Subject", "Body text")
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, "application/json; charset=utf-8", contentType)
			require.Len(t, got.Blocks, 2)
			assert.Equal(t, map[string]any{"type": "mrkdwn", "text": "*Subject*"}, got.Blocks[0]["text"])
			assert.Equal(t, map[string]any{"type": "mrkdwn", "text": "Body text"}, got.Blocks[1]["text"])
		})
	}
}

func TestSendMessage_TransportErrors(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()

	cases := []struct {
		name    string
		webhook string
	}{
		{"invalid url", "://not a url"},
		{"connection refused", srv.URL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &CloudSlack{Config: SlackConfig{Webhook: tc.webhook}}
			assert.Error(t, s.SendMessage("s", "m"))
		})
	}
}
