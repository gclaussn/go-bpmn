package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type errorBounaryEventDelegate struct {
}

func (d errorBounaryEventDelegate) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("event/error-boundary-event.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "errorBoundaryEventTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (d errorBounaryEventDelegate) Delegate(delegator worker.Delegator) error {
	delegator.SetErrorCode("errorBoundaryEvent", d.setErrorCode)
	delegator.Execute("serviceTask", d.execute)
	return nil
}

func (d errorBounaryEventDelegate) setErrorCode(jc worker.JobContext) (string, error) {
	return "TEST_CODE", nil
}

func (d errorBounaryEventDelegate) execute(jc worker.JobContext) error {
	return worker.NewBpmnError("TEST_CODE")
}

func TestErrorBoundaryEventProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	process, err := w.Register(&errorBounaryEventDelegate{})
	if err != nil {
		t.Fatalf("failed to register process: %v", err)
	}

	processInstance, err := process.CreateProcessInstance(context.Background(), worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("errorBoundaryEvent")
	piAssert.ExecuteJob()

	piAssert.IsWaitingAt("serviceTask")
	piAssert.ExecuteJob()

	piAssert.HasPassed("errorBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()
}
