// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package transcript

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartStop(t *testing.T) {
	origOut, origErr := os.Stdout, os.Stderr

	tr, err := Start()
	require.NoError(t, err)
	assert.NotEqual(t, origOut, os.Stdout, "stdout must be replaced while capturing")
	assert.NotEqual(t, origErr, os.Stderr, "stderr must be replaced while capturing")

	fmt.Println("to stdout")
	fmt.Fprintln(os.Stderr, "to stderr")
	fmt.Print("no newline")

	got := tr.Stop()
	assert.Equal(t, origOut, os.Stdout, "stdout must be restored")
	assert.Equal(t, origErr, os.Stderr, "stderr must be restored")
	assert.Contains(t, got, "to stdout\n")
	assert.Contains(t, got, "to stderr\n")
	assert.Contains(t, got, "no newline")

	// A second Stop is harmless and returns the same transcript
	assert.Equal(t, got, tr.Stop())
	assert.Equal(t, origOut, os.Stdout)

	// Output after Stop is not captured
	fmt.Println("after stop")
	assert.NotContains(t, tr.Stop(), "after stop")
}

func TestStartPassesThrough(t *testing.T) {
	// Point the real stdout at a pipe so the pass-through copy can be observed
	r, w, err := os.Pipe()
	require.NoError(t, err)
	origOut := os.Stdout
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = origOut })

	tr, err := Start()
	require.NoError(t, err)
	fmt.Println("visible")
	tr.Stop()
	_ = w.Close()

	buf := make([]byte, 64)
	n, _ := r.Read(buf)
	assert.Equal(t, "visible\n", string(buf[:n]))
}
