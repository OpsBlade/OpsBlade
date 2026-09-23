// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudjira

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andygrunwald/go-jira"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeJira starts an httptest server driven by handler and points the Jira
// environment variables at it.
func fakeJira(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("JIRA_USER", "user")
	t.Setenv("JIRA_TOKEN", "token")
	t.Setenv("JIRA_URL", srv.URL+"/")
	return srv
}

// jsonHandler answers every request with code and body.
func jsonHandler(code int, body any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(body)
	}
}

func newJira(t *testing.T) *CloudJira {
	t.Helper()
	j, err := New()
	require.NoError(t, err)
	return j
}

// brokenURL makes Client() fail: url.Parse rejects control characters.
func brokenURL(t *testing.T) {
	t.Helper()
	t.Setenv("JIRA_URL", "http://jira.example.com/\x7f")
}

func TestNew(t *testing.T) {
	t.Run("from environment", func(t *testing.T) {
		t.Setenv("JIRA_USER", "u")
		t.Setenv("JIRA_TOKEN", "tok")
		t.Setenv("JIRA_URL", "https://jira.example.com/")
		j, err := New()
		require.NoError(t, err)
		assert.Equal(t, JiraConfig{Username: "u", Token: "tok", BaseURL: "https://jira.example.com/"}, j.Config)
	})

	t.Run("missing variables", func(t *testing.T) {
		cases := []struct{ user, token, url string }{
			{"", "tok", "https://jira.example.com/"},
			{"u", "", "https://jira.example.com/"},
			{"u", "tok", ""},
		}
		for _, tc := range cases {
			t.Setenv("JIRA_USER", tc.user)
			t.Setenv("JIRA_TOKEN", tc.token)
			t.Setenv("JIRA_URL", tc.url)
			j, err := New()
			assert.Nil(t, j)
			assert.EqualError(t, err, "missing required Jira configuration")
		}
	})

	t.Run("environment file", func(t *testing.T) {
		// Register restoration with t.Setenv, then unset so the file values apply.
		for _, name := range []string{"JIRA_USER", "JIRA_TOKEN", "JIRA_URL"} {
			t.Setenv(name, "")
			require.NoError(t, os.Unsetenv(name))
		}
		envFile := filepath.Join(t.TempDir(), "jira.env")
		require.NoError(t, os.WriteFile(envFile,
			[]byte("JIRA_USER=fileuser\nJIRA_TOKEN=filetoken\nJIRA_URL=https://file.example.com/\n"), 0o600))
		j, err := New(WithEnvironment(envFile))
		require.NoError(t, err)
		assert.Equal(t, envFile, j.Config.Environment)
		assert.Equal(t, "fileuser", j.Config.Username)
		assert.Equal(t, "filetoken", j.Config.Token)
		assert.Equal(t, "https://file.example.com/", j.Config.BaseURL)
	})

	t.Run("missing environment file", func(t *testing.T) {
		j, err := New(WithEnvironment(filepath.Join(t.TempDir(), "missing.env")))
		assert.Nil(t, j)
		assert.Error(t, err)
	})

	t.Run("empty environment option is ignored", func(t *testing.T) {
		cfg := &JiraConfig{Environment: "keep"}
		WithEnvironment("")(cfg)
		assert.Equal(t, "keep", cfg.Environment)
	})
}

func TestClient(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fakeJira(t, jsonHandler(http.StatusOK, nil))
		client, err := newJira(t).Client()
		require.NoError(t, err)
		require.NotNil(t, client)
		base := client.GetBaseURL()
		assert.Equal(t, os.Getenv("JIRA_URL"), base.String())
	})

	t.Run("invalid base url", func(t *testing.T) {
		fakeJira(t, jsonHandler(http.StatusOK, nil))
		brokenURL(t)
		_, err := newJira(t).Client()
		assert.Error(t, err)
	})
}

