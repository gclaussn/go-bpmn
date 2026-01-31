package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type exclusiveGateway struct {
}

func (h exclusiveGateway) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("gateway/exclusive.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "exclusiveTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (h exclusiveGateway) Handle(mux worker.JobMux) error {
	mux.EvaluateExclusiveGateway("fork", h.evaluateFork)
	return nil
}

func (h exclusiveGateway) evaluateFork(_ worker.JobContext) (string, error) {
	return "join", nil
}

type exclusiveGatewayGeneric struct {
}

func (h exclusiveGatewayGeneric) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("gateway/exclusive.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "exclusiveTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (h exclusiveGatewayGeneric) Handle(mux worker.JobMux) error {
	mux.ExecuteAny("fork", h.evaluateFork)
	return nil
}

func (h exclusiveGatewayGeneric) evaluateFork(_ worker.JobContext) (*engine.JobCompletion, error) {
	return &engine.JobCompletion{
		ExclusiveGatewayDecision: "join",
	}, nil
}

func TestExclusiveGatewayProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	exclusiveGatewayProcess, err := w.Register(exclusiveGateway{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := exclusiveGatewayProcess.CreateProcessInstance(context.Background(), worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("fork")
	piAssert.ExecuteJob()

	piAssert.IsCompleted()
}

func TestExclusiveGatewayGenericProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	exclusiveGatewayProcess, err := w.Register(exclusiveGatewayGeneric{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := exclusiveGatewayProcess.CreateProcessInstance(context.Background(), worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("fork")
	piAssert.ExecuteJob()

	piAssert.IsCompleted()
}
