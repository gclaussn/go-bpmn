package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type signalCatchEvent struct {
}

func (h signalCatchEvent) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("event/signal-catch.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "signalCatchTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (h signalCatchEvent) Handle(mux worker.JobMux) error {
	mux.SubscribeSignal("signalCatchEvent", h.subscribeSignal)
	return nil
}

func (h signalCatchEvent) subscribeSignal(jc worker.JobContext) (string, error) {
	return "catch-signal", nil
}

func TestSignalCatchEventProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	signalCatchEventProcess, err := w.Register(signalCatchEvent{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := signalCatchEventProcess.CreateProcessInstance(context.Background(), worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.ExecuteJob()

	if _, err := e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     "catch-signal",
		WorkerId: worker.DefaultWorkerId,
	}); err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}
