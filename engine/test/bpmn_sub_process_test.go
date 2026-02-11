package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type subProcessTest struct {
	e engine.Engine
}

func (x subProcessTest) startEnd(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	startEndProcess := mustCreateProcess(t, x.e, "sub-process/start-end.bpmn", "startEndTest")

	piAssert := mustCreateProcessInstance(t, x.e, startEndProcess)
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 6)

	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("startEndTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[1].State)
	assert.Equal("startEvent", elementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)
	assert.Equal("subProcess", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("subProcessStartEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
	assert.Equal("subProcessEndEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
	assert.Equal("endEvent", elementInstances[5].BpmnElementId)
}
