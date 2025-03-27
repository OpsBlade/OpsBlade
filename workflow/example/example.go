// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package example

import (
	"encoding/json"

	"github.com/OpsBlade/OpsBlade/shared"
)

// NOTE: This task is an example and is not intended for use in a workflow. If you wish to implement a new task,
// copying this one and using it as a template would be a good starting point.

// Tasks use a modular design to allow for easy addition. Each task is a separate Go package and is imported
// by workflow.go using _ imports. For anyone new to go, this imports the package for side effects only, it can
// not be directly referenced. For that reason, package naming conflicts do not occur.

// When the package is imported, the init() function is automatically executed. This provides the task with an
// opportunity to register itself. If a package containing a task is not imported, it will not register, and thus
// as far as the application is concerned it does not exist. See comments above init() below for more details.

// To be clear, no package directly references any task package. They are only accessed via their registration.

// Each task must implement the shared.Task interface. This interface requires only a struct with a single method,
// Execute(), which returns a shared.TaskResult.

// The task struct is used to satisfy the interface as well as hold information relevant to the task. If you prefer,
// a separate struct could be used. But before you change it, note that task-specific data is passed as a byte slice
// so that each task can deserialize it into its own struct rather than having to deal with a generic map[string]any.

// Note that while storing the passed Context is critical, all other fields are optional, and the task can implement
// any structure that can be represented in the YAML file. However, as demonstrated in this example, in many cases
// supporting functions make using the shared types advantageous.

type Task struct {
	// Task context - this contains the instructions for the task
	Context shared.TaskContext `yaml:"context" json:"context"`

	// Credentials are task specific and could be used to override the global credentials
	Credentials shared.Credentials `yaml:"credentials" json:"credentials"`

	// Filters are intended to be passed into an API. They are name-value pairs. Note that the values are a
	// slice (list) for compatibility with the AWS API.
	Filters []shared.Filter `yaml:"filters" json:"filters"`

	// Select is intended to be used to filter records being returned by an API. Where possible, using filters
	// as an input to the API is preferable.
	Select []shared.SelectCriteria `yaml:"select" json:"select"`

	// Fields is a list of fields to return to the user. Wildcards are supported for slices (lists).
	// For example, "instance.*.instanceId" would return a list of all instance IDs.
	Fields []string `yaml:"fields" json:"fields"`
}

// init registers the task and executes automatically when the task is imported. The registration passes two
// arguments, the task name, and a constructor function that accepts a shared.TaskContext and returns a shared.Task.

// The shared.TaskContext received will be unique to this task and must be saved unless the task is so simple that it
// doesn't require any parameters or credentials.

// Note that share.Task is an interface, so any struct that implements the interface (in this case having an Execute()
// method) can be returned. Note that struct returned by the constructor function is the struct defined above,
// not to be confused with the shared.Task interface. In practical terms, copy this init() function, use Task as your
// struct name, and change the task ID to something unique.
func init() {
	shared.RegisterTask("example", func(context shared.TaskContext) shared.Task {
		return &Task{Context: context}
	})
}

// Execute is the method that satisfies the shared.Task interface. It is called by the workflow engine to execute
// the task. The method returns a shared.TaskResult which contains the result of the task execution.
func (t *Task) Execute() shared.TaskResult {
	var err error

	// Deserialize the task instructions into our task struct
	if err = json.Unmarshal(t.Context.Instructions, t); err != nil {
		return t.Context.Error("failed to deserialize data", err)
	}

	// Resolve input variables. This is a helper function that will iterate over the task's fields and
	// resolve any variables that are found. Variables are denoted by {{var_name}}
	shared.ProcessVars(t)

	// This is helpful to verify that variables have been properly resolved. It dumps to stdout.
	if t.Context.Debug {
		shared.DumpTask(t)
	}

	// Resolve credentials with priority to task credentials, then context credentials
	// This allows credentials to be overridden at the task level.
	// While not optimal, note that t.Context.Credentials contains all credentials for all services
	// used by the current workflow. Be careful not to dump them. The helper functions to dump tasks
	// for debugging suppress the credentials
	creds := shared.NewCredentials(t.Credentials, *t.Context.Credentials)

	// At this point, we have our task information deserialized, variables resolved, and credentials resolved.
	// It's time to perform the actual task. Note that if your task wishes to call various cloud APIs, there are
	// helper functions in services/cloud<name> that can be used to instantiate clients. The "cloud" prefix is
	// used to avoid conflicts with various packages/SDKs.

	someData, err := mockListTask(creds.Example, t.Filters)
	if err != nil {
		// Context has a helper function to return an error as a shared.TaskResult
		return t.Context.Error("failed to obtain mock list", err)
	}

	// Where applicable, you may wish to filter the results an API call and the fields that are provided
	// to the user
	// Iterate over the instances and apply selection criteria
	var selected bool
	var wantedItems []any
	for _, item := range someData.SomeList {

		// ApplySelectionCriteria will return true if the item meets the criteria
		// If there are no criteria, it will always return true
		selected, err = shared.ApplySelectionCriteria(item, t.Select)
		if err != nil {
			return t.Context.Error("failed applying selection criteria", err)
		}

		// If the item meets the criteria, add selected fields to the wantedItems list
		// If no fields are specified, the entire item is returned
		if selected {
			wantedItems = append(wantedItems, shared.SelectFields(item, t.Fields))
		}
	}

	// Any data for the user needs to be returned in the result object. The Context.Result() helper function
	// will accept any type to easily allow nil results, but a map with a string key should be returned so that
	// variables can be extracted.
	data := map[string]any{
		"mock_data":           wantedItems,
		"mock_api_items":      len(someData.SomeList),
		"mock_selected_items": len(wantedItems)}

	// Return the result
	return t.Context.Result(
		true,
		"example list",
		data)
}

// mockTaskResponse is a mock response structure that simulates the response from an API call after it is deserialized.
type mockTaskResponse struct {
	SomeList []map[string]any `json:"someList"`
}

// mockListTask is a mock function that simulates an API call. In this example we'll ignore credentials and
// not use the filters.
func mockListTask(_ shared.ExampleCreds, _ []shared.Filter) (mockTaskResponse, error) {
	return mockTaskResponse{
		SomeList: []map[string]any{
			{"name": "item1", "id": 1, "paid": "yes", "tags": map[string]string{"tag1": "tag_value1", "tag2": "yes"}},
			{"name": "item2", "id": 2, "paid": "no", "tags": map[string]string{"tag1": "tag_value2", "tag2": "yes"}},
			{"name": "item3", "id": 3, "paid": "yes", "tags": map[string]string{"tag1": "tag_value3", "tag2": "yes"}},
			{"name": "item4", "id": 4, "paid": "no", "tags": map[string]string{"tag1": "tag_value4", "tag2": "yes"}},
			{"name": "item5", "id": 5, "paid": "yes", "tags": map[string]string{"tag1": "tag_value5", "tag2": "yes"}},
		}}, nil
}
