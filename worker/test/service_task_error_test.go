package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type serviceTaskError struct {
}

func (h serviceTaskError) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("task/service.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "serviceTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (h serviceTaskError) Handle(mux worker.JobMux) error {
	mux.Execute("serviceTask", h.executeServiceTask)
	return nil
}

func (h serviceTaskError) executeServiceTask(jc worker.JobContext) error {
	if jc.Job.RetryCount < 2 {
		return worker.NewJobErrorWithRetryTimer(
			errors.New("test error"),
			2,
			engine.ISO8601Duration("PT1H"),
		)
	}

	return nil
}

func TestServiceTaskErrorProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	serviceTaskErrorProcess, err := w.Register(serviceTaskError{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := serviceTaskErrorProcess.CreateProcessInstance(context.Background(), worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("serviceTask")
	piAssert.ExecuteJobWithError()

	now := time.Now().UTC()

	e.SetTime(context.Background(), engine.SetTimeCmd{Time: now.Add(1 * time.Hour).Add(time.Minute)})
	piAssert.ExecuteJobWithError()

	e.SetTime(context.Background(), engine.SetTimeCmd{Time: now.Add(2 * time.Hour).Add(time.Minute)})
	piAssert.ExecuteJob()

	piAssert.IsCompleted()
}
