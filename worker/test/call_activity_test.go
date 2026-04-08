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

func (h callActivity) callProcess(jc worker.JobContext) (*engine.CalledProcess, error) {
	return &engine.CalledProcess{
		BpmnProcessId: "startEndTest",
		Version:       "1",
		Variables: []engine.VariableData{
			{Name: "a", Data: &engine.Data{Value: "av", Encoding: "json"}},
		},
	}, nil
}

func (h callActivity) passVariables(jc worker.JobContext, subProcessInstance engine.ProcessInstance) error {
	subVariables, err := jc.ProcessVariablesFrom(subProcessInstance.Partition, subProcessInstance.Id)
	if err != nil {
		return err
	}

	variables := worker.Variables{}
	variables["a"] = subVariables["a"]

	jc.SetProcessVariables(variables)

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

	processInstance, err := callActivityProcess.CreateProcessInstance(context.Background(), worker.Variables{})
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
