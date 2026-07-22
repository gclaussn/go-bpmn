/*
go-bpmn is a CLI for interacting with a process engine via HTTP.

Usage:

	go-bpmn [flags]
	go-bpmn [command]

Available Commands:

	completion       Generate the autocompletion script for the specified shell
	element          Query elements
	element-instance Manage and query element instances
	help             Help about any command
	incident         Resolve and query incidents
	job              Manage and query jobs
	message          Send messages, query messages and subscriptions
	process          Manage and query processes
	process-instance Manage and query process instances
	set-time         Increases the engine's time for testing purposes
	signal           Send signals and query signal subscriptions
	task             Manage and query tasks
	user-task        Update and query user tasks
	variable         Query variables
	version          Show version

Flags:

	    --debug              Log HTTP requests and responses
	-h, --help               help for go-bpmn
	    --timeout duration   Time limit for requests made by the HTTP client (default 40s)
	    --url string         HTTP server URL
	    --worker-id string   Worker ID (default "go-bpmn")

Use "go-bpmn [command] --help" for more information about a command.
*/
package main

import (
	"os"

	"github.com/gclaussn/go-bpmn/cli"
)

var (
	version = "unknown-version"
)

func main() {
	cli := cli.New(version)

	rootCmd := cli.RootCmd()
	rootCmd.SetOut(os.Stdout)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
