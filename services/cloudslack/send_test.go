// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudslack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// longFencedYAML builds a message shaped like slack_send output: body text followed by a
// pretty-printed list inside a ``` code fence.
func longFencedYAML(entries int) string {
	var b strings.Builder
	b.WriteString("ASG refresh finished\n```\n")
	for i := range entries {
		fmt.Fprintf(&b, "- name: asg-%03d\n  status: Successful\n  percentage_complete: 100\n", i)
	}
	b.WriteString("```")
	return b.String()
}

// fenceLines counts the lines of s that start a ``` code fence.
func fenceLines(s string) int {
	count := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "```") {
			count++
		}
	}
	return count
}

func TestSendMessage_SplitsLongMessages(t *testing.T) {
	atLimit := strings.Repeat("a", sectionTextLimit)
	overLimit := strings.Repeat("a", sectionTextLimit/2) + "\n" + strings.Repeat("b", sectionTextLimit/2)
	fenced := longFencedYAML(100)
	longLine := strings.Repeat("x", sectionTextLimit*2+10)

	cases := []struct {
		name    string
		message string
		// wantBlocks is the exact block count, or 0 for more than two.
		wantBlocks int
		// rebuild checks that the chunks join back into the message with lineSep once
		// fence closers and openers are removed: a break on a line boundary drops the
		// newline, a hard split does not.
		rebuild bool
		lineSep string
	}{
		{"just under the limit", atLimit[1:], 2, true, "\n"},
		{"exactly the limit", atLimit, 2, true, "\n"},
		{"one over the limit", overLimit, 3, true, "\n"},
		{"multi-line yaml in a code fence", fenced, 0, true, "\n"},
		{"single line over the limit", longLine, 4, true, ""},
		{"trailing newline adds no empty block", atLimit + "\n", 2, false, ""},
		{"long fence opener line", "```" + strings.Repeat("y", 4000) + "\nfoo\n```", 0, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got SlackMessage
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&got))
				_, _ = w.Write([]byte("ok"))
			}))
			t.Cleanup(srv.Close)

			s := &CloudSlack{Config: SlackConfig{Webhook: srv.URL}}
			require.NoError(t, s.SendMessage("Subject", tc.message))

			if tc.wantBlocks > 0 {
				require.Len(t, got.Blocks, tc.wantBlocks)
			} else {
				require.Greater(t, len(got.Blocks), 2)
			}
			assert.Equal(t, map[string]any{"type": "mrkdwn", "text": "*Subject*"}, got.Blocks[0]["text"])

			originalLines := make(map[string]bool)
			for _, line := range strings.Split(tc.message, "\n") {
				originalLines[line] = true
			}

			chunks := make([]string, 0, len(got.Blocks)-1)
			for _, block := range got.Blocks[1:] {
				text, ok := block["text"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "mrkdwn", text["type"])
				chunk, ok := text["text"].(string)
				require.True(t, ok)
				assert.NotEmpty(t, strings.TrimSpace(chunk), "no block is empty")
				assert.LessOrEqual(t, utf8.RuneCountInString(chunk), sectionTextLimit)
				assert.Zero(t, fenceLines(chunk)%2, "code fences are balanced in each block")
				chunks = append(chunks, chunk)
			}

			if !tc.rebuild {
				return
			}

			// Remove the closer and opener added at each break that fell inside a fence.
			for i := 0; i < len(chunks)-1; i++ {
				if strings.HasSuffix(chunks[i], fenceClose) && strings.HasPrefix(chunks[i+1], fenceOpen) {
					chunks[i] = strings.TrimSuffix(chunks[i], fenceClose)
					chunks[i+1] = strings.TrimPrefix(chunks[i+1], fenceOpen)
				}
			}
			assert.Equal(t, tc.message, strings.Join(chunks, tc.lineSep))

			if tc.lineSep == "\n" {
				for _, chunk := range chunks {
					for _, line := range strings.Split(chunk, "\n") {
						assert.True(t, originalLines[line], "line %q is not split", line)
					}
				}
			}
		})
	}
}
