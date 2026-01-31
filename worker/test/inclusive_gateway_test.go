package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type inclusiveGateway struct {
}

func (h inclusiveGateway) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("gateway/inclusive.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "inclusiveTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (h inclusiveGateway) Handle(mux worker.JobMux) error {
	mux.EvaluateInclusiveGateway("fork", h.evaluateFork)
	return nil
}

func (h inclusiveGateway) evaluateFork(jc worker.JobContext) ([]string, error) {
	return []string{"endEventA", "endEventC"}, nil
}

func TestInclusiveGatewayProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	inclusiveGatewayProcess, err := w.Register(inclusiveGateway{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := inclusiveGatewayProcess.CreateProcessInstance(context.Background(), worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("fork")
	piAssert.ExecuteJob()

	piAssert.IsCompleted()
}
