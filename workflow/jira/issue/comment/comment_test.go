// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package comment

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/shared"
)

// fakeJira serves user search (for tag resolution) and the add-comment endpoint.
// The comment body Jira received is stored in *gotBody.
func fakeJira(t *testing.T, commentCode int, gotBody *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/2/user/search":
			if r.URL.Query().Get("query") == "alice@example.com" {
				_, _ = w.Write([]byte(`[{"accountId":"acc-alice"}]`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/2/issue/OPS-1/comment":
			var body struct {
				Body string `json:"body"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			*gotBody = body.Body
			w.WriteHeader(commentCode)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "1", "body": body.Body})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("JIRA_USER", "user")
	t.Setenv("JIRA_TOKEN", "token")
	t.Setenv("JIRA_URL", srv.URL+"/")
	return srv
}

func execute(t *testing.T, ctx shared.TaskContext, instructions map[string]any) shared.TaskResult {
	t.Helper()
	raw, err := json.Marshal(instructions)
	require.NoError(t, err)
	ctx.Task = "jira_issue_comment"
	ctx.Instructions = raw
	return shared.TaskRegistry["jira_issue_comment"](ctx).Execute()
}

func TestExecute_Success(t *testing.T) {
	var got string
	fakeJira(t, http.StatusCreated, &got)
	r := execute(t, shared.TaskContext{Sequence: 3, Debug: true},
		map[string]any{"issue_id": "OPS-1", "comment": "ping [alice@example.com] and [bob@example.com]"})
	assert.True(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Equal(t, 3, r.Sequence)
	assert.Equal(t, "comment added to JIRA issue OPS-1", r.Msg)
	assert.Empty(t, r.Data)
	assert.Equal(t, "ping [~accountid:acc-alice] and *bob@example.com*", got, "tags must be resolved before posting")
}

func TestExecute_MissingFields(t *testing.T) {
	cases := []struct {
		name  string
		instr map[string]any
	}{
		{"no issue_id", map[string]any{"comment": "hello"}},
		{"no comment", map[string]any{"issue_id": "OPS-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			fakeJira(t, http.StatusCreated, &got)
			r := execute(t, shared.TaskContext{}, tc.instr)
			assert.False(t, r.Success)
			assert.Empty(t, r.OnFail)
			assert.Equal(t, "issue_id and comment are required", r.Msg)
			assert.Empty(t, got, "no request must be made")
		})
	}
}

func TestExecute_APIError(t *testing.T) {
	var got string
	fakeJira(t, http.StatusForbidden, &got)
	r := execute(t, shared.TaskContext{}, map[string]any{"issue_id": "OPS-1", "comment": "hello"})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to add comment to JIRA issue")
	assert.Contains(t, r.Msg, "Status code: 403")
	assert.Empty(t, r.Data)
}

func TestExecute_MissingCredentials(t *testing.T) {
	t.Setenv("JIRA_USER", "")
	t.Setenv("JIRA_TOKEN", "")
	t.Setenv("JIRA_URL", "")
	r := execute(t, shared.TaskContext{}, map[string]any{"issue_id": "OPS-1", "comment": "hello"})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to create JIRA client")
	assert.Contains(t, r.Msg, "missing required Jira configuration")
}

func TestExecute_InvalidURL(t *testing.T) {
	t.Setenv("JIRA_USER", "user")
	t.Setenv("JIRA_TOKEN", "token")
	t.Setenv("JIRA_URL", "http://jira.example.com/\x7f")
	r := execute(t, shared.TaskContext{}, map[string]any{"issue_id": "OPS-1", "comment": "hello"})
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "unable to create JIRA client")
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "jira_issue_comment", Instructions: []byte(`{"issue_id": 7}`)}
	r := shared.TaskRegistry["jira_issue_comment"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
