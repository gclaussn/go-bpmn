package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
	"github.com/stretchr/testify/assert"
)

type serviceTask struct {
	t *testing.T
}

func (h serviceTask) CreateProcessCmd() (engine.CreateProcessCmd, error) {
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

func (h serviceTask) Handle(mux worker.JobMux) error {
	mux.Execute("serviceTask", h.executeServiceTask)
	return nil
}

func (h serviceTask) executeServiceTask(jc worker.JobContext) error {
	assert := assert.New(h.t)

	assert.NotNil(jc.Engine)
	assert.Equal(int32(1), jc.Job.Id)
	assert.Equal(int32(1), jc.Process.Id)
	assert.Equal("serviceTask", jc.Element.BpmnElementId)

	oldProcessVariables, err := jc.ProcessVariables()
	if err != nil {
		return err
	}

	variableA := oldProcessVariables["a"]
	assert.Equal("json", variableA.Encoding)
	assert.False(variableA.IsEncrypted)
	assert.Equal("a", variableA.Name)
	assert.Equal("\"string\"", variableA.Value)

	var a string
	if err := variableA.Decode(jc, &a); err != nil {
		return err
	}

	assert.Equal("string", a)

	newProcessVariables := worker.Variables{}
	newProcessVariables.Put("a", "string*")
	newProcessVariables.Delete("e")

	jc.SetProcessVariables(newProcessVariables)

	newElementVariables := worker.Variables{}
	newElementVariables.Put("a", "string")

	jc.SetElementVariables(newElementVariables)

	return nil
}

func TestServiceTaskProcess(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	serviceTaskProcess, err := w.Register(serviceTask{t})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	process := serviceTaskProcess.Process
	assert.Equal(int32(1), process.Id)
	assert.Equal("serviceTest", process.BpmnProcessId)
	assert.Equal("1", process.Version)

	createProcessInstanceCmd := serviceTaskProcess.CreateProcessInstanceCmd()
	assert.Equal("serviceTest", createProcessInstanceCmd.BpmnProcessId)
	assert.Equal("1", createProcessInstanceCmd.Version)
	assert.Equal(worker.DefaultWorkerId, createProcessInstanceCmd.WorkerId)

	variables := worker.Variables{}
	variables.Put("a", "string")
	variables.Put("b", 1)
	variables.Put("c", true)
	variables.Put("d", 0.1)
	variables.Put("e", engine.Data{Value: "value"}) // example for a complex variable

	processInstance, err := serviceTaskProcess.CreateProcessInstance(context.Background(), variables)
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	assert.Equal(int32(1), processInstance.Id)

	piAssert := worker.Assert(t, w, processInstance)

	var variableA string
	piAssert.GetProcessVariable("a", &variableA)
	assert.Equal("string", variableA)
	var variableB int
	piAssert.GetProcessVariable("b", &variableB)
	assert.Equal(1, variableB)
	var variableC bool
	piAssert.GetProcessVariable("c", &variableC)
	assert.Equal(true, variableC)
	var variableD float64
	piAssert.GetProcessVariable("d", &variableD)
	assert.Equal(0.1, variableD)
	var variableE engine.Data
	piAssert.GetProcessVariable("e", &variableE)
	assert.Equal("", variableE.Encoding)
	assert.False(variableE.IsEncrypted)
	assert.Equal("value", variableE.Value)

	piAssert.IsWaitingAt("serviceTask")
	piAssert.ExecuteJob()

	piAssert.GetProcessVariable("a", &variableA)
	assert.Equal("string*", variableA)

	piAssert.HasNoProcessVariable("e")

	var elementVariableA string
	piAssert.GetElementVariable("serviceTask", "a", &elementVariableA)
	assert.Equal("string", elementVariableA)

	piAssert.IsCompleted()
}
