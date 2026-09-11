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
)

// requestTimeout bounds a webhook call so a hung Slack endpoint cannot stall the workflow
const requestTimeout = 30 * time.Second

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
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": message,
				},
			},
		},
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
