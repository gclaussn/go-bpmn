package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type signalThrowEvent struct {
}

func (h signalThrowEvent) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("event/signal-throw.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "signalThrowTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (h signalThrowEvent) Handle(mux worker.JobMux) error {
	mux.SetSignalName("signalThrowEvent", h.setSignalName)
	return nil
}

func (h signalThrowEvent) setSignalName(jc worker.JobContext) (string, error) {
	return "throw-signal", nil
}

func TestSignalThrowEventProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	signalThrowEventProcess, err := w.Register(signalThrowEvent{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := signalThrowEventProcess.CreateProcessInstance(context.Background(), worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("signalThrowEvent")
	piAssert.ExecuteJob()

	piAssert.IsWaitingAt("signalThrowEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}
