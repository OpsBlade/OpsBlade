// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package check

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/shared"
)

// fakeJira serves GET /rest/api/2/issue/<key> with the given status code and, on
// success, an issue whose status and resolution names are the given values.
func fakeJira(t *testing.T, code int, status, resolution string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		fields := map[string]any{"status": map[string]any{"name": status}}
		if resolution != "" {
			fields["resolution"] = map[string]any{"name": resolution}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "OPS-1", "fields": fields})
	}))
	t.Cleanup(srv.Close)
	t.Setenv("JIRA_USER", "user")
	t.Setenv("JIRA_TOKEN", "token")
	t.Setenv("JIRA_URL", srv.URL+"/")
	return srv
}

func execute(t *testing.T, instructions map[string]any) shared.TaskResult {
	t.Helper()
	raw, err := json.Marshal(instructions)
	require.NoError(t, err)
	ctx := shared.TaskContext{Task: "jira_issue_check", Sequence: 1, Instructions: raw, OnFail: shared.OnFailFatal}
	return shared.TaskRegistry["jira_issue_check"](ctx).Execute()
}

func TestExecute_Match(t *testing.T) {
	fakeJira(t, http.StatusOK, "Done / Closed", "Done")
	r := execute(t, map[string]any{
		"issue_id": "OPS-1", "required_status": "done / closed", "required_resolution": "done",
	})
	assert.True(t, r.Success)
	assert.False(t, r.Stop)
	assert.Empty(t, r.OnFail)
	assert.Equal(t, true, r.Data["check_jira_issue_passed"])
	assert.Equal(t, "Done / Closed", r.Data["check_jira_issue_status"])
	assert.Equal(t, "Done", r.Data["check_jira_issue_resolution"])
}

func TestExecute_Mismatch(t *testing.T) {
	cases := []struct {
		name       string
		onMismatch any // nil means absent
		wantOnFail string
	}{
		{"default is stop", nil, shared.OnFailStop},
		{"stop", "stop", shared.OnFailStop},
		{"warn", "warn", shared.OnFailWarn},
		{"fatal", "fatal", shared.OnFailFatal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeJira(t, http.StatusOK, "In Progress", "")
			instr := map[string]any{"issue_id": "OPS-1", "required_status": "Done / Closed", "required_resolution": "Done"}
			if tc.onMismatch != nil {
				instr["on_mismatch"] = tc.onMismatch
			}
			r := execute(t, instr)
			assert.False(t, r.Success)
			assert.Equal(t, tc.wantOnFail, r.OnFail, "mismatch must carry on_mismatch as the on_fail override")
			assert.Contains(t, r.Msg, "is in status 'In Progress' but status 'Done / Closed' is required")
			// Variables are still set so a later task can inspect the outcome
			assert.Equal(t, false, r.Data["check_jira_issue_passed"])
			assert.Equal(t, "In Progress", r.Data["check_jira_issue_status"])
			assert.Equal(t, "none", r.Data["check_jira_issue_resolution"])
		})
	}
}

func TestExecute_ResolutionMismatch(t *testing.T) {
	fakeJira(t, http.StatusOK, "Done / Closed", "Won't Do")
	r := execute(t, map[string]any{
		"issue_id": "OPS-1", "required_status": "Done / Closed", "required_resolution": "Done", "on_mismatch": "warn",
	})
	assert.False(t, r.Success)
	assert.Equal(t, shared.OnFailWarn, r.OnFail)
	assert.Contains(t, r.Msg, "is in resolution 'Won't Do' but resolution 'Done' is required")
}

func TestExecute_MismatchKeepsErrorMessage(t *testing.T) {
	fakeJira(t, http.StatusOK, "In Progress", "")
	raw, err := json.Marshal(map[string]any{"issue_id": "OPS-1", "required_status": "Done"})
	require.NoError(t, err)
	ctx := shared.TaskContext{Task: "jira_issue_check", Instructions: raw, ErrorMessage: "Ticket not approved yet."}
	r := shared.TaskRegistry["jira_issue_check"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Equal(t, shared.OnFailStop, r.OnFail)
	assert.Contains(t, r.Msg, "Ticket not approved yet.")
}

// API errors must never be classified by on_mismatch: a 401 with the default
// on_mismatch (stop) must still be an ordinary failure governed by on_fail.
func TestExecute_APIErrorIsNotAMismatch(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			fakeJira(t, code, "", "")
			r := execute(t, map[string]any{"issue_id": "OPS-1", "required_status": "Done", "on_mismatch": "stop"})
			assert.False(t, r.Success)
			assert.Empty(t, r.OnFail, "an API error must be governed by on_fail, not on_mismatch")
			assert.False(t, r.Stop)
			assert.Contains(t, r.Msg, "failed to get JIRA issue")
			assert.Contains(t, r.Msg, fmt.Sprintf("Status code: %d", code))
			assert.Empty(t, r.Data)
		})
	}
}

func TestExecute_MissingConfigIsNotAMismatch(t *testing.T) {
	t.Setenv("JIRA_USER", "")
	t.Setenv("JIRA_TOKEN", "")
	t.Setenv("JIRA_URL", "")
	r := execute(t, map[string]any{"issue_id": "OPS-1", "required_status": "Done"})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to create JIRA client")
}

func TestExecute_InvalidOnMismatch(t *testing.T) {
	fakeJira(t, http.StatusOK, "Done", "Done")
	r := execute(t, map[string]any{"issue_id": "OPS-1", "required_status": "Done", "on_mismatch": "ignore"})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "invalid on_mismatch value")
	assert.Contains(t, r.Msg, `got "ignore"`)
}
