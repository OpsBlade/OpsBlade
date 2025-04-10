// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package changeImage

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context          shared.TaskContext `yaml:"context" json:"context"`   // Task context
	Env              string             `yaml:"env" json:"env"`           // Optional file to load into the environment
	Region           string             `yaml:"region" json:"region"`     // AWS region - allow overriding
	Profile          string             `yaml:"profile" json:"profile"`   // AWS profile - allow overriding
	LaunchTemplateId string             `yaml:"lt_id" json:"lt_id"`       // Launch Template ID for which to create new version
	ImageId          string             `yaml:"image_id" json:"image_id"` // AMI ImageID to specify in new version
	Filters          []shared.Filter    `yaml:"filters" json:"filters"`   // Filters to pass to AWS API
	Fields           []string           `yaml:"fields" json:"fields"`     // List of fields to return as data (if empty, all fields are returned)
}

func init() {
	shared.RegisterTask("aws_ec2_lt_change_image", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error
	var defaultVersion int64

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	// Resolve input variables
	shared.ProcessVars(t)

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

	// Get the current default version to use as a template
	resp, err := client.DescribeLaunchTemplateVersions(context.TODO(), &ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: aws.String(t.LaunchTemplateId),
		Versions:         []string{"$Default"},
		Filters:          cloudaws.FiltersToEC2(t.Filters),
	})
	if err != nil {
		return t.Context.Error(fmt.Sprintf("failed to describe default launch template version for %s", t.LaunchTemplateId), err)
	}

	// Extract the default version from the response
	if len(resp.LaunchTemplateVersions) > 0 {
		defaultVersion = *resp.LaunchTemplateVersions[0].VersionNumber
	} else {
		return t.Context.Error(fmt.Sprintf("failed to obtain default launch template version for %s", t.LaunchTemplateId), nil)
	}

	if defaultVersion < 1 {
		return t.Context.Error(fmt.Sprintf("launch template version for %s is less than 1, aborting", t.LaunchTemplateId), nil)
	}

	// Create a new version using the current default as a template
	defaultVersionStr := fmt.Sprintf("%v", defaultVersion)
	input := &ec2.CreateLaunchTemplateVersionInput{
		LaunchTemplateId: aws.String(t.LaunchTemplateId),
		SourceVersion:    aws.String(defaultVersionStr),
		LaunchTemplateData: &types.RequestLaunchTemplateData{
			ImageId: aws.String(t.ImageId),
		},
		DryRun: aws.Bool(t.Context.DryRun),
	}

	newTemplate, err := client.CreateLaunchTemplateVersion(context.TODO(), input)
	if err != nil {
		if t.Context.DryRun && shared.DryRunErrCheck(err) {
			return t.Context.Result(true, fmt.Sprintf("Dryrun, AWS API returned: %s", err.Error()), nil)
		}
		return t.Context.Error("failed to create new launch template version", err)
	}

	// Get the new version number and convert it to a string
	newVersion := fmt.Sprintf("%d", *newTemplate.LaunchTemplateVersion.VersionNumber)
	if newVersion == "" {
		return t.Context.Error("new launch template version number is missing, aborting", nil)
	}

	// Set the new version as the default
	newDefault, err := client.ModifyLaunchTemplate(context.TODO(), &ec2.ModifyLaunchTemplateInput{
		LaunchTemplateId: aws.String(t.LaunchTemplateId),
		DefaultVersion:   aws.String(newVersion),
	})
	if err != nil {
		return t.Context.Error("failed to set new launch template version as default", err)
	}

	data := map[string]any{
		"lt_id":            t.LaunchTemplateId,
		"image_id":         t.ImageId,
		"previous_default": defaultVersionStr,
		"new_version":      newVersion,
	}

	data["launch_template"] = shared.SelectFields(newDefault.LaunchTemplate, t.Fields)

	return t.Context.Result(
		true,
		fmt.Sprintf("AWS EC2 Launch Template version %s created and set as default", newVersion),
		data)
}
