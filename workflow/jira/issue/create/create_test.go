// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package create

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/shared"
)

const (
	sprintCustomField = "customfield_10020"
	sprintSchema      = "com.pyxis.greenhopper.jira:gh-sprint"
)

// jiraState configures the fake Jira and records what it received.
type jiraState struct {
	mu sync.Mutex

	users       map[string]string // query -> accountId; absent means not found
	boards      []map[string]any
	sprints     []map[string]any
	fields      []map[string]any
	createCode  int
	assignCode  int
	createdBody map[string]any // POST /issue body
	assignBody  map[string]any // PUT /issue/{id}/assignee body
}

func defaultState() *jiraState {
	return &jiraState{
		users:      map[string]string{"alice@example.com": "acc-alice"},
		boards:     []map[string]any{{"id": 7, "name": "OPS board"}},
		sprints:    []map[string]any{{"id": 42, "name": "Sprint 42", "state": "active"}},
		fields:     []map[string]any{{"key": sprintCustomField, "schema": map[string]any{"custom": sprintSchema}}},
		createCode: http.StatusCreated,
		assignCode: http.StatusNoContent,
	}
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func fakeJira(t *testing.T, st *jiraState) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		defer st.mu.Unlock()
		key := r.Method + " " + r.URL.Path
		switch key {
		case "GET /rest/api/2/user/search":
			if id, ok := st.users[r.URL.Query().Get("query")]; ok {
				writeJSON(w, http.StatusOK, []map[string]any{{"accountId": id}})
				return
			}
			writeJSON(w, http.StatusOK, []map[string]any{})
		case "GET /rest/agile/1.0/board":
			assert.Equal(t, "OPS", r.URL.Query().Get("projectKeyOrId"))
			writeJSON(w, http.StatusOK, map[string]any{"values": st.boards})
		case "GET /rest/agile/1.0/board/7/sprint":
			assert.Equal(t, "active", r.URL.Query().Get("state"))
			writeJSON(w, http.StatusOK, map[string]any{"values": st.sprints})
		case "GET /rest/api/2/field":
			writeJSON(w, http.StatusOK, st.fields)
		case "POST /rest/api/2/issue":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&st.createdBody))
			if st.createCode >= http.StatusBadRequest {
				writeJSON(w, st.createCode, map[string]any{"errorMessages": []string{"rejected"}})
				return
			}
			writeJSON(w, st.createCode, map[string]any{"id": "10001", "key": "OPS-1"})
		case "PUT /rest/api/2/issue/10001/assignee":
			require.NoError(t, json.NewDecoder(r.Body).Decode(&st.assignBody))
			w.WriteHeader(st.assignCode)
		default:
			t.Errorf("unexpected request %s", key)
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
	ctx.Task = "jira_issue_create"
	ctx.Instructions = raw
	return shared.TaskRegistry["jira_issue_create"](ctx).Execute()
}

func baseInstructions() map[string]any {
	return map[string]any{
		"project": "OPS", "issue_type": "Task", "summary": "Rotate keys",
		"description": "Owner [alice@example.com], cc [bob@example.com]",
	}
}

// createdFields returns the "fields" object of the create request.
func createdFields(t *testing.T, st *jiraState) map[string]any {
	t.Helper()
	require.NotNil(t, st.createdBody, "issue must have been created")
	fields, ok := st.createdBody["fields"].(map[string]any)
	require.True(t, ok)
	return fields
}

func TestExecute_Success(t *testing.T) {
	st := defaultState()
	fakeJira(t, st)
	r := execute(t, shared.TaskContext{Sequence: 4}, baseInstructions())
	require.True(t, r.Success, r.Msg)
	assert.Empty(t, r.OnFail)
	assert.Equal(t, 4, r.Sequence)
	assert.Equal(t, "JIRA issue OPS-1 created", r.Msg)
	assert.Equal(t, map[string]any{"jira_issue_id": "OPS-1", "jira_project": "OPS"}, r.Data)

	fields := createdFields(t, st)
	assert.Equal(t, "Rotate keys", fields["summary"])
	assert.Equal(t, "Owner [~accountid:acc-alice], cc *bob@example.com*", fields["description"], "tags must be resolved")
	assert.Equal(t, map[string]any{"name": "Task"}, fields["issuetype"])
	assert.Equal(t, map[string]any{"key": "OPS"}, fields["project"])
	assert.NotContains(t, fields, sprintCustomField)
	assert.Nil(t, st.assignBody, "no assignee must be set")
}

func TestExecute_SuccessWithAssigneeAndSprint(t *testing.T) {
	st := defaultState()
	fakeJira(t, st)
	instr := baseInstructions()
	instr["assignee"] = "alice@example.com"
	instr["active_sprint"] = true
	r := execute(t, shared.TaskContext{Debug: true}, instr)
	require.True(t, r.Success, r.Msg)
	assert.Empty(t, r.OnFail)
	assert.Equal(t, map[string]any{
		"jira_issue_id": "OPS-1", "jira_project": "OPS",
		"jira_assignee": "alice@example.com", "jira_assignee_account_id": "acc-alice",
	}, r.Data)

	fields := createdFields(t, st)
	assert.Equal(t, float64(42), fields[sprintCustomField], "sprint ID must be sent in the sprint custom field")
	assert.Equal(t, "acc-alice", st.assignBody["accountId"])
}

