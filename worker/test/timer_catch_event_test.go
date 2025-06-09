package test

import (
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type timerCatchEventDelegate struct {
}

func (d timerCatchEventDelegate) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("event/timer-catch.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "timerCatchTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (d timerCatchEventDelegate) Delegate(delegator worker.Delegator) error {
	delegator.SetTimer("timerCatchEvent", d.setTimer)
	return nil
}

func (d timerCatchEventDelegate) setTimer(jc worker.JobContext) (engine.Timer, error) {
	return engine.Timer{TimeDuration: engine.ISO8601Duration("PT1H")}, nil
}

func TestTimerCatchEventProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	timerCatchEventProcess, err := w.Register(&timerCatchEventDelegate{})
	if err != nil {
		t.Fatalf("failed to register process: %v", err)
	}

	processInstance, err := timerCatchEventProcess.CreateProcessInstance(worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.ExecuteJob()

	if err := e.SetTime(engine.SetTimeCmd{
		Time: time.Now().Add(time.Hour * 1),
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsEnded()
}
