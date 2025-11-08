package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type escalationBounaryEventDelegate struct {
}

func (d escalationBounaryEventDelegate) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("event/escalation-boundary.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "escalationBoundaryTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (d escalationBounaryEventDelegate) Delegate(delegator worker.Delegator) error {
	delegator.SetEscalationCode("escalationBoundaryEvent", d.setEscalationCode)
	delegator.Execute("serviceTask", d.execute)
	return nil
}

func (d escalationBounaryEventDelegate) setEscalationCode(jc worker.JobContext) (string, error) {
	return "TEST_CODE", nil
}

func (d escalationBounaryEventDelegate) execute(jc worker.JobContext) error {
	return worker.NewBpmnEscalation("TEST_CODE")
}

func TestEscalationBoundaryEventProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	process, err := w.Register(&escalationBounaryEventDelegate{})
	if err != nil {
		t.Fatalf("failed to register process: %v", err)
	}

	processInstance, err := process.CreateProcessInstance(context.Background(), worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("escalationBoundaryEvent")
	piAssert.ExecuteJob()

	piAssert.IsWaitingAt("serviceTask")
	piAssert.ExecuteJob()

	piAssert.HasPassed("escalationBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()
}
