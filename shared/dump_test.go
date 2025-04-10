// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// TestDumpTask tests that DumpTask correctly excludes Context.Instructions
func TestDumpTask(t *testing.T) {
	// Create a test task with Context.Instructions
	testTask := &struct {
		Context TaskContext
		Name    string
	}{
		Context: TaskContext{
			Name:         "TestTask",
			Task:         "test_task",
			Sequence:     1,
			Instructions: []byte("sensitive data that should not be dumped"),
		},
		Name: "Test Task",
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Call DumpTask
	DumpTask(testTask)

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Read captured output
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify that the output does not contain the Instructions data
	if strings.Contains(output, "sensitive data that should not be dumped") {
		t.Errorf("DumpTask output contains Instructions data: %s", output)
	}

	// Verify that the output contains other task data
	if !strings.Contains(output, "TestTask") {
		t.Errorf("DumpTask output does not contain task name: %s", output)
	}

	// Parse the JSON output to verify structure
	var result map[string]interface{}
	jsonStr := strings.TrimSpace(output)
	err := json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		t.Errorf("Failed to parse JSON output: %v", err)
	}

	// Check that the dump field exists
	dump, ok := result["dump"]
	if !ok {
		t.Errorf("Output JSON does not contain 'dump' field: %s", jsonStr)
	}

	// Check that the task_dump field contains a task with Context
	taskDump, ok := dump.(map[string]interface{})
	if !ok {
		t.Errorf("'dump' field is not a map: %v", dump)
	}

	// Check that Context exists in the task_dump
	context, ok := taskDump["Context"].(map[string]interface{})
	if !ok {
		t.Errorf("Task dump does not contain Context field: %v", taskDump)
	}

	// Check that Instructions is empty or not present in Context
	instructions, exists := context["Instructions"]
	if exists {
		// If Instructions exists, it should be empty
		instructionsStr, ok := instructions.(string)
		if ok && instructionsStr != "" {
			t.Errorf("Context.Instructions is not empty: %s", instructionsStr)
		}

		instructionsArr, ok := instructions.([]interface{})
		if ok && len(instructionsArr) > 0 {
			t.Errorf("Context.Instructions is not empty: %v", instructionsArr)
		}
	}
}
