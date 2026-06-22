package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
	"github.com/stretchr/testify/assert"
)

type callActivity struct {
}

func (h callActivity) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("call-activity/start-end.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "callActivityStartEndTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (h callActivity) Handle(mux worker.JobMux) error {
	mux.CallProcess("callActivity", h.callProcess)
	mux.PassVariables("callActivity", h.passVariables)
	return nil
}

func (h callActivity) callProcess(jc worker.JobContext) (worker.CalledProcess, error) {
	variables := worker.NewProcessVariables()
	variables.Set(worker.ProcessVariable{
		Encoding: "json",
		Name:     "a",
		Value:    "av",
	})

	return worker.CalledProcess{
		BpmnProcessId: "startEndTest",
		Version:       "1",
		Variables:     variables,
	}, nil
}

func (h callActivity) passVariables(jc worker.JobContext, subProcessInstance engine.ProcessInstance) error {
	subVariables, err := jc.SubProcessVariables(subProcessInstance)
	if err != nil {
		return err
	}

	variables := jc.NewProcessVariables()

	a, _ := subVariables.Get("a")
	variables.Set(a)

	return nil
}

func TestCallActivityProcess(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	// create sub process
	bpmnXml, err := readBpmnFile("start-end.bpmn")
	if err != nil {
		t.Fatalf("failed to read BPMN file: %v", err)
	}

	e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "startEndTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	})

	callActivityProcess, err := w.Register(callActivity{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := callActivityProcess.CreateProcessInstance(context.Background(), worker.NewProcessVariables())
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("callActivity")
	piAssert.ExecuteJob()

	piAssert.IsWaitingAt("callActivity")
	piAssert.ExecuteJob()

	piAssert.IsCompleted()

	var variableA string
	piAssert.GetProcessVariable("a", &variableA)
	assert.Equal("av", variableA)
}
