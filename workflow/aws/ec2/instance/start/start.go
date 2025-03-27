// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package start

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context     shared.TaskContext `yaml:"context" json:"context"`         // Task context
	Credentials shared.Credentials `yaml:"credentials" json:"credentials"` // Allow override of credentials
	InstanceId  string             `yaml:"instance_id" json:"instance_id"` // Instance ID
}

func init() {
	shared.RegisterTask("aws_ec2_instance_start", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	// Resolve input variables
	shared.ProcessVars(t)

	if t.Context.Debug {
		shared.DumpTask(t)
	}

	// Resolve credentials with priority to task credentials, then context credentials
	creds := shared.NewCredentials(t.Credentials, *t.Context.Credentials)

	amazonInstance, err := cloudaws.New(
		cloudaws.WithRegion(creds.AWS.Region),
		cloudaws.WithAccessKey(creds.AWS.AccessKey),
		cloudaws.WithSecretKey(creds.AWS.SecretKey),
		cloudaws.WithProfile(creds.AWS.Profile),
		cloudaws.WithConfigFile(creds.AWS.ConfigFile),
		cloudaws.WithCredsFile(creds.AWS.CredsFile))
	if err != nil || amazonInstance == nil {
		return t.Context.Error("failed to create AWS client", err)
	}

	client := amazonInstance.EC2Client()
	req := &ec2.StartInstancesInput{
		InstanceIds: []string{t.InstanceId},
		DryRun:      &t.Context.DryRun}

	_, err = client.StartInstances(context.TODO(), req)
	if err != nil {
		if t.Context.DryRun && shared.DryRunErrCheck(err) {
			return t.Context.Result(true, fmt.Sprintf("Dryrun, AWS API returned: %s", err.Error()), nil)
		}
		return t.Context.Error("failed to start instance", err)
	}

	data := make(map[string]any)
	data["instance_id"] = t.InstanceId
	return t.Context.Result(
		true,
		"AWS EC2 Instance started",
		data)
}
