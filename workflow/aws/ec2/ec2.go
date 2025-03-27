// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package ec2

import (
	_ "github.com/OpsBlade/OpsBlade/workflow/aws/ec2/ami/create"
	_ "github.com/OpsBlade/OpsBlade/workflow/aws/ec2/ami/list"
	_ "github.com/OpsBlade/OpsBlade/workflow/aws/ec2/ami/wait"
	_ "github.com/OpsBlade/OpsBlade/workflow/aws/ec2/instance/list"
	_ "github.com/OpsBlade/OpsBlade/workflow/aws/ec2/instance/start"
	_ "github.com/OpsBlade/OpsBlade/workflow/aws/ec2/instance/stop"
	_ "github.com/OpsBlade/OpsBlade/workflow/aws/ec2/instance/wait"
	_ "github.com/OpsBlade/OpsBlade/workflow/aws/ec2/launchTemplate/changeImage"
	_ "github.com/OpsBlade/OpsBlade/workflow/aws/ec2/sg/list"
)
