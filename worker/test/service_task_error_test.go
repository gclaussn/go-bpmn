package test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type serviceTaskErrorDelegate struct {
}

func (d serviceTaskErrorDelegate) CreateProcessCmd() (engine.CreateProcessCmd, error) {
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

func (d serviceTaskErrorDelegate) Delegate(delegator worker.Delegator) error {
	delegator.Execute("serviceTask", d.executeServiceTask)
	return nil
}

func (d serviceTaskErrorDelegate) executeServiceTask(jc worker.JobContext) error {
	if jc.Job.RetryCount < 2 {
		return worker.NewJobErrorWithTimer(
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

	serviceTaskErrorProcess, err := w.Register(&serviceTaskErrorDelegate{})
	if err != nil {
		t.Fatalf("failed to register delegate: %v", err)
	}

	processInstance, err := serviceTaskErrorProcess.CreateProcessInstance(context.Background(), worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("serviceTask")
	piAssert.ExecuteJobWithError()

	now := time.Now().UTC()

	e.SetTime(context.Background(), engine.SetTimeCmd{Time: now.Add(1 * time.Hour).Add(time.Second)})
	piAssert.ExecuteJobWithError()

	e.SetTime(context.Background(), engine.SetTimeCmd{Time: now.Add(2 * time.Hour).Add(time.Second)})
	piAssert.ExecuteJob()

	piAssert.IsCompleted()
}
