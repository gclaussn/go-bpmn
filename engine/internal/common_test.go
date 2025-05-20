package internal

import (
	"os"
	"testing"

	"github.com/gclaussn/go-bpmn/model"
)

func mustCreateGraph(t *testing.T, fileName string, bpmnProcessId string) graph {
	graph, err := mustCreateGraphE(t, fileName, bpmnProcessId)
	if err != nil {
		t.Fatalf("failed to create graph: %v", err)
	}
	return graph
}

func mustCreateGraphE(t *testing.T, fileName string, bpmnProcessId string) (graph, error) {
	fileName = "../../test/bpmn/" + fileName

	bpmnFile, err := os.Open(fileName)
	if err != nil {
		t.Fatalf("failed to open BPMN file %s: %v", fileName, err)
	}

	defer bpmnFile.Close()

	model, err := model.New(bpmnFile)
	if err != nil {
		t.Fatalf("failed to parse BPMN XML: %v", err)
	}

	processElement, err := model.ProcessById(bpmnProcessId)
	if err != nil {
		t.Fatal(err.Error())
	}

	processElements := processElement.AllElements()

	elements := make([]*ElementEntity, len(processElements))
	for i, e := range processElements {
		element := ElementEntity{
			Id: int32(i + 1),

			BpmnElementId:   e.Id,
			BpmnElementName: e.Name,
			BpmnElementType: e.Type,
		}

		elements[i] = &element
	}

	return newGraph(processElements, elements)
}
