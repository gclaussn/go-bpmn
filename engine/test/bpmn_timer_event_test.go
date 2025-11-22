package test

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTimerEventTest(t *testing.T, e engine.Engine) timerEventTest {
	return timerEventTest{
		e: e,

		boundaryProcess: mustCreateProcess(t, e, "event/timer-boundary.bpmn", "timerBoundaryTest"),
		catchProcess:    mustCreateProcess(t, e, "event/timer-catch.bpmn", "timerCatchTest"),
	}
}

type timerEventTest struct {
	e engine.Engine

	boundaryProcess engine.Process
	catchProcess    engine.Process
}

func (x timerEventTest) boundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryProcess)

	piAssert.IsWaitingAt("serviceTask")

	triggerAt := time.Now().Add(time.Hour)

	piAssert.IsWaitingAt("timerBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				Time: triggerAt,
			},
		},
	})

	piAssert.IsWaitingAt("serviceTask")

	if err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: triggerAt,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	piAssert.IsWaitingAt("timerBoundaryEvent")
	piAssert.ExecuteTask()
	piAssert.HasPassed("timerBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal(engine.InstanceTerminated, elementInstances[2].State) // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // timerBoundaryEvent

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSetTimer, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
}

func (x timerEventTest) catch(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.catchProcess)

	triggerAt := time.Now().Add(time.Hour)

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				Time: triggerAt,
			},
		},
	})

	if err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: triggerAt,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}

func (x timerEventTest) start(t *testing.T) {
	bpmnXml := mustReadBpmnFile(t, "event/timer-start.bpmn")

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "timerStartTest",
		BpmnXml:       bpmnXml,
		Timers: map[string]*engine.Timer{
			"timerStartEvent": {
				TimeCycle: "0 * * * *",
			},
		},
		Version:  "1",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	piAssert1 := engine.AsserTimerStart(t, x.e, process.Id, "timerStartEvent")
	piAssert1.IsCompleted()

	piAssert2 := engine.AsserTimerStart(t, x.e, process.Id, "timerStartEvent")
	piAssert2.IsCompleted()

	assert := assert.New(t)
	assert.NotEqual(piAssert1.ProcessInstance().String(), piAssert2.ProcessInstance().String())
}