func TestGetIssue(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fakeJira(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodGet, r.Method)
			assert.Equal(t, "/rest/api/2/issue/OPS-1", r.URL.Path)
			jsonHandler(http.StatusOK, map[string]any{"key": "OPS-1", "id": "10001"})(w, r)
		})
		issue, err := newJira(t).GetIssue("OPS-1")
		require.NoError(t, err)
		assert.Equal(t, "OPS-1", issue.Key)
		assert.Equal(t, "10001", issue.ID)
	})

	t.Run("api error", func(t *testing.T) {
		fakeJira(t, jsonHandler(http.StatusNotFound, map[string]any{"errorMessages": []string{"gone"}}))
		issue, err := newJira(t).GetIssue("OPS-1")
		assert.Nil(t, issue)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Status code: 404")
	})

	t.Run("client error", func(t *testing.T) {
		fakeJira(t, jsonHandler(http.StatusOK, nil))
		brokenURL(t)
		issue, err := newJira(t).GetIssue("OPS-1")
		assert.Nil(t, issue)
		assert.Error(t, err)
	})
}

func TestGetUser(t *testing.T) {
	cases := []struct {
		name    string
		code    int
		body    any
		wantID  string
		wantErr string
	}{
		{"found", http.StatusOK, []map[string]any{{"accountId": "acc-1"}, {"accountId": "acc-2"}}, "acc-1", ""},
		{"not found", http.StatusOK, []map[string]any{}, "", "user not found"},
		{"api error", http.StatusForbidden, map[string]any{"errorMessages": []string{"no"}}, "", "Status code: 403"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeJira(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/rest/api/2/user/search", r.URL.Path)
				assert.Equal(t, "a@example.com", r.URL.Query().Get("query"))
				jsonHandler(tc.code, tc.body)(w, r)
			})
			id, err := newJira(t).GetUser("a@example.com")
			assert.Equal(t, tc.wantID, id)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}

	t.Run("client error", func(t *testing.T) {
		fakeJira(t, jsonHandler(http.StatusOK, nil))
		brokenURL(t)
		id, err := newJira(t).GetUser("a@example.com")
		assert.Empty(t, id)
		assert.Error(t, err)
	})
}

func TestResolveTags(t *testing.T) {
	// Users are looked up by the query parameter; unknown users get an empty list.
	users := map[string]string{"alice@example.com": "acc-alice", "blank@example.com": ""}
	fakeJira(t, func(w http.ResponseWriter, r *http.Request) {
		id, ok := users[r.URL.Query().Get("query")]
		if !ok {
			jsonHandler(http.StatusOK, []map[string]any{})(w, r)
			return
		}
		jsonHandler(http.StatusOK, []map[string]any{{"accountId": id}})(w, r)
	})
	j := newJira(t)

	cases := []struct{ name, in, want string }{
		{"no tags", "plain text", "plain text"},
		{"bracket without at sign", "[not a tag]", "[not a tag]"},
		{"resolved", "hi [alice@example.com]!", "hi [~accountid:acc-alice]!"},
		{"unknown user is bolded", "hi [nobody@example.com]", "hi *nobody@example.com*"},
		{"empty account id is bolded", "[blank@example.com]", "*blank@example.com*"},
		{"mixed", "[alice@example.com] and [nobody@example.com]", "[~accountid:acc-alice] and *nobody@example.com*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, j.ResolveTags(tc.in))
		})
	}
}

func TestGetFields(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fakeJira(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/rest/api/2/field", r.URL.Path)
			jsonHandler(http.StatusOK, []map[string]any{{"id": "summary", "key": "summary"}})(w, r)
		})
		fields, err := newJira(t).GetFields()
		require.NoError(t, err)
		require.Len(t, fields, 1)
		assert.Equal(t, "summary", fields[0].Key)
	})

	t.Run("api error", func(t *testing.T) {
		fakeJira(t, jsonHandler(http.StatusInternalServerError, nil))
		fields, err := newJira(t).GetFields()
		assert.Empty(t, fields)
		assert.Error(t, err)
	})

	t.Run("client error", func(t *testing.T) {
		fakeJira(t, jsonHandler(http.StatusOK, nil))
		brokenURL(t)
		fields, err := newJira(t).GetFields()
		assert.Empty(t, fields)
		assert.Error(t, err)
	})
}

