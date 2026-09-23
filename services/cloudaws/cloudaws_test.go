// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package cloudaws

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/shared"
)

// isolate points the SDK at an empty shared config and credentials file and
// clears every credential and region variable so a test starts from nothing.
func isolate(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config")
	require.NoError(t, os.WriteFile(configFile, []byte("[profile test]\nregion = eu-west-1\n"), 0o600))
	t.Setenv("AWS_CONFIG_FILE", configFile)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	for _, name := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_REGION", "AWS_DEFAULT_REGION", "AWS_PROFILE", "AWS_ENDPOINT_URL"} {
		t.Setenv(name, "")
	}
	return dir
}

func TestNew_StaticKeys(t *testing.T) {
	isolate(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_REGION", "us-east-1")

	c, err := New()
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, "AKIAEXAMPLE", c.Config.AccessKey)
	assert.Equal(t, "secret", c.Config.SecretKey)
	assert.Equal(t, "us-east-1", c.AWS.Region)

	creds, err := c.AWS.Credentials.Retrieve(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "AKIAEXAMPLE", creds.AccessKeyID)
	assert.Equal(t, "secret", creds.SecretAccessKey)
}

func TestNew_EnvironmentFile(t *testing.T) {
	dir := isolate(t)
	// godotenv only fills variables that are absent, so drop the empty placeholders
	// (t.Setenv above restores them on cleanup).
	for _, name := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_REGION"} {
		require.NoError(t, os.Unsetenv(name))
	}
	envFile := filepath.Join(dir, "aws.env")
	require.NoError(t, os.WriteFile(envFile,
		[]byte("AWS_ACCESS_KEY_ID=AKIAFROMFILE\nAWS_SECRET_ACCESS_KEY=filesecret\nAWS_REGION=ap-southeast-2\n"), 0o600))

	c, err := New(WithEnvironment(envFile))
	require.NoError(t, err)
	assert.Equal(t, envFile, c.Config.Environment)
	assert.Equal(t, "AKIAFROMFILE", c.Config.AccessKey)
	assert.Equal(t, "filesecret", c.Config.SecretKey)
	assert.Equal(t, "ap-southeast-2", c.AWS.Region)
}

func TestNew_EnvironmentFileMissing(t *testing.T) {
	dir := isolate(t)
	c, err := New(WithEnvironment(filepath.Join(dir, "missing.env")))
	assert.Error(t, err)
	assert.Nil(t, c)
}

func TestNew_Profile(t *testing.T) {
	cases := []struct {
		name       string
		region     string // AWS_REGION in the environment; empty leaves it to the profile
		wantRegion string
	}{
		{"region from profile", "", "eu-west-1"},
		{"region from environment", "ca-central-1", "ca-central-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolate(t)
			t.Setenv("AWS_REGION", tc.region)
			c, err := New(WithProfile("test"))
			require.NoError(t, err)
			assert.Equal(t, "test", c.Config.Profile)
			assert.Empty(t, c.Config.AccessKey)
			assert.Equal(t, tc.wantRegion, c.AWS.Region)
		})
	}
}

func TestNew_ProfileMissing(t *testing.T) {
	isolate(t)
	c, err := New(WithProfile("nonexistent"))
	assert.Error(t, err)
	assert.Nil(t, c)
}

func TestNew_DefaultChain(t *testing.T) {
	cases := []struct {
		name       string
		region     string
		wantRegion string
	}{
		{"no region", "", ""},
		{"region from environment", "us-east-2", "us-east-2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolate(t)
			t.Setenv("AWS_REGION", tc.region)
			c, err := New()
			require.NoError(t, err)
			assert.Empty(t, c.Config.Profile)
			assert.Empty(t, c.Config.AccessKey)
			assert.Equal(t, tc.wantRegion, c.AWS.Region)
		})
	}
}

func TestOptions_IgnoreEmpty(t *testing.T) {
	cfg := &AWSConfig{Profile: "p", Region: "r", Environment: "e"}
	WithProfile("")(cfg)
	WithRegion("")(cfg)
	WithEnvironment("")(cfg)
	assert.Equal(t, &AWSConfig{Profile: "p", Region: "r", Environment: "e"}, cfg)

	WithProfile("p2")(cfg)
	WithRegion("r2")(cfg)
	WithEnvironment("e2")(cfg)
	assert.Equal(t, &AWSConfig{Profile: "p2", Region: "r2", Environment: "e2"}, cfg)
}

func TestClients(t *testing.T) {
	isolate(t)
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIAEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_REGION", "us-east-1")
	c, err := New()
	require.NoError(t, err)
	assert.NotNil(t, c.EC2Client())
	assert.NotNil(t, c.ASGClient())
	assert.NotNil(t, c.STSClient())
	c.Dump()
}

func TestFiltersToEC2(t *testing.T) {
	assert.Nil(t, FiltersToEC2(nil))
	got := FiltersToEC2([]shared.Filter{
		{Name: "tag:env", Values: []string{"prod", "stage"}},
		{Name: "instance-state-name", Values: []string{"running"}},
	})
	require.Len(t, got, 2)
	assert.Equal(t, "tag:env", *got[0].Name)
	assert.Equal(t, []string{"prod", "stage"}, got[0].Values)
	assert.Equal(t, "instance-state-name", *got[1].Name)
	assert.Equal(t, []string{"running"}, got[1].Values)
}

func TestFiltersToASG(t *testing.T) {
	assert.Nil(t, FiltersToASG(nil))
	got := FiltersToASG([]shared.Filter{{Name: "tag-key", Values: []string{"env"}}})
	require.Len(t, got, 1)
	assert.Equal(t, "tag-key", *got[0].Name)
	assert.Equal(t, []string{"env"}, got[0].Values)
}
