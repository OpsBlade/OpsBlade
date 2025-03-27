// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package create

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context      shared.TaskContext `yaml:"context" json:"context"`             // Task context
	Credentials  shared.Credentials `yaml:"credentials" json:"credentials"`     // Allow override of credentials
	InstanceID   string             `yaml:"instance_id" json:"instance_id"`     // Instance ID to create AMI from
	InstanceName string             `yaml:"instance_name" json:"instance_name"` // Name of the AMI
	Name         string             `yaml:"name" json:"name"`                   // Name of the task
	Description  string             `yaml:"description" json:"description"`     // Description of the AMI
	Tags         map[string]string  `yaml:"tags" json:"tags"`                   // Tags to apply to the AMI
	Fields       []string           `yaml:"fields" json:"fields"`               // List of fields to return as data
	NoReboot     bool               `yaml:"no_reboot" json:"no_reboot"`         // Do not reboot the instance before creating the AMI
}

func init() {
	shared.RegisterTask("aws_ec2_ami_create", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data to Task", err)
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

	var amiTags []ec2types.Tag
	for key, value := range t.Tags {
		amiTags = append(amiTags, ec2types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	client := amazonInstance.EC2Client()

	input := &ec2.CreateImageInput{
		InstanceId:  aws.String(t.InstanceID),
		Name:        aws.String(t.InstanceName),
		Description: aws.String(t.Description),
		NoReboot:    aws.Bool(t.NoReboot),
		DryRun:      aws.Bool(t.Context.DryRun),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeImage,
				Tags:         amiTags,
			},
		},
	}

	var imageId string
	result, err := client.CreateImage(context.TODO(), input)
	if err != nil {
		if t.Context.DryRun && shared.DryRunErrCheck(err) {
			return t.Context.Result(true, fmt.Sprintf("Dryrun, AWS API returned: %s", err.Error()), map[string]string{"image_id": "dry-run"})
		}
		return t.Context.Error("error creating image", err)
	}
	imageId = *result.ImageId

	return t.Context.Result(true, fmt.Sprintf("AWS AMI %s created", imageId), map[string]string{"image_id": imageId})
}
