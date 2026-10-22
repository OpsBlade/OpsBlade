// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTask struct{}

func (stubTask) Execute() TaskResult { return TaskResult{Success: true} }

func TestRegisterTask(t *testing.T) {
	const id = "shared_test_stub"
	RegisterTask(id, func(TaskContext) Task { return stubTask{} })
	t.Cleanup(func() { delete(TaskRegistry, id) })

	constructor, ok := TaskRegistry[id]
	require.True(t, ok)
	assert.True(t, constructor(TaskContext{}).Execute().Success)
}

func TestTaskContextString(t *testing.T) {
	c := TaskContext{Sequence: 3, Name: "n", Task: "t"}
	assert.Equal(t, "Task 3 \"n\" (t)", c.String())
}

func TestTaskContextResult(t *testing.T) {
	c := TaskContext{Sequence: 3, Name: "n", Task: "t"}

	r := c.Result(true, "ok", map[string]any{"a": 1})
	assert.Equal(t, TaskResult{
		MessageType: "task_stop",
		Success:     true,
		Msg:         "ok",
		Sequence:    3,
		Name:        "n",
		Task:        "t",
		Data:        map[string]any{"a": 1},
	}, r)

	r = c.Result(false, "nil data", nil)
	assert.Equal(t, map[string]any{}, r.Data)
}

func TestTaskContextStop(t *testing.T) {
	c := TaskContext{Sequence: 1, Task: "t"}
	r := c.Stop("done", nil)
	assert.True(t, r.Success)
	assert.True(t, r.Stop)
	assert.Equal(t, "done", r.Msg)
}

func TestTaskContextError(t *testing.T) {
	tests := []struct {
		name    string
		ctx     TaskContext
		msg     string
		err     error
		wantMsg string
	}{
		{name: "nil error", ctx: TaskContext{}, msg: "boom", err: nil, wantMsg: "boom"},
		{name: "wrapped error", ctx: TaskContext{}, msg: "boom", err: errors.New("bad"), wantMsg: "boom: bad"},
		{name: "custom message", ctx: TaskContext{ErrorMessage: "custom"}, msg: "boom", err: errors.New("bad"), wantMsg: "boom: bad\n\ncustom\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.ctx.Error(tt.msg, tt.err)
			assert.False(t, r.Success)
			assert.Equal(t, tt.wantMsg, r.Msg)
		})
	}
}
