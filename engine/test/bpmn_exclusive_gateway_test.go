package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

type exclusiveGatewayTest struct {
	e engine.Engine
}

func (x exclusiveGatewayTest) gateway(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/exclusive.bpmn", "exclusiveTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	fork.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ExclusiveGatewayDecision: "join",
		},
	})

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 5)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 5)
}

func (x exclusiveGatewayTest) gatewayDefault(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/exclusive-default.bpmn", "exclusiveDefaultTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	fork.HasJob(engine.JobEvaluateExclusiveGateway)
	fork.CompleteJob()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 5)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 5)
}

func (x exclusiveGatewayTest) errorNoBpmnElementId(t *testing.T) {
	process := mustCreateProcess(t, x.e, "gateway/exclusive.bpmn", "exclusiveTest")

	_, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	fork.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ExclusiveGatewayDecision: "",
		},
	})
}

func (x exclusiveGatewayTest) errorSequenceFlowNotExits(t *testing.T) {
	process := mustCreateProcess(t, x.e, "gateway/exclusive.bpmn", "exclusiveTest")

	_, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	completedJob := fork.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ExclusiveGatewayDecision: "startEvent",
		},
	})

	assert.Contains(t, completedJob.Error, "no outgoing sequence flow to startEvent")
}
