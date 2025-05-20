package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func newExclusiveGatewayTest(t *testing.T, e engine.Engine) exclusiveGatewayTest {
	return exclusiveGatewayTest{
		e: e,

		exclusiveTest: mustCreateProcess(t, e, "gateway/exclusive.bpmn", "exclusiveTest"),
	}
}

type exclusiveGatewayTest struct {
	e engine.Engine

	exclusiveTest engine.Process
}

func (x exclusiveGatewayTest) gateway(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.exclusiveTest)

	piAssert.IsWaitingAt("fork")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ExclusiveGatewayDecision: "join",
		},
	})

	piAssert.IsEnded()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 5)

	endedElementInstances := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceEnded}})
	assert.Len(endedElementInstances, 5)
}

func (x exclusiveGatewayTest) errorNoBpmnElementId(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.exclusiveTest)

	piAssert.IsWaitingAt("fork")
	job := piAssert.Job()

	lockedJobs, err := x.e.LockJobs(engine.LockJobsCmd{
		Id:        job.Id,
		Partition: job.Partition,
		WorkerId:  testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to lock job: %v", err)
	}

	if len(lockedJobs) == 0 {
		t.Fatalf("no job locked")
	}

	completedJob, err := x.e.CompleteJob(engine.CompleteJobCmd{
		Id:        job.Id,
		Partition: job.Partition,
		Completion: &engine.JobCompletion{
			ExclusiveGatewayDecision: "",
		},
		WorkerId: testWorkerId,
	})

	assert.True(completedJob.HasError())
	assert.True(completedJob.IsCompleted())
	assert.Nil(err)
}

func (x exclusiveGatewayTest) errorSequenceFlowNotExits(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.exclusiveTest)

	piAssert.IsWaitingAt("fork")
	job := piAssert.Job()

	lockedJobs, err := x.e.LockJobs(engine.LockJobsCmd{
		Id:        job.Id,
		Partition: job.Partition,
		WorkerId:  testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to lock job: %v", err)
	}

	if len(lockedJobs) == 0 {
		t.Fatalf("no job locked")
	}

	completedJob, err := x.e.CompleteJob(engine.CompleteJobCmd{
		Id:        job.Id,
		Partition: job.Partition,
		Completion: &engine.JobCompletion{
			ExclusiveGatewayDecision: "startEvent",
		},
		WorkerId: testWorkerId,
	})

	assert.True(completedJob.HasError())
	assert.True(completedJob.IsCompleted())
	assert.Nil(err)

	assert.Contains(completedJob.Error, "no outgoing sequence flow to startEvent")
}
