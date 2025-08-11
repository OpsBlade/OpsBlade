// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package workflow

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"

	"github.com/OpsBlade/OpsBlade/shared"

	// Import all task packages so that they register.
	// Each task's init function registers the task with the shared.TaskRegistry and includes
	// a constructor function that returns a shared.Task interface. This allows the workflow
	// to call the constructor function to obtain a task object that can be executed.
	// _ must be used to import the package without using it, as the package is only needed for its init function.
	// This avoids package name conflicts and the "package imported but not used" error.
	// Note that these packages below import their sub-packages
	_ "github.com/OpsBlade/OpsBlade/workflow/aws"
	_ "github.com/OpsBlade/OpsBlade/workflow/cmd"
	_ "github.com/OpsBlade/OpsBlade/workflow/example" // for demo and testing purposes
	_ "github.com/OpsBlade/OpsBlade/workflow/file"
	_ "github.com/OpsBlade/OpsBlade/workflow/jira"
	_ "github.com/OpsBlade/OpsBlade/workflow/misc"
	_ "github.com/OpsBlade/OpsBlade/workflow/slack"
	_ "github.com/OpsBlade/OpsBlade/workflow/variables"
)

type Workflow struct {
	Env      string           `yaml:"env"`
	DryRun   bool             `yaml:"dryrun"`
	Debug    bool             `yaml:"debug"`
	JSON     bool             `yaml:"json"`
	Tasks    []map[string]any `yaml:"tasks"`
	callback shared.Callback  `yaml:"-"`
}

// Option is used for the golang options pattern
type Option func(*Workflow)

// New creates a new Workflow applying any provided options
func New(options ...Option) *Workflow {
	w := &Workflow{
		DryRun:   false,
		Debug:    false,
		JSON:     false,
		Env:      "",
		callback: nil,
		Tasks:    make([]map[string]any, 0),
	}
	for _, opt := range options {
		opt(w)
	}
	return w
}

// WithCallback sets a callback function on the Workflow
//
//goland:noinspection GoUnusedExportedFunction
func WithCallback(cb shared.Callback) Option {
	return func(w *Workflow) {
		w.callback = cb
	}
}

// WithJSON sets JSON output on the Workflow
// Note that if "json" is present in the workflow, it will override this setting
//
//goland:noinspection GoUnusedExportedFunction
func WithJSON(b bool) Option {
	return func(w *Workflow) {
		w.JSON = b
	}
}

// WithDebug sets Debug on the Workflow
// Note that if "debug" is present in the workflow, it will override this setting
//
//goland:noinspection GoUnusedExportedFunction
func WithDebug(b bool) Option {
	return func(w *Workflow) {
		w.Debug = b
	}
}

// WithDryRun sets Debug on the Workflow
// Note that if "dryrun" is present in the workflow, it will override this setting
//
//goland:noinspection GoUnusedExportedFunction
func WithDryRun(b bool) Option {
	return func(w *Workflow) {
		w.DryRun = b
	}
}

// Load reads a task configuration from a file or stdin
//
//goland:noinspection GoUnusedExportedFunction
func (w *Workflow) Load(filename string) error {
	var data []byte
	var err error

	// Dump all existing workflow
	w.Tasks = make([]map[string]any, 0)

	// Read the file or stdin
	if filename == "" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("error reading from stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("unable to read file: %w", err)
		}
	}

	// Unmarshal the data
	if err = yaml.Unmarshal(data, &w); err != nil {
		return fmt.Errorf("deserialization error: %w", err)
	}
	return nil
}

// AddTask adds a task to the configuration
//
//goland:noinspection GoUnusedExportedFunction
func (w *Workflow) AddTask(task map[string]any) {
	w.Tasks = append(w.Tasks, task)
}

// AddTaskJSON adds a task in JSON format to the configuration
//
//goland:noinspection GoUnusedExportedFunction
func (w *Workflow) AddTaskJSON(task []byte) error {
	var taskMap map[string]any
	if err := json.Unmarshal(task, &taskMap); err != nil {
		return fmt.Errorf("deserialization failure: %w", err)
	}
	w.AddTask(taskMap)
	return nil
}

// AddTaskYAML adds a task in YAML format to the configuration
//
//goland:noinspection GoUnusedExportedFunction
func (w *Workflow) AddTaskYAML(task []byte) error {
	var taskMap map[string]any
	if err := yaml.Unmarshal(task, &taskMap); err != nil {
		return fmt.Errorf("deserialization failure: %w", err)
	}
	w.AddTask(taskMap)
	return nil
}

