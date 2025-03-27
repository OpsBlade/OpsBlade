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
	Context     shared.TaskContext      `yaml:"context" json:"context"`         // Task context
	Credentials shared.Credentials      `yaml:"credentials" json:"credentials"` // Allow override of credentials
	Owner       string                  `yaml:"owner" json:"owner"`             // Owner filter to pass to AWS ("self" is often useful)
	Filters     []shared.Filter         `yaml:"filters" json:"filters"`         // Filters to pass to AWS API
	Select      []shared.SelectCriteria `yaml:"select" json:"select"`           // Selection criteria to apply to the list of AMIs
	Fields      []string                `yaml:"fields" json:"fields"`           // List of fields to return as data
}

func init() {
	shared.RegisterTask("aws_ec2_instance_list", func(context shared.TaskContext) shared.Task {
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

	// Some AWS API calls allow owner filtering outside of tags, but not this one
	if t.Owner != "" {
		t.Filters = append(t.Filters, shared.Filter{Name: "owner-id", Values: []string{t.Owner}})
	}

	client := amazonInstance.EC2Client()
	req := &ec2.DescribeInstancesInput{
		Filters: cloudaws.FiltersToEC2(t.Filters),
		DryRun:  &t.Context.DryRun}

	// Set up the paginator
	var instanceData []any
	paginator := ec2.NewDescribeInstancesPaginator(client, req,
		func(o *ec2.DescribeInstancesPaginatorOptions) {
			o.Limit = 25
			o.StopOnDuplicateToken = true
		})

	// Get the pages
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.TODO())
		if err != nil {
			return t.Context.Error("error describing instances", err)
		}

		// Iterate over the instances and apply selection criteria
		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				if len(t.Select) > 0 {
					selected, err := shared.ApplySelectionCriteria(instance, t.Select)
					if err != nil {
						return t.Context.Error("failed applying selection criteria", err)
					}

					if selected {
						instanceData = append(instanceData, shared.SelectFields(instance, t.Fields))
					}
				} else {
					instanceData = append(instanceData, shared.SelectFields(instance, t.Fields))
				}
			}
		}
	}

	data := make(map[string]any)
	data["instance_data"] = instanceData
	data["instance_count"] = len(instanceData)
	return t.Context.Result(
		true,
		"AWS EC2 Instance list",
		data)
}