func TestGetSprintCustomField(t *testing.T) {
	sprintField := map[string]any{"id": "customfield_10020", "key": "customfield_10020",
		"schema": map[string]any{"custom": "com.pyxis.greenhopper.jira:gh-sprint"}}
	otherField := map[string]any{"id": "customfield_10001", "key": "customfield_10001",
		"schema": map[string]any{"custom": "com.atlassian.jira.plugin.system.customfieldtypes:textfield"}}

	cases := []struct {
		name    string
		code    int
		body    any
		want    string
		wantErr string
	}{
		{"found", http.StatusOK, []any{otherField, sprintField}, "customfield_10020", ""},
		{"not found", http.StatusOK, []any{otherField}, "", "sprint field not found"},
		{"api error", http.StatusBadGateway, nil, "", "Status code: 502"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeJira(t, jsonHandler(tc.code, tc.body))
			got, err := newJira(t).GetSprintCustomField()
			assert.Equal(t, tc.want, got)
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestGetActiveSprint(t *testing.T) {
	boards := map[string]any{"values": []map[string]any{{"id": 7, "name": "OPS board"}, {"id": 8}}}
	sprints := map[string]any{"values": []map[string]any{{"id": 42, "name": "Sprint 42", "state": "active"}}}
	noValues := map[string]any{"values": []any{}}

	cases := []struct {
		name       string
		boardCode  int
		boardBody  any
		sprintCode int
		sprintBody any
		wantID     int
		wantErr    string
	}{
		{"success", http.StatusOK, boards, http.StatusOK, sprints, 42, ""},
		{"no boards", http.StatusOK, noValues, http.StatusOK, sprints, 0, "no boards found for project OPS"},
		{"board api error", http.StatusUnauthorized, nil, http.StatusOK, sprints, 0, "Status code: 401"},
		{"no sprints", http.StatusOK, boards, http.StatusOK, noValues, 0, "no sprints found for board 7"},
		{"sprint api error", http.StatusOK, boards, http.StatusInternalServerError, nil, 0, "Status code: 500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeJira(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/rest/agile/1.0/board":
					assert.Equal(t, "OPS", r.URL.Query().Get("projectKeyOrId"))
					jsonHandler(tc.boardCode, tc.boardBody)(w, r)
				case "/rest/agile/1.0/board/7/sprint":
					assert.Equal(t, "active", r.URL.Query().Get("state"))
					jsonHandler(tc.sprintCode, tc.sprintBody)(w, r)
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					w.WriteHeader(http.StatusNotFound)
				}
			})
			sprint, err := newJira(t).GetActiveSprint("OPS")
			assert.Equal(t, tc.wantID, sprint.ID)
			if tc.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, "Sprint 42", sprint.Name)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}

	t.Run("client error", func(t *testing.T) {
		fakeJira(t, jsonHandler(http.StatusOK, nil))
		brokenURL(t)
		sprint, err := newJira(t).GetActiveSprint("OPS")
		assert.Equal(t, jira.Sprint{}, sprint)
		assert.Error(t, err)
	})
}

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("boom") }
func (failingBody) Close() error             { return nil }

func TestResponseToString(t *testing.T) {
	j := &CloudJira{}

	t.Run("returns body", func(t *testing.T) {
		resp := &jira.Response{Response: &http.Response{Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}}
		assert.Equal(t, `{"ok":true}`, j.ResponseToString(resp))
	})

	t.Run("read error", func(t *testing.T) {
		resp := &jira.Response{Response: &http.Response{Body: failingBody{}}}
		assert.Equal(t, "error reading response body: boom", j.ResponseToString(resp))
	})

	t.Run("nil response", func(t *testing.T) {
		assert.Equal(t, "no response", j.ResponseToString(nil))
	})

	t.Run("nil http response", func(t *testing.T) {
		assert.Equal(t, "no response", j.ResponseToString(&jira.Response{}))
	})
}
