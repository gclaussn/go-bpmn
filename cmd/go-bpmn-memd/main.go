/*
go-bpmn-memd is a daemon, running an in-memory process engine that is accessible via HTTP.

Usage:

	go-bpmn-memd [flags]
	go-bpmn-memd [command]

Available Commands:

	create-encryption-key Create a new encryption key - used for GO_BPMN_ENCRYPTION_KEYS
	list-conf             List configuration
	list-conf-opts        List configuration options
	run                   Run mem engine daemon
	version               Show version

Flags:

	-e, --env strings        set environment variable
	    --env-file strings   read in a file of environment variables
	-h, --help               help for go-bpmn-memd

Use "go-bpmn-memd [command] --help" for more information about a command.
*/
package main

import (
	"os"

	"github.com/gclaussn/go-bpmn/daemon/memd"
)

var (
	version = "unknown-version"
)

func main() {
	d := memd.New(version)

	rootCmd := d.RootCmd()
	rootCmd.SetOut(os.Stdout)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
