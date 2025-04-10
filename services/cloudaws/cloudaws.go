// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudaws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/joho/godotenv"
)

type CloudAWS struct {
	Config *AWSConfig
	AWS    *aws.Config
}

type AWSConfig struct {
	Profile     string
	Region      string
	AccessKey   string
	SecretKey   string
	Environment string
}

type Option func(*AWSConfig)

func New(options ...Option) (*CloudAWS, error) {
	cfg := &AWSConfig{}

	// Apply user-provided options to override default values
	for _, opt := range options {
		opt(cfg)
	}

	var awsCfg aws.Config
	var err error

	// If an environment file is provided, load it. If not, load from the default AWS files
	if cfg.Environment != "" {
		err = godotenv.Load(cfg.Environment)
		if err != nil {
			return nil, err
		}
	}

	// Attempt to load from the environment
	cfg.AccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	cfg.SecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	cfg.Region = os.Getenv("AWS_REGION")

	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		awsCfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
			config.WithRegion(cfg.Region))
	} else if cfg.Profile != "" { // If profile is provided, use shared config profile
		if cfg.Region == "" { // If region is set, use it
			awsCfg, err = config.LoadDefaultConfig(context.TODO(),
				config.WithSharedConfigProfile(cfg.Profile))
		} else {
			awsCfg, err = config.LoadDefaultConfig(context.TODO(),
				config.WithSharedConfigProfile(cfg.Profile),
				config.WithRegion(cfg.Region))
		}
	} else { // Otherwise, use the default credential provider chain
		if cfg.Region == "" { // If region is set, use it
			awsCfg, err = config.LoadDefaultConfig(context.TODO())
		} else {
			awsCfg, err = config.LoadDefaultConfig(context.TODO(),
				config.WithRegion(cfg.Region))
		}
	}

	// Return an error if unable to configure
	if err != nil {
		return nil, err
	}

	// Return the CloudAWS struct with the loaded configuration
	return &CloudAWS{AWS: &awsCfg, Config: cfg}, nil
}

func WithProfile(profile string) Option {
	return func(cfg *AWSConfig) {
		if profile != "" {
			cfg.Profile = profile
		}
	}
}

func WithRegion(region string) Option {
	return func(cfg *AWSConfig) {
		if region != "" {
			cfg.Region = region
		}
	}
}

func WithEnvironment(env string) Option {
	return func(cfg *AWSConfig) {
		if env != "" {
			cfg.Environment = env
		}
	}
}

func (c *CloudAWS) Dump() {
	// Marshall the configuration to JSON and print it
	data, err := json.MarshalIndent(c.Config, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal CloudAWS config: %s\n", err.Error())
		return
	}
	fmt.Printf("cloudaws:\n%s\n\n", string(data))
}
