package client

import (
	"io"
	"os"
	"testing"
)

func mustReadBpmnFile(t *testing.T, fileName string) string {
	bpmnFile, err := os.Open("../../test/bpmn/" + fileName)
	if err != nil {
		t.Fatalf("failed to open BPMN file: %v", err)
	}

	defer bpmnFile.Close()

	b, err := io.ReadAll(bpmnFile)
	if err != nil {
		t.Fatalf("failed to read BPMN XML: %v", err)
	}

	return string(b)
}
