package test

import (
	"io"
	"os"
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

func readBpmnFile(fileName string) (string, error) {
	bpmnFile, err := os.Open("../../test/bpmn/" + fileName)
	if err != nil {
		return "", err
	}

	defer bpmnFile.Close()

	b, err := io.ReadAll(bpmnFile)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
