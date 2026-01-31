package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type errorBoundaryEvent struct {
}

func (h errorBoundaryEvent) CreateProcessCmd() (engine.CreateProcessCmd, error) {
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

func (h errorBoundaryEvent) Handle(mux worker.JobMux) error {
	mux.SetErrorCode("errorBoundaryEvent", h.setErrorCode)
	mux.Execute("serviceTask", h.execute)
	return nil
}

func (h errorBoundaryEvent) setErrorCode(jc worker.JobContext) (string, error) {
	return "TEST_CODE", nil
}

func (h errorBoundaryEvent) execute(jc worker.JobContext) error {
	return worker.NewBpmnError("TEST_CODE")
}

func TestErrorBoundaryEventProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	errorBoundaryEventProcess, err := w.Register(errorBoundaryEvent{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := errorBoundaryEventProcess.CreateProcessInstance(context.Background(), worker.Variables{})
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
