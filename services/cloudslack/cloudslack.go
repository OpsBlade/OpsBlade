// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudslack

import (
	"fmt"
	"github.com/joho/godotenv"
	"os"
)

type CloudSlack struct {
	Config SlackConfig
}

type SlackConfig struct {
	Env     string
	Webhook string
	Debug   bool
}

type Option func(*SlackConfig)

func New(options ...Option) (*CloudSlack, error) {

	cfg := &SlackConfig{Debug: false}

	// Apply provided options
	for _, opt := range options {
		opt(cfg)
	}

	if cfg.Env != "" {
		err := godotenv.Load(cfg.Env)
		if err != nil {
			return nil, err
		}
	}

	// Load the webhook from the environment
	cfg.Webhook = os.Getenv("SLACK_WEBHOOK")

	// Validate that required fields are set
	if cfg.Webhook == "" {
		return nil, fmt.Errorf("webhook is not configured")
	}

	// Return the CloudSlack struct with the loaded configuration
	return &CloudSlack{
		Config: *cfg,
	}, nil
}

func WithEnvironment(env string) Option {
	return func(cfg *SlackConfig) {
		if env != "" {
			cfg.Env = env
		}
	}
}

func WithDebug(debug bool) Option {
	return func(cfg *SlackConfig) {
		cfg.Debug = debug
	}
}
