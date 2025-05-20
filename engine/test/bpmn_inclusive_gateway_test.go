package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func newInclusiveGatewayTest(t *testing.T, e engine.Engine) inclusiveGatewayTest {
	return inclusiveGatewayTest{
		e: e,

		inclusiveTest: mustCreateProcess(t, e, "gateway/inclusive.bpmn", "inclusiveTest"),
	}
}

type inclusiveGatewayTest struct {
	e engine.Engine

	inclusiveTest engine.Process
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
	piAssert.IsEnded()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 6)

	endedElementInstances := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceEnded}})
	assert.Len(endedElementInstances, 6)
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
	piAssert.IsEnded()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 4)

	endedElementInstances := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceEnded}})
	assert.Len(endedElementInstances, 4)
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
