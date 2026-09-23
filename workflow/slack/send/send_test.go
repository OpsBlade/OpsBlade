// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package send

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/shared"
)

// fakeSlack records the text of every block posted to it and answers with code
type fakeSlack struct {
	srv   *httptest.Server
	mu    sync.Mutex
	calls int
	texts []string
}

func newFakeSlack(t *testing.T, code int) *fakeSlack {
	t.Helper()
	f := &fakeSlack{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Blocks []struct {
				Text struct {
					Text string `json:"text"`
				} `json:"text"`
			} `json:"blocks"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		f.mu.Lock()
		f.calls++
		for _, b := range payload.Blocks {
			f.texts = append(f.texts, b.Text.Text)
		}
		f.mu.Unlock()
		w.WriteHeader(code)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeSlack) received() (int, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, append([]string(nil), f.texts...)
}

func resetVars(t *testing.T) {
	t.Helper()
	shared.Variables = make(map[string]any)
	t.Cleanup(func() { shared.Variables = make(map[string]any) })
}

func execute(t *testing.T, ctx shared.TaskContext, instructions map[string]any) shared.TaskResult {
	t.Helper()
	raw, err := json.Marshal(instructions)
	require.NoError(t, err)
	ctx.Task = "slack_send"
	ctx.Instructions = raw
	return shared.TaskRegistry["slack_send"](ctx).Execute()
}

func TestExecute_Sends(t *testing.T) {
	f := newFakeSlack(t, http.StatusOK)
	t.Setenv("SLACK_WEBHOOK", f.srv.URL)
	r := execute(t, shared.TaskContext{Debug: true}, map[string]any{"subject": "Deploy", "body": "done"})
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "Slack message sent", r.Msg)
	assert.Equal(t, "Deploy", r.Data["slack_subject"])
	assert.Equal(t, "done", r.Data["slack_body"])
	calls, texts := f.received()
	assert.Equal(t, 1, calls)
	assert.Equal(t, []string{"*Deploy*", "done"}, texts)
}

func TestExecute_PrettyAppendsVariables(t *testing.T) {
	resetVars(t)
	shared.SetVar("info", map[string]any{"region": "ca-central-1"})
	f := newFakeSlack(t, http.StatusOK)
	t.Setenv("SLACK_WEBHOOK", f.srv.URL)
	r := execute(t, shared.TaskContext{}, map[string]any{
		"subject": "s", "body": "b", "pretty": []string{"info", "missing"},
	})
	require.True(t, r.Success, r.Msg)
	_, texts := f.received()
	require.Len(t, texts, 2)
	assert.Contains(t, texts[1], "b\n```\n")
	assert.Contains(t, texts[1], "region: ca-central-1")
	assert.Equal(t, 1, countOf(texts[1], "```\n"), "unset variables are not appended")
	assert.Equal(t, "b", r.Data["slack_body"], "returned body excludes the appended variables")
}

func countOf(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

func TestExecute_EnvSuffixAndEnvFile(t *testing.T) {
	f := newFakeSlack(t, http.StatusOK)

	t.Run("env_suffix", func(t *testing.T) {
		t.Setenv("SLACK_WEBHOOK", "")
		t.Setenv("SLACK_WEBHOOK_OPS", f.srv.URL)
		r := execute(t, shared.TaskContext{}, map[string]any{"subject": "s", "body": "b", "env_suffix": "_OPS"})
		assert.True(t, r.Success, r.Msg)
	})

	t.Run("env file", func(t *testing.T) {
		t.Setenv("SLACK_WEBHOOK", "")
		require.NoError(t, os.Unsetenv("SLACK_WEBHOOK"))
		envFile := filepath.Join(t.TempDir(), "slack.env")
		require.NoError(t, os.WriteFile(envFile, []byte("SLACK_WEBHOOK="+f.srv.URL+"\n"), 0o600))
		r := execute(t, shared.TaskContext{}, map[string]any{"subject": "s", "body": "b", "env": envFile})
		assert.True(t, r.Success, r.Msg)
	})
}

func TestExecute_DryRun(t *testing.T) {
	f := newFakeSlack(t, http.StatusOK)
	t.Setenv("SLACK_WEBHOOK", f.srv.URL)
	r := execute(t, shared.TaskContext{DryRun: true}, map[string]any{"subject": "s", "body": "b"})
	require.True(t, r.Success)
	assert.Equal(t, "DryRun, no message sent", r.Msg)
	assert.Empty(t, r.Data)
	calls, _ := f.received()
	assert.Equal(t, 0, calls)
}

func TestExecute_Errors(t *testing.T) {
	t.Run("webhook not configured", func(t *testing.T) {
		t.Setenv("SLACK_WEBHOOK", "")
		r := execute(t, shared.TaskContext{}, map[string]any{"subject": "s", "body": "b"})
		assert.False(t, r.Success)
		assert.Contains(t, r.Msg, "failed to create Slack client")
	})

	t.Run("slack rejects message", func(t *testing.T) {
		f := newFakeSlack(t, http.StatusNotFound)
		t.Setenv("SLACK_WEBHOOK", f.srv.URL)
		r := execute(t, shared.TaskContext{}, map[string]any{"subject": "s", "body": "b"})
		assert.False(t, r.Success)
		assert.Contains(t, r.Msg, "failed to send Slack message")
		assert.Empty(t, r.Data)
	})
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "slack_send", Instructions: []byte("{not json")}
	r := shared.TaskRegistry["slack_send"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
