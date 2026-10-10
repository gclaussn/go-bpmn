package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

type inclusiveGatewayTest struct {
	e engine.Engine
}

func (x inclusiveGatewayTest) gatewayAll(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/inclusive.bpmn", "inclusiveTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	fork.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"endEventA", "endEventB", "endEventC"},
		},
	})

	psAssert.HasPassed("endEventA")
	psAssert.HasPassed("endEventB")
	psAssert.HasPassed("endEventC")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 6)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 6)
}

func (x inclusiveGatewayTest) gatewayOne(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/inclusive.bpmn", "inclusiveTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	fork.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"endEventC"},
		},
	})

	psAssert.HasPassed("endEventC")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 4)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 4)
}

// gatewayDefault tests if the default sequence flow is taken
// when there is no inclusive gateway decision.
func (x inclusiveGatewayTest) gatewayDefault(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/inclusive-default.bpmn", "inclusiveDefaultTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	fork.HasJob(engine.JobEvaluateInclusiveGateway)
	fork.CompleteJob()

	psAssert.HasPassed("endEventA")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 4)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 4)
}

// gatewayDefaultOne tests if the default sequence flow is taken,
// when it is not explicitly taken by the inclusive gateway decision.
func (x inclusiveGatewayTest) gatewayDefaultImplicit(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/inclusive-default.bpmn", "inclusiveDefaultTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	fork.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"endEventC"},
		},
	})

	psAssert.HasPassed("endEventA")
	psAssert.HasPassed("endEventC")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 5)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 5)
}

// gatewayDefaultExplicit tests if the default sequence flow is not additionally taken,
// when it is explicitly taken by the inclusive gateway decision.
func (x inclusiveGatewayTest) gatewayDefaultExplicit(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/inclusive-default.bpmn", "inclusiveDefaultTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	fork.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"endEventA"},
		},
	})

	psAssert.HasPassed("endEventA")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 4)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 4)
}

func (x inclusiveGatewayTest) errorNoBpmnElementId(t *testing.T) {
	process := mustCreateProcess(t, x.e, "gateway/inclusive.bpmn", "inclusiveTest")

	_, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	fork.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{},
		},
	})
}

func (x inclusiveGatewayTest) errorDuplicateBpmnElementId(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/inclusive.bpmn", "inclusiveTest")

	_, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	completedJob := fork.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"endEventA", "endEventA"},
		},
	})

	assert.Contains(completedJob.Error, "duplicate")
	assert.Contains(completedJob.Error, "endEventA")
}

func (x inclusiveGatewayTest) errorSequenceFlowNotExits(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/inclusive.bpmn", "inclusiveTest")

	_, psAssert := mustCreateProcessInstance(t, x.e, process)

	fork := psAssert.IsWaitingAt("fork")
	completedJob := fork.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"startEvent"},
		},
	})

	assert.Contains(completedJob.Error, "no outgoing sequence flow to startEvent")
}
