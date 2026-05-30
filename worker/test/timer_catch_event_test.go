package test

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type timerCatchEvent struct {
}

func (h timerCatchEvent) CreateProcessCmd() (engine.CreateProcessCmd, error) {
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

func (h timerCatchEvent) Handle(mux worker.JobMux) error {
	mux.SetTimer("timerCatchEvent", h.setTimer)
	return nil
}

func (h timerCatchEvent) setTimer(jc worker.JobContext) (engine.Timer, error) {
	return engine.Timer{TimeDuration: engine.ISO8601Duration("PT1H")}, nil
}

func TestTimerCatchEventProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	timerCatchEventProcess, err := w.Register(timerCatchEvent{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := timerCatchEventProcess.CreateProcessInstance(context.Background(), worker.NewProcessVariables())
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.ExecuteJob()

	plusOneHour := time.Now().Add(time.Hour * 1)
	if _, _, err := e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: &plusOneHour,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}
