package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type linkEventTest struct {
	e engine.Engine
}

func (x linkEventTest) linkA(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/link.bpmn", "linkTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("fork")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ExclusiveGatewayDecision: "linkThrowEventA",
		},
	})

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal("linkThrowEventA", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("linkCatchEventA", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
}

func (x linkEventTest) linkB(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/link.bpmn", "linkTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("fork")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ExclusiveGatewayDecision: "linkThrowEventB",
		},
	})

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal("linkThrowEventB", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("linkCatchEventB", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
}

func (x linkEventTest) linkScope(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/link-scope.bpmn", "linkScopeTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("fork")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ExclusiveGatewayDecision: "subProcess",
		},
	})

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 10)

	assert.Equal("linkThrowEventB", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
	assert.Equal("linkCatchEventB", elementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[6].State)
}
