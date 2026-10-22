// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudslack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// requestTimeout bounds a webhook call so a hung Slack endpoint cannot stall the workflow
const requestTimeout = 30 * time.Second

// sectionTextLimit is Slack's maximum length, in characters, of a section block's text
const sectionTextLimit = 3000

// fenceClose and fenceOpen close and re-open a ``` code fence across a block break
const (
	fenceClose = "\n```"
	fenceOpen  = "```\n"
)

type SlackMessage struct {
	Blocks []map[string]any `json:"blocks"`
}

func (s *CloudSlack) SendMessage(subject, message string) error {
	var err error

	payload := SlackMessage{
		Blocks: []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": "*" + subject + "*",
				},
			},
		},
	}
	for _, chunk := range splitText(message) {
		payload.Blocks = append(payload.Blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": chunk,
			},
		})
	}

	// Serialize the payload to JSON
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Set up the HTTP POST request
	req, err := http.NewRequest("POST", s.Config.Webhook, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	// Send it
	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != 200 {
		// Slack returns a short reason in the body, such as channel_not_found or channel_is_archived
		reason, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		if r := strings.TrimSpace(string(reason)); r != "" {
			return fmt.Errorf("non-200 response from Slack: %s (%s)", resp.Status, r)
		}
		return fmt.Errorf("non-200 response from Slack: %s", resp.Status)
	}
	return nil
}

// splitText splits message into chunks of at most sectionTextLimit characters, one per
// section block. It breaks on line boundaries, hard-splits a line that cannot fit on its
// own, and closes and re-opens a ``` code fence that is open at a break.
func splitText(message string) []string {
	if utf8.RuneCountInString(message) <= sectionTextLimit {
		return []string{message}
	}

	var chunks []string
	var b strings.Builder
	n := 0 // characters in b
	hasLines := false
	inFence := false

	flush := func() {
		// A chunk with no visible text is dropped: Slack rejects an empty section block.
		content := b.String()
		if inFence {
			content = strings.TrimPrefix(content, fenceOpen)
		}
		if strings.TrimSpace(content) != "" {
			if inFence {
				b.WriteString(fenceClose)
			}
			chunks = append(chunks, b.String())
		}
		b.Reset()
		n, hasLines = 0, false
		if inFence {
			b.WriteString(fenceOpen)
			n = utf8.RuneCountInString(fenceOpen)
		}
	}

	closeLen := utf8.RuneCountInString(fenceClose)
	for _, line := range strings.Split(message, "\n") {
		fenceAfter := inFence != strings.HasPrefix(line, "```")
		lineLen := utf8.RuneCountInString(line)
		need := func() int {
			c := lineLen
			if hasLines {
				c++
			}
			if fenceAfter {
				c += closeLen
			}
			return c
		}

		if n+need() > sectionTextLimit && hasLines {
			flush()
		}
		if n+need() <= sectionTextLimit {
			if hasLines {
				b.WriteByte('\n')
				n++
			}
			b.WriteString(line)
			n += lineLen
			hasLines = true
			inFence = fenceAfter
			continue
		}

		// The line does not fit even in an empty chunk: give each piece its own chunk,
		// leaving room for a fence opener and closer. A fence the line opens or closes
		// applies from its first piece.
		inFence = fenceAfter
		runes := []rune(line)
		size := sectionTextLimit - utf8.RuneCountInString(fenceOpen) - closeLen
		for start := 0; start < len(runes); start += size {
			if start > 0 {
				flush()
			}
			end := min(start+size, len(runes))
			b.WriteString(string(runes[start:end]))
			n += end - start
			hasLines = true
		}
	}
	flush()
	return chunks
}
