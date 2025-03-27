// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

import (
	"fmt"
	"strings"
)

func DryRunErrCheck(err error) bool {
	e := strings.ToLower(fmt.Sprintf("%s", err.Error()))
	if strings.Contains(e, "api error dryrunoperation: request would have succeeded") {
		return true
	}
	return false
}
