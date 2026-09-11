// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

// Package transcript captures everything the process writes to stdout and stderr
// while still passing it through, so a run's output can be attached to a
// notification without asking the caller to redirect it to a file.
package transcript

import (
	"bytes"
	"io"
	"os"
	"sync"
)

// Transcript is an active capture of stdout and stderr
type Transcript struct {
	buf            lockedBuffer
	stdout, stderr *os.File // the originals, restored by Stop
	wOut, wErr     *os.File // pipe write ends installed as os.Stdout and os.Stderr
	wg             sync.WaitGroup
}

// lockedBuffer is a bytes.Buffer safe for use by the two tee goroutines
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// Start replaces os.Stdout and os.Stderr with pipes. Everything written to them is
// copied to the original streams and to the transcript until Stop is called.
func Start() (*Transcript, error) {
	rOut, wOut, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		_ = rOut.Close()
		_ = wOut.Close()
		return nil, err
	}

	t := &Transcript{stdout: os.Stdout, stderr: os.Stderr, wOut: wOut, wErr: wErr}
	t.wg.Add(2)
	go t.tee(rOut, t.stdout)
	go t.tee(rErr, t.stderr)
	os.Stdout, os.Stderr = wOut, wErr
	return t, nil
}

// tee copies a pipe to its original stream and to the transcript until the write end closes
func (t *Transcript) tee(r *os.File, original *os.File) {
	defer t.wg.Done()
	defer func() { _ = r.Close() }()
	_, _ = io.Copy(io.MultiWriter(original, &t.buf), r)
}

// Stop restores os.Stdout and os.Stderr, waits for buffered output to drain, and
// returns everything captured. It is safe to call more than once.
func (t *Transcript) Stop() string {
	if t.wOut != nil {
		os.Stdout, os.Stderr = t.stdout, t.stderr
		_ = t.wOut.Close()
		_ = t.wErr.Close()
		t.wg.Wait()
		t.wOut, t.wErr = nil, nil
	}
	return t.buf.String()
}
