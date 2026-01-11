package internal

import (
	"os"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

func mustCreateGraph(t *testing.T, fileName string, bpmnProcessId string) graph {
	fileName = "../../test/bpmn/" + fileName

	bpmnFile, err := os.Open(fileName)
	if err != nil {
		t.Fatalf("failed to open BPMN file %s: %v", fileName, err)
	}

	defer bpmnFile.Close()

	bpmnModel, err := model.New(bpmnFile)
	if err != nil {
		t.Fatalf("failed to parse BPMN XML: %v", err)
	}

	bpmnElements := bpmnModel.ElementsByProcessId(bpmnProcessId)
	if len(bpmnElements) == 0 {
		t.Fatalf("failed to collect elements of BPMN process %s", bpmnProcessId)
	}

	elements := make([]*ElementEntity, len(bpmnElements))
	for i, bpmnElement := range bpmnElements {
		element := ElementEntity{
			Id: int32(i + 1),

			BpmnElementId:   bpmnElement.Id,
			BpmnElementName: pgtype.Text{String: bpmnElement.Name, Valid: bpmnElement.Name != ""},
			BpmnElementType: bpmnElement.Type,
		}

		elements[i] = &element
	}

	graph, err := newGraph(bpmnModel, bpmnElements, elements)
	if err != nil {
		t.Fatalf("failed to create execution graph: %v", err)
	}

	return graph
}

func mustValidateProcess(t *testing.T, fileName string, customizers ...func(processElement *model.Element)) []engine.ErrorCause {
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

	if len(model.Definitions.Processes) == 0 {
		t.Fatal("model has no process")
	}

	processElement := model.Definitions.Processes[0]

	for _, customizer := range customizers {
		customizer(processElement)
	}

	bpmnElements := model.ElementsByProcessId(processElement.Id)

	causes, err := validateProcess(bpmnElements)
	if err != nil {
		t.Fatalf("failed to validate BPMN process: %v", err)
	}

	return causes
}