func TestExecute_DryRun(t *testing.T) {
	t.Cleanup(func() { delete(shared.Variables, "jira_issue_id") })
	st := defaultState()
	fakeJira(t, st)
	instr := baseInstructions()
	instr["assignee"] = "alice@example.com"
	instr["active_sprint"] = true
	r := execute(t, shared.TaskContext{DryRun: true}, instr)
	require.True(t, r.Success, r.Msg)
	assert.Empty(t, r.OnFail)
	assert.Equal(t, "Dry run, JIRA issue not created", r.Msg)
	assert.Empty(t, r.Data)
	assert.Equal(t, "jira-issue-dry-run", shared.Variables["jira_issue_id"])
	assert.Nil(t, st.createdBody, "dry run must not create an issue")
	assert.Nil(t, st.assignBody, "dry run must not assign an issue")
}

func TestExecute_DryRunAssigneeLookupIsReal(t *testing.T) {
	st := defaultState()
	fakeJira(t, st)
	instr := baseInstructions()
	instr["assignee"] = "nobody@example.com"
	r := execute(t, shared.TaskContext{DryRun: true}, instr)
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "unable to resolve JIRA user 'nobody@example.com'")
	assert.Contains(t, r.Msg, "user not found")
	assert.Nil(t, st.createdBody)
}

func TestExecute_Failures(t *testing.T) {
	sprintOn := func(m map[string]any) map[string]any { m["active_sprint"] = true; return m }
	assignee := func(m map[string]any) map[string]any { m["assignee"] = "alice@example.com"; return m }

	cases := []struct {
		name        string
		setup       func(st *jiraState)
		instr       func(m map[string]any) map[string]any
		debug       bool
		wantMsg     string
		wantCreated bool
	}{
		{
			name:    "assignee not found",
			setup:   func(st *jiraState) { delete(st.users, "alice@example.com") },
			instr:   assignee,
			wantMsg: "unable to resolve JIRA user 'alice@example.com': user not found",
		},
		{
			name:    "no boards for project",
			setup:   func(st *jiraState) { st.boards = nil },
			instr:   sprintOn,
			wantMsg: "failed to get active sprint: no boards found for project OPS",
		},
		{
			name:    "no active sprint",
			setup:   func(st *jiraState) { st.sprints = nil },
			instr:   sprintOn,
			wantMsg: "failed to get active sprint: no sprints found for board 7",
		},
		{
			name:    "sprint id zero",
			setup:   func(st *jiraState) { st.sprints = []map[string]any{{"id": 0, "name": "odd"}} },
			instr:   sprintOn,
			wantMsg: "jira returned ID 0 for sprint",
		},
		{
			name:    "sprint custom field missing",
			setup:   func(st *jiraState) { st.fields = []map[string]any{{"key": "summary"}} },
			instr:   sprintOn,
			wantMsg: "error determining custom field used for 'sprint': sprint field not found",
		},
		{
			name: "sprint custom field has empty key",
			setup: func(st *jiraState) {
				st.fields = []map[string]any{{"schema": map[string]any{"custom": sprintSchema}}}
			},
			instr:   sprintOn,
			wantMsg: "jira returned empty sprint field",
		},
		{
			name:        "create rejected",
			setup:       func(st *jiraState) { st.createCode = http.StatusBadRequest },
			instr:       func(m map[string]any) map[string]any { return m },
			wantMsg:     "failed to create JIRA issue",
			wantCreated: true,
		},
		{
			name:        "create rejected in debug mode",
			setup:       func(st *jiraState) { st.createCode = http.StatusInternalServerError },
			instr:       func(m map[string]any) map[string]any { return m },
			debug:       true,
			wantMsg:     "failed to create JIRA issue",
			wantCreated: true,
		},
		{
			name:        "assign rejected",
			setup:       func(st *jiraState) { st.assignCode = http.StatusForbidden },
			instr:       assignee,
			wantMsg:     "failed to update JIRA user assignee",
			wantCreated: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st := defaultState()
			tc.setup(st)
			fakeJira(t, st)
			r := execute(t, shared.TaskContext{Debug: tc.debug}, tc.instr(baseInstructions()))
			assert.False(t, r.Success)
			assert.Empty(t, r.OnFail, "API errors must be governed by on_fail")
			assert.False(t, r.Stop)
			assert.Contains(t, r.Msg, tc.wantMsg)
			assert.Empty(t, r.Data)
			assert.Equal(t, tc.wantCreated, st.createdBody != nil)
		})
	}
}

func TestExecute_TransportErrorInDebugMode(t *testing.T) {
	st := defaultState()
	srv := fakeJira(t, st)
	srv.Close() // connection refused: go-jira returns a nil response
	r := execute(t, shared.TaskContext{Debug: true}, baseInstructions())
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to create JIRA issue")
	assert.Nil(t, st.createdBody)
}

func TestExecute_MissingCredentials(t *testing.T) {
	t.Setenv("JIRA_USER", "")
	t.Setenv("JIRA_TOKEN", "")
	t.Setenv("JIRA_URL", "")
	r := execute(t, shared.TaskContext{}, baseInstructions())
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to create JIRA client")
	assert.Contains(t, r.Msg, "missing required Jira configuration")
}

func TestExecute_InvalidURL(t *testing.T) {
	t.Setenv("JIRA_USER", "user")
	t.Setenv("JIRA_TOKEN", "token")
	t.Setenv("JIRA_URL", "http://jira.example.com/\x7f")
	r := execute(t, shared.TaskContext{}, baseInstructions())
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "unable to create JIRA client")
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "jira_issue_create", Instructions: []byte(`{"project": 7}`)}
	r := shared.TaskRegistry["jira_issue_create"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
