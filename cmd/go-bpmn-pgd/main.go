/*
go-bpmn-pgd is a daemon, running a PostgreSQL based process engine that is accessible via HTTP.

Usage:

	-create-api-key
		create a new API key
	-create-encryption-key
		create a new encryption key - used for GO_BPMN_ENCRYPTION_KEYS
	-env value
		set environment variables
	-env-file value
		read in a file of environment variables
	-list-conf
		list configuration
	-list-conf-opts
		list configuration options
	-secret-id string
		secret ID, required when creating a new API key
	-version
		show version
*/
package main

import (
	"log"
	"os"

	"github.com/gclaussn/go-bpmn/daemon"
)

func main() {
	log.SetOutput(os.Stdout)

	code := daemon.RunPg(os.Args[1:])
	os.Exit(code)
}
