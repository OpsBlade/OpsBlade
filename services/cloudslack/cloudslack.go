// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudslack

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

type CloudSlack struct {
	Config SlackConfig
}

type SlackConfig struct {
	Webhook string
	Debug   bool
}

type Option func(*SlackConfig)

func New(options ...Option) (*CloudSlack, error) {

	// Check to see if the environment is already loaded
	if os.Getenv("SLACK_WEBHOOK") == "" {
		// Load environment variables from the user's home/.slack file if it exists
		homeDir, err := os.UserHomeDir()
		if err == nil {
			slackFilePath := filepath.Join(homeDir, ".slack")
			_ = godotenv.Load(slackFilePath)
		}
	}

	// Initialize SlackConfig with default values from environment variables
	cfg := &SlackConfig{
		Webhook: os.Getenv("SLACK_WEBHOOK"),
		Debug:   false}

	// Apply user-provided options to override default values
	for _, opt := range options {
		opt(cfg)
	}

	// Validate that required fields are set
	if cfg.Webhook == "" {
		return nil, fmt.Errorf("webhook is not configured")
	}

	// Return the CloudSlack struct with the loaded configuration
	return &CloudSlack{
		Config: *cfg,
	}, nil
}

func WithWebhook(webhook string) Option {
	return func(cfg *SlackConfig) {
		if webhook != "" {
			cfg.Webhook = webhook
		}
	}
}

func WithDebug(debug bool) Option {
	return func(cfg *SlackConfig) {
		cfg.Debug = debug
	}
}
