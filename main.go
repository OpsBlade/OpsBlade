// Copyright (c) 2025-2026 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/OpsBlade/OpsBlade/app"
	"github.com/OpsBlade/OpsBlade/notify"
	"github.com/OpsBlade/OpsBlade/transcript"
	"github.com/OpsBlade/OpsBlade/workflow"
)

// This program serves both as a CLI to execute workflows from a YAML file or stdin, and as an example of
// how to use the workflow package.
func main() {
	var stdin bool
	var dryrun bool
	var json bool
	var debug bool
	var version bool

	// Use the pflag package to parse command line arguments
	pflag.BoolVarP(&dryrun, "dryrun", "d", false, "Dry run")
	pflag.BoolVarP(&stdin, "stdin", "s", false, "Read from stdin")
	pflag.BoolVarP(&json, "json", "j", false, "Output JSON")
	pflag.BoolVarP(&debug, "debug", "v", false, "Debug mode")
	pflag.BoolVarP(&version, "version", "V", false, "Print version and exit")
	pflag.Usage = usage
	pflag.Parse()

	// Capture stdout and stderr from here on so the run's transcript can be emailed.
	// Output still reaches the terminal. Every exit below goes through exit() so the
	// capture is drained first.
	capture, err := transcript.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: unable to capture output: %v\n", err)
	}

	// Startup banner goes to stderr so stdout stays clean for task output
	fmt.Fprintf(os.Stderr, "%s %s\n%s\n\n", app.Name(), app.Version(), app.Copyright())

	if version {
		buildTime, goVersion := app.BuildInfo()
		fmt.Fprintf(os.Stderr, "%s\n", app.TagLine())
		if buildTime != "" {
			fmt.Fprintf(os.Stderr, "Built: %s\n", buildTime)
		}
		fmt.Fprintf(os.Stderr, "Toolchain: %s\n", goVersion)
		exit(capture, 0)
	}

	// Require stdin or a filename, but not both
	var yamlFilename = ""
	if stdin {
		if len(pflag.Args()) > 0 {
			fmt.Println("Error: Cannot use both -stdin and a filename argument")
			usage()
			exit(capture, 1)
		}
	} else {
		if len(pflag.Args()) < 1 {
			fmt.Println("Error: Either a filename or --stdin must be provided")
			usage()
			exit(capture, 1)
		}
		yamlFilename = pflag.Arg(0)
	}

	// Create a new workflow
	w := workflow.New(
		workflow.WithJSON(json),
		workflow.WithDryRun(dryrun),
		workflow.WithDebug(debug))

	// Load the workflow. If the string is empty, Load will read from stdin
	if err = w.Load(yamlFilename); err != nil {
		fmt.Printf("Error: Unable to load tasks: %v\n", err)
		exit(capture, 1)
	}

	// Execute the workflow
	result := w.Execute()
	fmt.Println(result.Summary())

	// Send the notification the workflow configured. The transcript ends with the
	// summary line above. Notification failures are reported but do not change
	// the exit code.
	alert := w.Alert(result)
	if capture != nil {
		alert.Transcript = capture.Stop()
	}
	n := notify.New(w.Notify,
		notify.WithGlobalEnv(w.Env),
		notify.WithDryRun(w.DryRun),
		notify.WithDebug(w.Debug))
	for _, err := range n.Send(alert) {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
	os.Exit(result.ExitCode())
}

// exit drains the output capture, if any, and exits with the code
func exit(capture *transcript.Transcript, code int) {
	if capture != nil {
		capture.Stop()
	}
	os.Exit(code)
}

// usage prints the usage message
func usage() {
	fmt.Printf("\nUse: %s [filename.yaml] [--stdin] [--json] [--dryrun] [--debug] [--version]\n", app.Name())
}
