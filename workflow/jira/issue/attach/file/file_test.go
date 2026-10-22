// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package file

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/shared"
)

// upload records the multipart file part Jira received.
type upload struct {
	fileName string
	content  string
}

// fakeJira serves the attachment endpoint for OPS-1 with the given status code
// and records the uploaded part in *got.
func fakeJira(t *testing.T, code int, got *upload) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issue/OPS-1/attachments" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		assert.Equal(t, "nocheck", r.Header.Get("X-Atlassian-Token"))
		f, header, err := r.FormFile("file")
		require.NoError(t, err)
		defer func() { _ = f.Close() }()
		content, err := io.ReadAll(f)
		require.NoError(t, err)
		*got = upload{fileName: header.Filename, content: string(content)}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode([]map[string]any{{"id": "1", "filename": header.Filename}})
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
	ctx.Task = "jira_issue_attach_file"
	ctx.Instructions = raw
	return shared.TaskRegistry["jira_issue_attach_file"](ctx).Execute()
}

// tempFile writes content to name inside a fresh temp directory and returns its path.
func tempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestExecute_Success(t *testing.T) {
	cases := []struct {
		name     string
		fileName string
	}{
		{"nested path", "report.txt"},
		{"bare name", "bare.log"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got upload
			fakeJira(t, http.StatusOK, &got)
			path := tempFile(t, tc.fileName, "hello attachment")
			if tc.name == "bare name" {
				// Run with the file in the working directory so no separator is present.
				t.Chdir(filepath.Dir(path))
				path = tc.fileName
			}
			r := execute(t, shared.TaskContext{Sequence: 2, Debug: true},
				map[string]any{"issue_id": "OPS-1", "file_name": path})
			assert.True(t, r.Success)
			assert.Empty(t, r.OnFail)
			assert.Equal(t, 2, r.Sequence)
			assert.Equal(t, "file attached to JIRA issue OPS-1", r.Msg)
			assert.Equal(t, map[string]any{"jira_attached_file_name": tc.fileName}, r.Data)
			assert.Equal(t, upload{fileName: tc.fileName, content: "hello attachment"}, got)
		})
	}
}

func TestExecute_MissingFields(t *testing.T) {
	cases := []struct {
		name  string
		instr map[string]any
	}{
		{"no issue_id", map[string]any{"file_name": "x.txt"}},
		{"no file_name", map[string]any{"issue_id": "OPS-1"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got upload
			fakeJira(t, http.StatusOK, &got)
			r := execute(t, shared.TaskContext{}, tc.instr)
			assert.False(t, r.Success)
			assert.Empty(t, r.OnFail)
			assert.Equal(t, "issue_id and file_name are required", r.Msg)
			assert.Equal(t, upload{}, got, "no request must be made")
		})
	}
}

func TestExecute_FileNotFound(t *testing.T) {
	var got upload
	fakeJira(t, http.StatusOK, &got)
	missing := filepath.Join(t.TempDir(), "missing.txt")
	r := execute(t, shared.TaskContext{}, map[string]any{"issue_id": "OPS-1", "file_name": missing})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to open file")
	assert.Equal(t, upload{}, got, "no request must be made")
}

func TestExecute_APIError(t *testing.T) {
	var got upload
	fakeJira(t, http.StatusRequestEntityTooLarge, &got)
	path := tempFile(t, "big.bin", "data")
	r := execute(t, shared.TaskContext{}, map[string]any{"issue_id": "OPS-1", "file_name": path})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to attach file to JIRA issue")
	assert.Contains(t, r.Msg, "Status code: 413")
	assert.Empty(t, r.Data)
}

func TestExecute_MissingCredentials(t *testing.T) {
	t.Setenv("JIRA_USER", "")
	t.Setenv("JIRA_TOKEN", "")
	t.Setenv("JIRA_URL", "")
	r := execute(t, shared.TaskContext{}, map[string]any{"issue_id": "OPS-1", "file_name": "x.txt"})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to create JIRA client")
	assert.Contains(t, r.Msg, "missing required Jira configuration")
}

func TestExecute_InvalidURL(t *testing.T) {
	t.Setenv("JIRA_USER", "user")
	t.Setenv("JIRA_TOKEN", "token")
	t.Setenv("JIRA_URL", "http://jira.example.com/\x7f")
	r := execute(t, shared.TaskContext{}, map[string]any{"issue_id": "OPS-1", "file_name": "x.txt"})
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "unable to create JIRA client")
}

func TestExecute_DeserializeError(t *testing.T) {
	ctx := shared.TaskContext{Task: "jira_issue_attach_file", Instructions: []byte(`{"issue_id": 7}`)}
	r := shared.TaskRegistry["jira_issue_attach_file"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
