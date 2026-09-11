// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

// Package cloudemail sends plain-text email over SMTP. Server settings come from the
// environment only, following the same credential rules as the other services:
//
//	SMTP_HOST  required
//	SMTP_PORT  optional, default 587; 465 selects implicit TLS
//	SMTP_USER  optional; PLAIN authentication is used when set
//	SMTP_PASS  optional
package cloudemail

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

const (
	defaultPort     = 587
	implicitTLSPort = 465
)

type CloudEmail struct {
	Config EmailConfig
}

type EmailConfig struct {
	Env      string
	Host     string
	Port     int
	User     string
	Password string
	Debug    bool
}

type Option func(*EmailConfig)

// New loads the optional env file and reads SMTP settings from the environment
func New(options ...Option) (*CloudEmail, error) {
	cfg := &EmailConfig{Port: defaultPort}

	for _, opt := range options {
		opt(cfg)
	}

	if cfg.Env != "" {
		if err := godotenv.Load(cfg.Env); err != nil {
			return nil, err
		}
	}

	cfg.Host = os.Getenv("SMTP_HOST")
	cfg.User = os.Getenv("SMTP_USER")
	cfg.Password = os.Getenv("SMTP_PASS")

	if p := os.Getenv("SMTP_PORT"); p != "" {
		port, err := strconv.Atoi(p)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("SMTP_PORT is not a valid port: %q", p)
		}
		cfg.Port = port
	}

	if cfg.Host == "" {
		return nil, fmt.Errorf("SMTP_HOST is not configured")
	}

	return &CloudEmail{Config: *cfg}, nil
}

func WithEnvironment(env string) Option {
	return func(cfg *EmailConfig) {
		if env != "" {
			cfg.Env = env
		}
	}
}

func WithDebug(debug bool) Option {
	return func(cfg *EmailConfig) {
		cfg.Debug = debug
	}
}
