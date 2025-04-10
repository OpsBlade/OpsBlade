// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package stop

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context    shared.TaskContext `yaml:"context" json:"context"`         // Task context
	Env        string             `yaml:"env" json:"env"`                 // Optional file to load into the environment
	Region     string             `yaml:"region" json:"region"`           // AWS region - allow overriding
	Profile    string             `yaml:"profile" json:"profile"`         // AWS profile - allow overriding
	InstanceId string             `yaml:"instance_id" json:"instance_id"` // Instance ID
	Force      bool               `yaml:"force" json:"force"`             // Force stop
}

func init() {
	shared.RegisterTask("aws_ec2_instance_stop", func(context shared.TaskContext) shared.Task {
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

	// Resolve environment file
	envFile := shared.SelectEnv(t.Env, t.Context.Env)

	amazonInstance, err := cloudaws.New(
		cloudaws.WithRegion(t.Region),
		cloudaws.WithEnvironment(envFile),
		cloudaws.WithProfile(t.Profile))
	if err != nil || amazonInstance == nil {
		return t.Context.Error("failed to create AWS client", err)
	}

	client := amazonInstance.EC2Client()
	req := &ec2.StopInstancesInput{
		InstanceIds: []string{t.InstanceId},
		Force:       aws.Bool(t.Force),
		DryRun:      &t.Context.DryRun}

	_, err = client.StopInstances(context.TODO(), req)
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
		"AWS EC2 Instance stopping",
		data)
}
