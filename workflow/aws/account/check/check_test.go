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

const (
	fakeAccount = "111111111111"
	fakeArn     = "arn:aws:iam::111111111111:user/opsblade"
	fakeUserId  = "AIDAEXAMPLE"
)

// fakeSTS answers GetCallerIdentity with the given status code and, on success,
// the fake identity above. Static credentials and AWS_ENDPOINT_URL point the SDK at it.
func fakeSTS(t *testing.T, code int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		_, _ = fmt.Fprintf(w, `<GetCallerIdentityResponse xmlns="https://sts.amazonaws.com/doc/2011-06-15/">
  <GetCallerIdentityResult><Arn>%s</Arn><UserId>%s</UserId><Account>%s</Account></GetCallerIdentityResult>
  <ResponseMetadata><RequestId>test</RequestId></ResponseMetadata>
</GetCallerIdentityResponse>`, fakeArn, fakeUserId, fakeAccount)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ENDPOINT_URL", srv.URL)
}

func execute(t *testing.T, instructions map[string]any) shared.TaskResult {
	t.Helper()
	raw, err := json.Marshal(instructions)
	require.NoError(t, err)
	ctx := shared.TaskContext{Task: "aws_account_check", Sequence: 1, Instructions: raw, OnFail: shared.OnFailFatal}
	return shared.TaskRegistry["aws_account_check"](ctx).Execute()
}

func TestExecute_Match(t *testing.T) {
	fakeSTS(t, http.StatusOK)
	r := execute(t, map[string]any{"account_id": fakeAccount})
	assert.True(t, r.Success)
	assert.False(t, r.Stop)
	assert.Empty(t, r.OnFail)
	assert.Equal(t, true, r.Data["check_aws_account_passed"])
	assert.Equal(t, fakeAccount, r.Data["aws_account_id"])
	assert.Equal(t, fakeArn, r.Data["aws_arn"])
	assert.Equal(t, fakeUserId, r.Data["aws_user_id"])
}

func TestExecute_Mismatch(t *testing.T) {
	cases := []struct {
		name       string
		onMismatch any // nil means absent
		wantOnFail string
	}{
		{"default is fatal", nil, shared.OnFailFatal},
		{"fatal", "fatal", shared.OnFailFatal},
		{"warn", "warn", shared.OnFailWarn},
		{"stop", "stop", shared.OnFailStop},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeSTS(t, http.StatusOK)
			instr := map[string]any{"account_id": "222222222222"}
			if tc.onMismatch != nil {
				instr["on_mismatch"] = tc.onMismatch
			}
			r := execute(t, instr)
			assert.False(t, r.Success)
			assert.Equal(t, tc.wantOnFail, r.OnFail, "mismatch must carry on_mismatch as the on_fail override")
			assert.Contains(t, r.Msg, "belong to account 111111111111 but account 222222222222 is required")
			// Variables are still set so a later task can inspect the outcome
			assert.Equal(t, false, r.Data["check_aws_account_passed"])
			assert.Equal(t, fakeAccount, r.Data["aws_account_id"])
		})
	}
}

func TestExecute_MismatchKeepsErrorMessage(t *testing.T) {
	fakeSTS(t, http.StatusOK)
	raw, err := json.Marshal(map[string]any{"account_id": "222222222222"})
	require.NoError(t, err)
	ctx := shared.TaskContext{Task: "aws_account_check", Instructions: raw, ErrorMessage: "Wrong AWS credentials loaded."}
	r := shared.TaskRegistry["aws_account_check"](ctx).Execute()
	assert.False(t, r.Success)
	assert.Equal(t, shared.OnFailFatal, r.OnFail)
	assert.Contains(t, r.Msg, "Wrong AWS credentials loaded.")
}

// API errors must never be classified by on_mismatch: a 403 with on_mismatch
// set to stop must still be an ordinary failure governed by on_fail.
func TestExecute_APIErrorIsNotAMismatch(t *testing.T) {
	for _, code := range []int{http.StatusForbidden, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			fakeSTS(t, code)
			r := execute(t, map[string]any{"account_id": fakeAccount, "on_mismatch": "stop"})
			assert.False(t, r.Success)
			assert.Empty(t, r.OnFail, "an API error must be governed by on_fail, not on_mismatch")
			assert.False(t, r.Stop)
			assert.Contains(t, r.Msg, "failed to get AWS caller identity")
			assert.Empty(t, r.Data)
		})
	}
}

func TestExecute_AccountIdFromEnvironment(t *testing.T) {
	fakeSTS(t, http.StatusOK)
	t.Setenv("AWS_ACCOUNT_ID", fakeAccount)
	r := execute(t, map[string]any{})
	assert.True(t, r.Success)
	assert.Equal(t, true, r.Data["check_aws_account_passed"])

	t.Setenv("AWS_ACCOUNT_ID", "222222222222")
	r = execute(t, map[string]any{})
	assert.False(t, r.Success)
	assert.Equal(t, shared.OnFailFatal, r.OnFail)
	assert.Contains(t, r.Msg, "but account 222222222222 is required")
}

func TestExecute_FieldOverridesEnvironment(t *testing.T) {
	fakeSTS(t, http.StatusOK)
	t.Setenv("AWS_ACCOUNT_ID", "222222222222")
	r := execute(t, map[string]any{"account_id": fakeAccount})
	assert.True(t, r.Success)
}

func TestExecute_MissingAccountId(t *testing.T) {
	fakeSTS(t, http.StatusOK)
	t.Setenv("AWS_ACCOUNT_ID", "")
	r := execute(t, map[string]any{})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "account_id is required")
}

func TestExecute_InvalidOnMismatch(t *testing.T) {
	fakeSTS(t, http.StatusOK)
	r := execute(t, map[string]any{"account_id": fakeAccount, "on_mismatch": "abort"})
	assert.False(t, r.Success)
	assert.Empty(t, r.OnFail)
	assert.Contains(t, r.Msg, "invalid on_mismatch value")
}
