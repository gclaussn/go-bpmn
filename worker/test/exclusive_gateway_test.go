package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type exclusiveGatewayDelegate struct {
}

func (d exclusiveGatewayDelegate) CreateProcessCmd() (engine.CreateProcessCmd, error) {
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

func (d exclusiveGatewayDelegate) Delegate(delegator worker.Delegator) error {
	delegator.EvaluateExclusiveGateway("fork", d.evaluateFork)
	return nil
}

func (d exclusiveGatewayDelegate) evaluateFork(_ worker.JobContext) (string, error) {
	return "join", nil
}

func TestExclusiveGatewayProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	exclusiveGatewayProcess, err := w.Register(&exclusiveGatewayDelegate{})
	if err != nil {
		t.Fatalf("failed to register process: %v", err)
	}

	processInstance, err := exclusiveGatewayProcess.CreateProcessInstance(worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("fork")
	piAssert.ExecuteJob()

	piAssert.IsEnded()
}
