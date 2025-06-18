package cli

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/mem"
)

func mustCreateEngine(t *testing.T) engine.Engine {
	e, err := mem.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	return e
}

func mustExecute(t *testing.T, e engine.Engine, args []string) {
	rootCmd := newRootCmd(&Cli{engine: e})
	rootCmd.PersistentPostRun = nil

	rootCmd.SetArgs(args)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("failed to execute %v: %v", args, err)
	}
}
