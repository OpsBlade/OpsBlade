// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDryRunErrCheck(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "dry run success", err: errors.New("api error DryRunOperation: Request would have succeeded, but DryRun flag is set"), want: true},
		{name: "other error", err: errors.New("api error UnauthorizedOperation"), want: false},
		{name: "empty", err: errors.New(""), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, DryRunErrCheck(tt.err))
		})
	}
}
