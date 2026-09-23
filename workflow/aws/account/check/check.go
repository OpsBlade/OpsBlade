// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package check

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/OpsBlade/OpsBlade/services/cloudaws"
	"github.com/OpsBlade/OpsBlade/shared"
)

type Task struct {
	Context    shared.TaskContext `yaml:"context" json:"context"`         // Task context
	Env        string             `yaml:"env" json:"env"`                 // Optional file to load into the environment
	Region     string             `yaml:"region" json:"region"`           // AWS region - allow overriding
	Profile    string             `yaml:"profile" json:"profile"`         // AWS profile - allow overriding
	AccountId  string             `yaml:"account_id" json:"account_id"`   // Account the credentials must belong to
	OnMismatch string             `yaml:"on_mismatch" json:"on_mismatch"` // What a wrong account means: fatal (default), warn, or stop
}

func init() {
	shared.RegisterTask("aws_account_check", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

func (t *Task) Execute() shared.TaskResult {
	var err error
	data := make(map[string]any)

	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	shared.ProcessVars(t)

	if t.Context.Debug {
		shared.DumpTask(t)
	}

	if t.AccountId == "" {
		return t.Context.Error("account_id is required", nil)
	}

	// A wrong account is an expected condition, not an error, so it is classified by
	// on_mismatch rather than on_fail. API and credential errors still use on_fail.
	switch t.OnMismatch {
	case "":
		t.OnMismatch = shared.OnFailFatal
	case shared.OnFailStop, shared.OnFailWarn, shared.OnFailFatal:
	default:
		return t.Context.Error("invalid on_mismatch value",
			fmt.Errorf("on_mismatch must be %q, %q, or %q, got %q",
				shared.OnFailStop, shared.OnFailWarn, shared.OnFailFatal, t.OnMismatch))
	}

	amazonInstance, err := cloudaws.New(
		cloudaws.WithRegion(t.Region),
		cloudaws.WithEnvironment(shared.SelectEnv(t.Env, t.Context.Env)),
		cloudaws.WithProfile(t.Profile))
	if err != nil || amazonInstance == nil {
		return t.Context.Error("failed to create AWS client", err)
	}

	// GetCallerIdentity is read-only and needs no permissions, so it runs in dry run too.
	identity, err := amazonInstance.STSClient().GetCallerIdentity(context.TODO(), &sts.GetCallerIdentityInput{})
	if err != nil {
		return t.Context.Error("failed to get AWS caller identity", err)
	}

	data["aws_account_id"] = aws.ToString(identity.Account)
	data["aws_arn"] = aws.ToString(identity.Arn)
	data["aws_user_id"] = aws.ToString(identity.UserId)
	data["check_aws_account_passed"] = false

	if aws.ToString(identity.Account) != t.AccountId {
		r := t.Context.Error(
			fmt.Sprintf("AWS credentials belong to account %s but account %s is required (%s)",
				aws.ToString(identity.Account), t.AccountId, aws.ToString(identity.Arn)), nil)
		r.Data = data
		r.OnFail = t.OnMismatch
		return r
	}

	data["check_aws_account_passed"] = true
	return t.Context.Result(true,
		fmt.Sprintf("AWS credentials belong to account %s (%s)", t.AccountId, aws.ToString(identity.Arn)), data)
}
