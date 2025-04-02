// Copyright (c) 2025 Tenebris Technologies Inc.
// This software is licensed under the MIT License (see LICENSE for details).

package main

import (
	"fmt"
	"os"

	"github.com/spf13/pflag"

	"github.com/OpsBlade/OpsBlade/workflow"
)

//goland:noinspection GoUnusedConst
const (
	PROGNAME = "OpsBlade"
	VERSION  = "0.1.6"
)

// This program serves both as a CLI to execute workflows from a YAML file or stdin, and as an example of
// how to use the autoadmin workflow package.
func main() {
	var stdin bool
	var dryrun bool
	var json bool
	var debug bool

	// Use the pflag package to parse command line arguments
	pflag.BoolVarP(&dryrun, "dryrun", "d", false, "Dry run")
	pflag.BoolVarP(&stdin, "stdin", "s", false, "Read from stdin")
	pflag.BoolVarP(&json, "json", "j", false, "Output JSON")
	pflag.BoolVarP(&debug, "debug", "v", false, "Debug mode")
	pflag.Usage = usage
	pflag.Parse()

	fmt.Printf("%s v%s\n\n", PROGNAME, VERSION)

	// Require stdin or a filename, but not both
	var yamlFilename = ""
	if stdin {
		if len(pflag.Args()) > 0 {
			fmt.Println("Error: Cannot use both -stdin and a filename argument")
			usage()
			os.Exit(1)
		}
	} else {
		if len(pflag.Args()) < 1 {
			fmt.Println("Error: Either a filename or --stdin must be provided")
			usage()
			os.Exit(1)
		}
		yamlFilename = pflag.Arg(0)
	}

	// Create a new workflow
	w := workflow.New(
		workflow.WithJSON(json),
		workflow.WithDryRun(dryrun),
		workflow.WithDebug(debug))

	// Load the workflow. If the string is empty, Load will read from stdin
	err := w.Load(yamlFilename)
	if err != nil {
		fmt.Printf("Error: Unable to load tasks: %v\n", err)
		os.Exit(1)
	}

	// Execute the workflow
	result := w.Execute()
	if result {
		fmt.Println("All tasks complete. Exiting with code 0.")
		os.Exit(0)
	}
	fmt.Println("Terminating due to failed task. Exiting with code 1.")
	os.Exit(1)
}

// usage prints the usage message
func usage() {
	fmt.Printf("\nUse: %s [filename.yaml] [--stdin] [--json] [--dryrun] [--debug]\n", PROGNAME)
}
