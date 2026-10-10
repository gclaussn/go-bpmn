package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type escalationBoundaryEvent struct {
}

func (h escalationBoundaryEvent) CreateProcessCmd() (engine.CreateProcessCmd, error) {
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

func (h escalationBoundaryEvent) Handle(mux worker.JobMux) error {
	mux.SetEscalationCode("escalationBoundaryEvent", h.setEscalationCode)
	mux.Execute("serviceTask", h.execute)
	return nil
}

func (h escalationBoundaryEvent) setEscalationCode(jc worker.JobContext) (string, error) {
	return "TEST_CODE", nil
}

func (h escalationBoundaryEvent) execute(jc worker.JobContext) error {
	return worker.NewBpmnEscalation("TEST_CODE")
}

func TestEscalationBoundaryEventProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	escalationBounaryEventProcess, err := w.Register(escalationBoundaryEvent{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := escalationBounaryEventProcess.CreateProcessInstance(context.Background(), worker.NewProcessVariables())
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert, psAssert := worker.Assert(t, w, processInstance)

	psAssert.IsWaitingAt("serviceTask")
	psAssert.IsWaitingAt("escalationBoundaryEvent").ExecuteJob()
	psAssert.IsWaitingAt("serviceTask").ExecuteJob()

	psAssert.HasPassed("escalationBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()
}
