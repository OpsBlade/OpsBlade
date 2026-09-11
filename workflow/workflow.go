// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package workflow

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/OpsBlade/OpsBlade/notify"
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
	Name     string           `yaml:"name"` // Human-readable workflow name, used in alerts
	Env      string           `yaml:"env"`
	DryRun   bool             `yaml:"dryrun"`
	Debug    bool             `yaml:"debug"`
	JSON     bool             `yaml:"json"`
	Notify   notify.Config    `yaml:"notify"` // Optional workflow-level alert configuration
	Tasks    []map[string]any `yaml:"tasks"`
	callback shared.Callback  `yaml:"-"`
}

// defaultName is used when a workflow read from stdin has no name field
const defaultName = "OpsBlade workflow"

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

	if err = w.Notify.Validate(); err != nil {
		return fmt.Errorf("invalid notify block: %w", err)
	}

	// Default the workflow name so alerts always identify the workflow
	if w.Name == "" {
		if filename == "" {
			w.Name = defaultName
		} else {
			w.Name = filepath.Base(filename)
		}
	}
	return nil
}

// Alert builds the notification for a result. Every outcome produces one; the
// notifier decides which channels, if any, receive it.
func (w *Workflow) Alert(r Result) notify.Alert {
	var severity notify.Severity
	switch r.Outcome {
	case OutcomeWarnings:
		severity = notify.SeverityWarning
	case OutcomeFatal:
		severity = notify.SeverityFatal
	case OutcomeStopped:
		severity = notify.SeverityStopped
	default:
		severity = notify.SeverityCompleted
	}

	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}

	name := w.Name
	if name == "" {
		name = defaultName
	}

	a := notify.Alert{
		Severity: severity,
		Workflow: name,
		Host:     host,
		Started:  r.Started,
	}
	for _, f := range r.Warnings {
		a.Failures = append(a.Failures, failure(notify.SeverityWarning, f))
	}
	if r.Fatal != nil {
		a.Failures = append(a.Failures, failure(notify.SeverityFatal, *r.Fatal))
		a.TasksNotRun = r.TasksTotal - r.TasksRun
	}
	if r.StoppedAt != nil {
		a.Failures = append(a.Failures, failure(notify.SeverityStopped, *r.StoppedAt))
	}
	return a
}

