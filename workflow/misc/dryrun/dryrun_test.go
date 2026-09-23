// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package dryrun

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/OpsBlade/OpsBlade/shared"
)

func TestExecute(t *testing.T) {
	cases := []struct {
		name    string
		dryRun  bool
		wantOK  bool
		wantMsg string
	}{
		{"dry run enabled", true, true, "Dryrun is confirmed"},
		{"dry run disabled", false, false, "dryrun required but not enabled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := shared.TaskContext{Task: "dryrun_or_die", DryRun: tc.dryRun}
			r := shared.TaskRegistry["dryrun_or_die"](ctx).Execute()
			assert.Equal(t, tc.wantOK, r.Success)
			assert.Contains(t, r.Msg, tc.wantMsg)
		})
	}
}
