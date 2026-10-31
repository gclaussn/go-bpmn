package test

import (
	"context"
	"strconv"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type multiInstanceServiceTask struct {
	t *testing.T

	serviceTaskCounter int
}

func (h *multiInstanceServiceTask) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("multi-instance/parallel-service-task.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "parallelServiceTaskTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (h *multiInstanceServiceTask) Handle(mux worker.JobMux) error {
	mux.StartMultiInstance("serviceTask", h.startServiceTaskMultiInstance)
	mux.Execute("serviceTask", h.executeServiceTask)
	mux.EndMultiInstance("serviceTask", h.endServiceTaskMultiInstance)
	return nil
}

func (h *multiInstanceServiceTask) startServiceTaskMultiInstance(jc worker.JobContext) (engine.MultiInstance, error) {
	return engine.MultiInstance{
		LoopCardinality: 3,
	}, nil
}

func (h *multiInstanceServiceTask) executeServiceTask(jc worker.JobContext) error {
	assert, require := assert.New(h.t), require.New(h.t)

	oldElementVariables, err := jc.ElementVariables("loopCounter")
	if err != nil {
		return err
	}

	loopCounter, ok := oldElementVariables.Get("loopCounter")
	require.True(ok)

	assert.Equal("json", loopCounter.Encoding)
	assert.False(loopCounter.IsEncrypted)
	assert.True(loopCounter.IsEncoded)
	assert.Equal("loopCounter", loopCounter.Name)
	assert.Equal(strconv.Itoa(h.serviceTaskCounter), loopCounter.Value)

	h.serviceTaskCounter++

	newElementVariables := jc.NewElementVariables()
	newElementVariables.Put("serviceTaskCounter", h.serviceTaskCounter)
	newElementVariables.PutAt("parallelServiceTaskTest", "serviceTaskCounter", h.serviceTaskCounter)

	return nil
}

func (h *multiInstanceServiceTask) endServiceTaskMultiInstance(jc worker.JobContext) error {
	assert, require := assert.New(h.t), require.New(h.t)

	elementVariables, err := jc.ElementVariables("loopCounter", "serviceTaskCounter")
	if err != nil {
		return err
	}

	loopCounters := elementVariables.GetAll("loopCounter")
	require.Len(loopCounters, 3)

	assert.Equal("0", loopCounters[0].Value)
	assert.Equal("1", loopCounters[1].Value)
	assert.Equal("2", loopCounters[2].Value)

	serviceTaskCounters := elementVariables.GetAll("serviceTaskCounter")
	require.Len(serviceTaskCounters, 3)

	assert.Equal("1", serviceTaskCounters[0].Value)
	assert.Equal("2", serviceTaskCounters[1].Value)
	assert.Equal("3", serviceTaskCounters[2].Value)

	serviceTaskCounter, ok := elementVariables.GetAt("parallelServiceTaskTest", "serviceTaskCounter")
	require.True(ok)

	assert.Equal("3", serviceTaskCounter.Value)

	return nil
}

func TestMultiInstanceServiceTaskProcess(t *testing.T) {
	require := require.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	serviceTaskProcess, err := w.Register(&multiInstanceServiceTask{t: t})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := serviceTaskProcess.CreateProcessInstance(context.Background(), worker.NewProcessVariables())
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert, psAssert := worker.Assert(t, w, processInstance)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.HasJob(engine.JobStartMultiInstance)
	serviceTask.ExecuteJob()

	multiInstanceActivities := serviceTask.Children()
	require.Len(multiInstanceActivities, 3)

	for _, multiInstanceActivity := range multiInstanceActivities {
		multiInstanceActivity.HasJob(engine.JobExecute)
		multiInstanceActivity.ExecuteJob()
	}

	serviceTask.HasJob(engine.JobEndMultiInstance)
	serviceTask.ExecuteJob()

	piAssert.IsCompleted()
}
