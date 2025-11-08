package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newEscalationEventTest(t *testing.T, e engine.Engine) escalationEventTest {
	return escalationEventTest{
		e: e,

		boundaryProcess:                mustCreateProcess(t, e, "event/escalation-boundary.bpmn", "escalationBoundaryTest"),
		boundaryNonInterruptingProcess: mustCreateProcess(t, e, "event/escalation-boundary-non-interrupting.bpmn", "escalationBoundaryNonInterruptingTest"),
	}
}

type escalationEventTest struct {
	e engine.Engine

	boundaryProcess                engine.Process
	boundaryNonInterruptingProcess engine.Process
}

func (x escalationEventTest) boundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("escalationBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "TEST_CODE",
		},
	})

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "TEST_CODE",
		},
	})

	piAssert.HasPassed("escalationBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal(engine.InstanceTerminated, elementInstances[2].State) // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // esclationBoundaryEvent

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSetEscalationCode, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
}

func (x escalationEventTest) boundaryNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryNonInterruptingProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("escalationBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "TEST_CODE",
		},
	})

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "TEST_CODE",
		},
	})

	piAssert.HasPassed("endEventB")
	piAssert.IsNotCompleted()

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob()

	piAssert.HasPassed("endEventA")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)  // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // esclationBoundaryEvent #1
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State) // esclationBoundaryEvent #2

	jobs := piAssert.Jobs()
	require.Len(jobs, 3)

	assert.Equal(engine.JobSetEscalationCode, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
	assert.Equal(engine.JobExecute, jobs[2].Type)
}
