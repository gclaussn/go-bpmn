package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type signalCatchEventDelegate struct {
}

func (d signalCatchEventDelegate) CreateProcessCmd() (engine.CreateProcessCmd, error) {
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

func (d signalCatchEventDelegate) Delegate(delegator worker.Delegator) error {
	delegator.SetSignalName("signalCatchEvent", d.setSignalName)
	return nil
}

func (d signalCatchEventDelegate) setSignalName(jc worker.JobContext) (string, error) {
	return "catch-signal", nil
}

func TestSignalCatchEventProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	signalCatchEventProcess, err := w.Register(&signalCatchEventDelegate{})
	if err != nil {
		t.Fatalf("failed to register process: %v", err)
	}

	processInstance, err := signalCatchEventProcess.CreateProcessInstance(worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.ExecuteJob()

	if _, err := e.SendSignal(engine.SendSignalCmd{
		Name:     "catch-signal",
		WorkerId: worker.DefaultEncoding,
	}); err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}
