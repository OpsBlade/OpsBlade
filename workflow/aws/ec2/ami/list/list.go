// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package list

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context shared.TaskContext      `yaml:"context" json:"context"` // Task context
	Env     string                  `yaml:"env" json:"env"`         // Optional file to load into the environment
	Region  string                  `yaml:"region" json:"region"`   // AWS region - allow overriding
	Profile string                  `yaml:"profile" json:"profile"` // AWS profile - allow overriding
	Owner   string                  `yaml:"owner" json:"owner"`     // Owner filter to pass to AWS ("self" is often useful)
	Filters []shared.Filter         `yaml:"filters" json:"filters"` // Filters to pass to AWS API
	Select  []shared.SelectCriteria `yaml:"select" json:"select"`   // Selection criteria to apply to the list of AMIs
	Fields  []string                `yaml:"fields" json:"fields"`   // List of fields to return as data
}

func init() {
	shared.RegisterTask("aws_ec2_ami_list", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error
	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

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
	req := &ec2.DescribeImagesInput{
		Filters: cloudaws.FiltersToEC2(t.Filters),
		DryRun:  &t.Context.DryRun,
	}
	if t.Owner != "" {
		req.Owners = []string{t.Owner}
	}

	// Set up paginator
	var amiData []any
	paginator := ec2.NewDescribeImagesPaginator(client, req,
		func(o *ec2.DescribeImagesPaginatorOptions) {
			o.Limit = 25
			o.StopOnDuplicateToken = true
		})

	// Get the pages
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return t.Context.Error("error describing images", err)
		}

		// Iterate over the images and apply selection criteria
		for _, image := range page.Images {
			if len(t.Select) > 0 {
				selected, err := shared.ApplySelectionCriteria(image, t.Select)
				if err != nil {
					return t.Context.Error("failed applying selection criteria", err)
				}
				if selected {
					amiData = append(amiData, shared.SelectFields(image, t.Fields))
				}
			} else {
				amiData = append(amiData, shared.SelectFields(image, t.Fields))
			}
		}
	}

	data := make(map[string]any)
	data["ami_data"] = amiData
	data["ami_count"] = len(amiData)
	return t.Context.Result(true, "AWS EC2 AMI list", data)
}