// failure converts a task failure into a notification entry of the given severity
func failure(severity notify.Severity, f TaskFailure) notify.Failure {
	return notify.Failure{Severity: severity, Sequence: f.Sequence, Name: f.Name, Task: f.Task, Msg: f.Msg}
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

// Execute the loaded workflow and return a summary of the outcome.
//
// Each task's on_fail field (warn, fatal, stop; default fatal) decides what a failure
// means. A task may also request a clean stop by returning a result with Stop set.
func (w *Workflow) Execute() Result {
	var err error

	result := Result{
		Outcome:    OutcomeCompleted,
		TasksTotal: len(w.Tasks),
		Started:    time.Now(),
	}

	// Validate on_fail values before running anything so a typo cannot produce a partial run
	for i, rawTask := range w.Tasks {
		taskContext := w.newTaskContext(i+1, rawTask)
		if _, err = parseOnFail(rawTask); err != nil {
			taskContext.OnFail = shared.OnFailFatal
			w.finish(&result, taskContext, taskContext.Error("Invalid on_fail value", err))
			return result
		}
	}

	// Iterate over the tasks
	for i, rawTask := range w.Tasks {
		taskContext := w.newTaskContext(i+1, rawTask)
		result.TasksRun++

		if taskContext.Task == "" {
			if !w.finish(&result, taskContext, taskContext.Error("Task type is missing or not a string", nil)) {
				return result
			}
			continue
		}

		if skip, _ := rawTask["skip"].(bool); skip {
			r := taskContext.Result(true, "Task skipped", nil)
			r.MessageType = "task_skipped"
			r.Outcome = "skipped"
			if !w.finish(&result, taskContext, r) {
				return result
			}
			continue
		}

		// Obtain the task constructor from the registry
		constructor, ok := shared.TaskRegistry[taskContext.Task]
		if !ok {
			if !w.finish(&result, taskContext, taskContext.Error(fmt.Sprintf("Invalid task: %s", taskContext.Task), nil)) {
				return result
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
			if !w.finish(&result, taskContext, taskContext.Error("Failed to serialize task", err)) {
				return result
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
		taskResult := task.Execute()

		// Force the message type
		taskResult.MessageType = "task_stop"

		// Copy returned data to variables
		if !taskResult.NoVars {
			for key, value := range taskResult.Data {
				shared.SetVar(key, value)
			}
		}

		// Process the result and stop if necessary
		if !w.finish(&result, taskContext, taskResult) {
			return result
		}
	}
	return result
}

// newTaskContext builds the context for a task from its raw map and the workflow defaults.
// on_fail is copied as-is; parseOnFail validates it.
func (w *Workflow) newTaskContext(sequence int, rawTask map[string]any) shared.TaskContext {
	name, _ := rawTask["name"].(string)
	taskType, _ := rawTask["task"].(string)
	errorMessage, _ := rawTask["error_message"].(string)
	onFail, _ := parseOnFail(rawTask)
	return shared.TaskContext{
		Env:          w.Env,
		DryRun:       w.DryRun,
		Debug:        w.Debug,
		Name:         name,
		Task:         taskType,
		Sequence:     sequence,
		Instructions: make([]byte, 0),
		ErrorMessage: errorMessage,
		OnFail:       onFail,
	}
}

// parseOnFail returns the task's on_fail value, defaulting to fatal, or an error if it is not valid
func parseOnFail(rawTask map[string]any) (string, error) {
	raw, present := rawTask["on_fail"]
	if !present || raw == nil {
		return shared.OnFailFatal, nil
	}
	value, ok := raw.(string)
	if !ok {
		return shared.OnFailFatal, fmt.Errorf("on_fail must be a string, got %T", raw)
	}
	switch value {
	case shared.OnFailWarn, shared.OnFailFatal, shared.OnFailStop:
		return value, nil
	case "":
		return shared.OnFailFatal, nil
	default:
		return shared.OnFailFatal, fmt.Errorf("on_fail must be %q, %q, or %q, got %q",
			shared.OnFailWarn, shared.OnFailFatal, shared.OnFailStop, value)
	}
}

// classify maps a task result and its on_fail setting to an outcome label.
// A result may carry its own on_fail, which takes precedence; tasks use this to
// classify an expected condition differently from an error.
func classify(r shared.TaskResult, onFail string) string {
	if r.OnFail != "" {
		onFail = r.OnFail
	}
	switch {
	case r.Outcome == "skipped":
		return "skipped"
	case r.Success && r.Stop:
		return "stop"
	case r.Success:
		return "success"
	case onFail == shared.OnFailWarn:
		return "warning"
	case onFail == shared.OnFailStop:
		return "stop"
	default:
		return "fatal"
	}
}

// finish applies on_fail to a task result, reports it, updates the workflow result,
// and returns true if the workflow should continue
func (w *Workflow) finish(result *Result, c shared.TaskContext, r shared.TaskResult) bool {
	outcome := classify(r, c.OnFail)
	r.Outcome = outcome

	// The recorded failure carries the task's own message; the console copy also says what happens next
	failure := &TaskFailure{Sequence: c.Sequence, Name: c.Name, Task: c.Task, Msg: r.Msg}
	if outcome == "warning" {
		if r.OnFail != "" {
			r.Msg += " (continuing: warn)"
		} else {
			r.Msg += " (continuing: on_fail is warn)"
		}
	}

	// Report the result. A callback may veto continuation.
	if !w.taskEnd(r) && outcome != "stop" && outcome != "fatal" {
		outcome = "fatal"
		failure.Msg = "halted by callback"
	}

	switch outcome {
	case "warning":
		result.Warnings = append(result.Warnings, *failure)
		result.Outcome = OutcomeWarnings
		return true
	case "stop":
		result.Outcome = OutcomeStopped
		result.StoppedAt = failure
		return false
	case "fatal":
		result.Outcome = OutcomeFatal
		result.Fatal = failure
		return false
	default:
		return true
	}
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

// taskEnd either passes the results to the callback function or prints them to stdout.
// It returns false only when a callback vetoes continuation; the engine decides
// what a failure means via on_fail.
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

	return true
}
