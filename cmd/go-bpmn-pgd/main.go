/*
go-bpmn-pgd is a daemon, running a PostgreSQL based process engine that is accessible via HTTP.

Usage:

	go-bpmn-pgd [flags]
	go-bpmn-pgd [command]

Available Commands:

	api-key               Manage API keys
	create-encryption-key Create a new encryption key - used for GO_BPMN_ENCRYPTION_KEYS
	list-conf             List configuration
	list-conf-opts        List configuration options
	run                   Run pg engine daemon
	version               Show version

Flags:

	-e, --env strings        set environment variable
	    --env-file strings   read in a file of environment variables
	-h, --help               help for go-bpmn-pgd

Use "go-bpmn-pgd [command] --help" for more information about a command.
*/
package main

import (
	"os"

	"github.com/gclaussn/go-bpmn/daemon/pgd"
)

var (
	version = "unknown-version"
)

func main() {
	d := pgd.New(version)

	rootCmd := d.RootCmd()
	rootCmd.SetOut(os.Stdout)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
