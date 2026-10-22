// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package changeImage

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/OpsBlade/OpsBlade/services/cloudaws/awstest"
	"github.com/OpsBlade/OpsBlade/shared"
)

const taskID = "aws_ec2_lt_change_image"

func describeVersions(versions ...int) string {
	items := ""
	for _, v := range versions {
		items += fmt.Sprintf(`<item><launchTemplateId>lt-1</launchTemplateId><launchTemplateName>web</launchTemplateName>
      <versionNumber>%d</versionNumber><defaultVersion>true</defaultVersion>
      <launchTemplateData><imageId>ami-old</imageId></launchTemplateData></item>`, v)
	}
	return fmt.Sprintf(`<DescribeLaunchTemplateVersionsResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId><launchTemplateVersionSet>%s</launchTemplateVersionSet>
</DescribeLaunchTemplateVersionsResponse>`, items)
}

const createVersionResponse = `<CreateLaunchTemplateVersionResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <launchTemplateVersion><launchTemplateId>lt-1</launchTemplateId><versionNumber>4</versionNumber>
    <launchTemplateData><imageId>ami-new</imageId></launchTemplateData></launchTemplateVersion>
</CreateLaunchTemplateVersionResponse>`

// Responses to CreateLaunchTemplateVersion that omit the new version entirely
// or omit only its number.
const createVersionNoVersion = `<CreateLaunchTemplateVersionResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
</CreateLaunchTemplateVersionResponse>`

const createVersionNoNumber = `<CreateLaunchTemplateVersionResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <launchTemplateVersion><launchTemplateId>lt-1</launchTemplateId>
    <launchTemplateData><imageId>ami-new</imageId></launchTemplateData></launchTemplateVersion>
</CreateLaunchTemplateVersionResponse>`

const modifyResponse = `<ModifyLaunchTemplateResponse xmlns="http://ec2.amazonaws.com/doc/2016-11-15/">
  <requestId>test</requestId>
  <launchTemplate><launchTemplateId>lt-1</launchTemplateId><launchTemplateName>web</launchTemplateName>
    <defaultVersionNumber>4</defaultVersionNumber><latestVersionNumber>4</latestVersionNumber></launchTemplate>
</ModifyLaunchTemplateResponse>`

type reply struct {
	status int
	body   string
}

func ok(body string) reply { return reply{http.StatusOK, body} }

func serve(t *testing.T, replies map[string]reply) *awstest.Recorder {
	t.Helper()
	return awstest.Serve(t, func(action string, _ url.Values) (int, string) {
		if r, found := replies[action]; found {
			return r.status, r.body
		}
		return awstest.Unknown(action)
	})
}

func instructions() map[string]any {
	return map[string]any{
		"lt_id":    "lt-1",
		"image_id": "ami-new",
		"filters":  []map[string]any{{"name": "is-default-version", "values": []string{"true"}}},
	}
}

func TestExecute_Success(t *testing.T) {
	rec := serve(t, map[string]reply{
		"DescribeLaunchTemplateVersions": ok(describeVersions(3)),
		"CreateLaunchTemplateVersion":    ok(createVersionResponse),
		"ModifyLaunchTemplate":           ok(modifyResponse),
	})
	instr := instructions()
	instr["fields"] = []string{"LaunchTemplateId", "DefaultVersionNumber"}
	r := awstest.Run(t, taskID, shared.TaskContext{}, instr)
	require.True(t, r.Success, r.Msg)
	assert.Equal(t, "AWS EC2 Launch Template version 4 created and set as default", r.Msg)
	assert.Equal(t, "lt-1", r.Data["lt_id"])
	assert.Equal(t, "ami-new", r.Data["image_id"])
	assert.Equal(t, "3", r.Data["previous_default"])
	assert.Equal(t, "4", r.Data["new_version"])
	assert.Equal(t, map[string]any{"LaunchTemplateId": "lt-1", "DefaultVersionNumber": float64(4)}, r.Data["launch_template"])

	describe := rec.Forms("DescribeLaunchTemplateVersions")
	require.Len(t, describe, 1)
	assert.Equal(t, "lt-1", describe[0].Get("LaunchTemplateId"))
	assert.Equal(t, "$Default", describe[0].Get("LaunchTemplateVersion.1"))
	assert.Equal(t, "is-default-version", describe[0].Get("Filter.1.Name"))

	create := rec.Forms("CreateLaunchTemplateVersion")
	require.Len(t, create, 1)
	assert.Equal(t, "3", create[0].Get("SourceVersion"))
	assert.Equal(t, "ami-new", create[0].Get("LaunchTemplateData.ImageId"))
	assert.Equal(t, "false", create[0].Get("DryRun"))

	modify := rec.Forms("ModifyLaunchTemplate")
	require.Len(t, modify, 1)
	assert.Equal(t, "4", modify[0].Get("SetDefaultVersion"))
}