// Execute the loaded workflow
//
//goland:noinspection GoUnusedExportedFunction
func (w *Workflow) Execute() bool {
	var err error
	var count int

	// Create a task context, defaulting to global file settings
	var taskContext = shared.TaskContext{
		Env:          w.Env,
		DryRun:       w.DryRun,
		Debug:        w.Debug,
		Instructions: make([]byte, 0),
	}

	// Iterate over the tasks
	for _, rawTask := range w.Tasks {
		count++
		taskName, ok := rawTask["name"].(string)
		if !ok {
			taskName = ""
		}

		taskType, ok := rawTask["task"].(string)
		if !ok {
			taskType = ""
		}

		skip, ok := rawTask["skip"].(bool)
		if !ok {
			skip = false
		}

		errorMessage, ok := rawTask["error_message"].(string)
		if !ok {
			errorMessage = ""
		}

		taskContext.Name = taskName
		taskContext.Task = taskType
		taskContext.Sequence = count
		taskContext.ErrorMessage = errorMessage

		if taskType == "" {
			if w.taskEnd(taskContext.Error(fmt.Sprintf("%s: Task type is missing or not a string\n", taskContext.String()), nil)) {
				return false
			}
			continue
		}

		if skip {
			r := taskContext.Result(true, "Task skipped", nil)
			r.MessageType = "task_skipped"
			if !w.taskEnd(r) {
				break
			}
			continue
		}

		// Obtain the task constructor from the registry
		constructor, ok := shared.TaskRegistry[taskType]
		if !ok {
			if !w.taskEnd(taskContext.Error(fmt.Sprintf("Invalid task: %s", taskType), nil)) {
				return false
			}
			continue
		}

		// Tasks can have different structures, so they are initial deserialized into a map[string]any
		// to obtain information such as the task name and type. To make it easier for individual tasks,
		// the raw task is then serialized into a byte slice and passed to the task as a single field.
		// This allows the task to deserialize the raw task into its own struct rather than have to deal
		// with the raw map[string]any.
		taskContext.Instructions, err = json.Marshal(rawTask)
		if err != nil {
			if !w.taskEnd(taskContext.Error("Failed to serialize task", err)) {
				return false
			}
			continue
		}

		// Send the task start information
		w.taskStart(shared.TaskInfo{
			MessageType:  "task_start",
			Sequence:     taskContext.Sequence,
			Name:         taskContext.Name,
			Task:         taskContext.Task,
			Instructions: rawTask,
			Debug:        taskContext.Debug,
		})

		// Call the task's constructor, which returns an object that implements the
		// shared.Task interface
		task := constructor(taskContext)

		// Execute the task
		result := task.Execute()

		// Force the message type
		result.MessageType = "task_stop"

		// Copy returned data to variables
		if !result.NoVars {
			for key, value := range result.Data {
				shared.SetVar(key, value)
			}
		}

		// Process the result and break if necessary
		if !w.taskEnd(result) {
			return false
		}
	}
	return true
}

// Dump pretty-prints the loaded workflow
func (w *Workflow) Dump() {
	fmt.Printf("Global dryrun: %t\n", w.DryRun)
	fmt.Printf("Global debug: %t\n", w.Debug)
	for i, task := range w.Tasks {
		data, err := json.MarshalIndent(task, "", "  ")
		if err != nil {
			fmt.Printf("Failed to marshal task %d: %v\n", i+1, err)
			continue
		}
		fmt.Printf("Task %d:\n%s\n\n", i+1, string(data))
	}
}

// taskStart either passes the task information to the startCallback function or prints them to stdout
func (w *Workflow) taskStart(task shared.TaskInfo) bool {

	// If a callback function is set, pass it the task information
	if w.callback != nil {
		return w.callback.OnStart(task)
	}

	// Output to the console
	if w.JSON {
		fmt.Println(task.SerializePretty())
	} else {
		fmt.Println(task.String())
	}
	fmt.Println()

	// Allow the task to continue
	return true
}

// taskEnd either passes the results to the callback function or prints them to stdout
func (w *Workflow) taskEnd(result shared.TaskResult) bool {

	// If a callback function is set, pass it the results
	if w.callback != nil {
		return w.callback.OnStop(result)
	}

	// Output to the console
	if w.JSON {
		fmt.Println(result.SerializePretty())
	} else {
		fmt.Println(result.String())
	}
	fmt.Println()

	// Only continue if there was success
	return result.Success
}
