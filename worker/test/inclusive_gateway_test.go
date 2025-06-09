package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type inclusiveGatewayDelegate struct {
}

func (d inclusiveGatewayDelegate) CreateProcessCmd() (engine.CreateProcessCmd, error) {
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

func (d inclusiveGatewayDelegate) Delegate(delegator worker.Delegator) error {
	delegator.EvaluateInclusiveGateway("fork", d.evaluateFork)
	return nil
}

func (d inclusiveGatewayDelegate) evaluateFork(jc worker.JobContext) ([]string, error) {
	return []string{"endEventA", "endEventC"}, nil
}

func TestInclusiveGatewayProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	inclusiveGatewayProcess, err := w.Register(&inclusiveGatewayDelegate{})
	if err != nil {
		t.Fatalf("failed to register process: %v", err)
	}

	processInstance, err := inclusiveGatewayProcess.CreateProcessInstance(worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("fork")
	piAssert.ExecuteJob()

	piAssert.IsEnded()
}
