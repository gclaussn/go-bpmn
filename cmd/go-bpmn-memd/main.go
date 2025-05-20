/*
go-bpmn-memd is a daemon, running an in-memory process engine that is accessible via HTTP.

Usage:

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

	code := daemon.RunMem(os.Args[1:])
	os.Exit(code)
}
