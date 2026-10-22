// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

// Package awstest is a test-support package: it serves a fake AWS Query-protocol
// endpoint (EC2, Auto Scaling, STS) from an httptest server and points the SDK at it
// through the environment for the life of a test. It has no production callers.
package awstest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/OpsBlade/OpsBlade/shared"
)

// DryRunCode is the error code EC2 returns when a DryRun request would have succeeded.
const DryRunCode = "DryRunOperation"

// Handler answers one Query-protocol request. action is the Action form parameter
// and form holds the whole decoded request body. It returns the HTTP status and body.
type Handler func(action string, form url.Values) (int, string)

// Recorder keeps every request the fake endpoint received.
type Recorder struct {
	mu    sync.Mutex
	calls []call
}

type call struct {
	action string
	form   url.Values
}

// Count returns how many requests carried the given action.
func (r *Recorder) Count(action string) int {
	return len(r.Forms(action))
}

// Forms returns the decoded request bodies of every request that carried the given action.
func (r *Recorder) Forms(action string) []url.Values {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []url.Values
	for _, c := range r.calls {
		if c.action == action {
			out = append(out, c.form)
		}
	}
	return out
}

// Serve starts the fake endpoint and configures static credentials, a region,
// AWS_ENDPOINT_URL and a single retry attempt so every SDK client built by the
// test talks to it. The server is closed and the environment restored on cleanup.
func Serve(t *testing.T, h Handler) *Recorder {
	t.Helper()
	rec := &Recorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		action := r.PostForm.Get("Action")
		rec.mu.Lock()
		rec.calls = append(rec.calls, call{action: action, form: r.PostForm})
		rec.mu.Unlock()
		status, body := h(action, r.PostForm)
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ENDPOINT_URL", srv.URL)
	t.Setenv("AWS_MAX_ATTEMPTS", "1")
	t.Setenv("AWS_PROFILE", "")
	return rec
}

// EC2Error builds an ec2query-protocol error body.
func EC2Error(code, msg string) string {
	return fmt.Sprintf(`<Response><Errors><Error><Code>%s</Code><Message>%s</Message></Error></Errors><RequestID>test</RequestID></Response>`, code, msg)
}

// EC2DryRun is the 412 EC2 returns when a DryRun request would have succeeded.
func EC2DryRun() (int, string) {
	return http.StatusPreconditionFailed, EC2Error(DryRunCode, "Request would have succeeded, but DryRun flag is set.")
}

// ASGError builds an awsquery-protocol (Auto Scaling, STS) error body.
func ASGError(code, msg string) string {
	return fmt.Sprintf(`<ErrorResponse xmlns="http://autoscaling.amazonaws.com/doc/2011-01-01/"><Error><Type>Sender</Type><Code>%s</Code><Message>%s</Message></Error><RequestId>test</RequestId></ErrorResponse>`, code, msg)
}

// Unknown is the reply for an action the test did not expect.
func Unknown(action string) (int, string) {
	return http.StatusBadRequest, EC2Error("InvalidAction", "unexpected action "+action)
}

// Reject is a Handler for tests that expect no request at all.
func Reject(action string, _ url.Values) (int, string) {
	return Unknown(action)
}

// Run marshals instructions into ctx.Instructions and executes the registered task.
func Run(t *testing.T, taskID string, ctx shared.TaskContext, instructions map[string]any) shared.TaskResult {
	t.Helper()
	raw, err := json.Marshal(instructions)
	if err != nil {
		t.Fatalf("marshal instructions: %v", err)
	}
	ctx.Instructions = raw
	if ctx.Task == "" {
		ctx.Task = taskID
	}
	constructor, ok := shared.TaskRegistry[taskID]
	if !ok {
		t.Fatalf("task %q is not registered", taskID)
	}
	return constructor(ctx).Execute()
}
