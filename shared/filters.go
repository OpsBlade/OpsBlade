// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package shared

type Filter struct {
	Name   string   `yaml:"name" json:"name"`
	Values []string `yaml:"values" json:"values"`
}
