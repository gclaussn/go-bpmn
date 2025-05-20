package mem

import (
	"io"
	"os"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
)

func mustCreateEngine(t *testing.T) engine.Engine {
	e, err := New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	return e
}

func mustInsertEntities(t *testing.T, e engine.Engine, entities []any) {
	memEngine := e.(*memEngine)

	ctx := memEngine.wlock()
	defer memEngine.unlock()

	for _, entity := range entities {
		switch entity := entity.(type) {
		case *internal.ElementInstanceEntity:
			ctx.ElementInstances().Insert(entity)
		case *internal.ElementEntity:
			ctx.Elements().Insert([]*internal.ElementEntity{entity})
		case *internal.IncidentEntity:
			ctx.Incidents().Insert(entity)
		case *internal.JobEntity:
			ctx.Jobs().Insert(entity)
		case *internal.ProcessEntity:
			ctx.Processes().Insert(entity)
		case *internal.ProcessInstanceEntity:
			ctx.ProcessInstances().Insert(entity)
		case *internal.TaskEntity:
			ctx.Tasks().Insert(entity)
		case *internal.VariableEntity:
			ctx.Variables().Insert(entity)
		default:
			t.Fatalf("unsupported entity type %T", entity)
		}
	}
}

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

func mustUpdateEntities(t *testing.T, e engine.Engine, entities []any) {
	memEngine := e.(*memEngine)

	ctx := memEngine.wlock()
	defer memEngine.unlock()

	for _, entity := range entities {
		switch entity := entity.(type) {
		case *internal.JobEntity:
			ctx.Jobs().Update(entity)
		case *internal.TaskEntity:
			ctx.Tasks().Update(entity)
		default:
			t.Fatalf("unsupported entity type %T", entity)
		}
	}
}
