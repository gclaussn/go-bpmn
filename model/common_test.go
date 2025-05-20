package model

import (
	"os"
	"testing"
)

func mustCreateModel(t *testing.T, fileName string) *Model {
	fileName = "../test/bpmn/" + fileName

	bpmnFile, err := os.Open(fileName)
	if err != nil {
		t.Fatalf("failed to open BPMN file %s: %v", fileName, err)
	}

	defer bpmnFile.Close()

	model, err := New(bpmnFile)
	if err != nil {
		t.Fatalf("failed to parse BPMN XML: %v", err)
	}

	return model
}
