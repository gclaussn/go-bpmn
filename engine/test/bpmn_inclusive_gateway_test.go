package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func newInclusiveGatewayTest(t *testing.T, e engine.Engine) inclusiveGatewayTest {
	return inclusiveGatewayTest{
		e: e,

		inclusiveTest:        mustCreateProcess(t, e, "gateway/inclusive.bpmn", "inclusiveTest"),
		inclusiveDefaultTest: mustCreateProcess(t, e, "gateway/inclusive-default.bpmn", "inclusiveDefaultTest"),
	}
}

type inclusiveGatewayTest struct {
	e engine.Engine

	inclusiveTest        engine.Process
	inclusiveDefaultTest engine.Process
}

func (x inclusiveGatewayTest) gatewayAll(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.inclusiveTest)

	piAssert.IsWaitingAt("fork")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"endEventA", "endEventB", "endEventC"},
		},
	})

	piAssert.HasPassed("endEventA")
	piAssert.HasPassed("endEventB")
	piAssert.HasPassed("endEventC")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 6)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 6)
}

func (x inclusiveGatewayTest) gatewayOne(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.inclusiveTest)

	piAssert.IsWaitingAt("fork")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"endEventC"},
		},
	})

	piAssert.HasPassed("endEventC")
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

	piAssert := mustCreateProcessInstance(t, x.e, x.inclusiveDefaultTest)

	piAssert.IsWaitingAt("fork")
	piAssert.CompleteJob()

	piAssert.HasPassed("endEventA")
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

	piAssert := mustCreateProcessInstance(t, x.e, x.inclusiveDefaultTest)

	piAssert.IsWaitingAt("fork")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"endEventC"},
		},
	})

	piAssert.HasPassed("endEventA")
	piAssert.HasPassed("endEventC")
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

	piAssert := mustCreateProcessInstance(t, x.e, x.inclusiveDefaultTest)

	piAssert.IsWaitingAt("fork")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"endEventA"},
		},
	})

	piAssert.HasPassed("endEventA")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 4)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 4)
}

func (x inclusiveGatewayTest) errorNoBpmnElementId(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.inclusiveTest)

	piAssert.IsWaitingAt("fork")
	piAssert.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{},
		},
	})
}

func (x inclusiveGatewayTest) errorDuplicateBpmnElementId(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.inclusiveTest)

	piAssert.IsWaitingAt("fork")

	completedJob := piAssert.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"endEventA", "endEventA"},
		},
	})

	assert.Contains(completedJob.Error, "duplicate")
	assert.Contains(completedJob.Error, "endEventA")
}

func (x inclusiveGatewayTest) errorSequenceFlowNotExits(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.inclusiveTest)

	piAssert.IsWaitingAt("fork")

	completedJob := piAssert.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			InclusiveGatewayDecision: []string{"startEvent"},
		},
	})

	assert.Contains(completedJob.Error, "no outgoing sequence flow to startEvent")
}