func TestExecute_AllFields(t *testing.T) {
	serve(t, map[string]reply{
		"DescribeLaunchTemplateVersions": ok(describeVersions(3)),
		"CreateLaunchTemplateVersion":    ok(createVersionResponse),
		"ModifyLaunchTemplate":           ok(modifyResponse),
	})
	r := awstest.Run(t, taskID, shared.TaskContext{}, instructions())
	require.True(t, r.Success, r.Msg)
	lt, isMap := r.Data["launch_template"].(map[string]any)
	require.True(t, isMap)
	assert.Equal(t, "web", lt["LaunchTemplateName"])
	assert.Equal(t, float64(4), lt["LatestVersionNumber"])
}

func TestExecute_DryRun(t *testing.T) {
	status, body := awstest.EC2DryRun()
	rec := serve(t, map[string]reply{
		"DescribeLaunchTemplateVersions": ok(describeVersions(3)),
		"CreateLaunchTemplateVersion":    {status, body},
	})
	r := awstest.Run(t, taskID, shared.TaskContext{DryRun: true}, instructions())
	require.True(t, r.Success, r.Msg)
	assert.Contains(t, r.Msg, "Dryrun, AWS API returned: ")
	assert.Empty(t, r.Data)
	create := rec.Forms("CreateLaunchTemplateVersion")
	require.Len(t, create, 1)
	assert.Equal(t, "true", create[0].Get("DryRun"))
	assert.Equal(t, 0, rec.Count("ModifyLaunchTemplate"))
}

func TestExecute_Errors(t *testing.T) {
	apiErr := reply{http.StatusBadRequest, awstest.EC2Error("InvalidLaunchTemplateId.NotFound", "The specified launch template does not exist")}
	denied := reply{http.StatusForbidden, awstest.EC2Error("UnauthorizedOperation", "You are not authorized to perform this operation.")}
	cases := []struct {
		name    string
		dryRun  bool
		replies map[string]reply
		wantMsg string
	}{
		{
			name:    "describe fails",
			replies: map[string]reply{"DescribeLaunchTemplateVersions": apiErr},
			wantMsg: "failed to describe default launch template version for lt-1: ",
		},
		{
			name:    "no default version",
			replies: map[string]reply{"DescribeLaunchTemplateVersions": ok(describeVersions())},
			wantMsg: "failed to obtain default launch template version for lt-1",
		},
		{
			name:    "version below one",
			replies: map[string]reply{"DescribeLaunchTemplateVersions": ok(describeVersions(0))},
			wantMsg: "launch template version for lt-1 is less than 1, aborting",
		},
		{
			name:    "create fails",
			replies: map[string]reply{"DescribeLaunchTemplateVersions": ok(describeVersions(3)), "CreateLaunchTemplateVersion": denied},
			wantMsg: "failed to create new launch template version: ",
		},
		{
			name:    "dry run with real error",
			dryRun:  true,
			replies: map[string]reply{"DescribeLaunchTemplateVersions": ok(describeVersions(3)), "CreateLaunchTemplateVersion": denied},
			wantMsg: "failed to create new launch template version: ",
		},
		{
			name:    "create returns no version",
			replies: map[string]reply{"DescribeLaunchTemplateVersions": ok(describeVersions(3)), "CreateLaunchTemplateVersion": ok(createVersionNoVersion)},
			wantMsg: "launch template version not returned",
		},
		{
			name:    "create returns no version number",
			replies: map[string]reply{"DescribeLaunchTemplateVersions": ok(describeVersions(3)), "CreateLaunchTemplateVersion": ok(createVersionNoNumber)},
			wantMsg: "launch template version not returned",
		},
		{
			name: "modify fails",
			replies: map[string]reply{
				"DescribeLaunchTemplateVersions": ok(describeVersions(3)),
				"CreateLaunchTemplateVersion":    ok(createVersionResponse),
				"ModifyLaunchTemplate":           denied,
			},
			wantMsg: "failed to set new launch template version as default: ",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := serve(t, tc.replies)
			r := awstest.Run(t, taskID, shared.TaskContext{DryRun: tc.dryRun}, instructions())
			assert.False(t, r.Success)
			assert.Empty(t, r.OnFail)
			assert.Contains(t, r.Msg, tc.wantMsg)
			assert.Empty(t, r.Data)
			if tc.wantMsg == "launch template version not returned" {
				assert.Equal(t, 0, rec.Count("ModifyLaunchTemplate"))
			}
		})
	}
}

func TestExecute_MissingEnvFile(t *testing.T) {
	awstest.Serve(t, awstest.Reject)
	instr := instructions()
	instr["env"] = t.TempDir() + "/missing.env"
	r := awstest.Run(t, taskID, shared.TaskContext{}, instr)
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to create AWS client")
}

func TestExecute_BadInstructions(t *testing.T) {
	awstest.Serve(t, awstest.Reject)
	ctx := shared.TaskContext{Task: taskID, Instructions: []byte(`{"fields": "ImageId"}`)}
	r := shared.TaskRegistry[taskID](ctx).Execute()
	assert.False(t, r.Success)
	assert.Contains(t, r.Msg, "failed to deserialize data")
}
