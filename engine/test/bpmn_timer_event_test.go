package test

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type timerEventTest struct {
	e engine.Engine
}

func (x timerEventTest) boundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/timer-boundary.bpmn", "timerBoundaryTest")

	piAssert, psAssert := mustCreateProcessInstance2(t, x.e, process)

	psAssert.IsWaitingAt("serviceTask")

	triggerAt := time.Now().Add(time.Hour)

	timerBoundaryEvent := psAssert.IsWaitingAt("timerBoundaryEvent")

	timerBoundaryEvent.HasJob(engine.JobSetTimer)
	timerBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				Time: triggerAt,
			},
		},
	})

	psAssert.IsWaitingAt("serviceTask")

	if _, _, err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: triggerAt,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	timerBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	timerBoundaryEvent.ExecuteTask()

	psAssert.HasPassed("timerBoundaryEvent")
	psAssert.HasPassed("endEventB")

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

// boundaryWithTimer tests that for a timer boundary event with timer, no SET_TIMER job is created.
func (x timerEventTest) boundaryWithTimer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	triggerAt := time.Now().Add(time.Hour)

	process := mustCreateProcess(t, x.e, "event/timer-boundary.bpmn", "timerBoundaryTest", engine.CreateProcessCmd{
		Timers: []engine.TimerDefinition{
			{BpmnElementId: "timerBoundaryEvent", Timer: &engine.Timer{Time: triggerAt}},
		},
	})

	piAssert, psAssert := mustCreateProcessInstance2(t, x.e, process)

	psAssert.IsWaitingAt("serviceTask")

	if _, _, err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: triggerAt,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	timerBoundaryEvent := psAssert.IsWaitingAt("timerBoundaryEvent")
	timerBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	timerBoundaryEvent.ExecuteTask()

	psAssert.HasPassed("timerBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal(engine.InstanceTerminated, elementInstances[2].State) // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // timerBoundaryEvent

	jobs := piAssert.Jobs()
	require.Len(jobs, 1)

	assert.Equal(engine.JobExecute, jobs[0].Type)
}

func (x timerEventTest) boundaryNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/timer-boundary-non-interrupting.bpmn", "timerBoundaryNonInterruptingTest")

	piAssert, psAssert := mustCreateProcessInstance2(t, x.e, process)

	timerBoundaryEvent1 := psAssert.IsWaitingAt("timerBoundaryEvent")

	timerBoundaryEvent1.HasJob(engine.JobSetTimer)
	timerBoundaryEvent1.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				TimeDuration: engine.ISO8601Duration("PT1H"),
			},
		},
	})

	// #1
	plusOneHour := time.Now().Add(time.Hour)
	if _, _, err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: plusOneHour,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	timerBoundaryEvent1.HasTask(engine.TaskTriggerEvent)
	timerBoundaryEvent1.ExecuteTask()

	// #2
	plusTwoHour := time.Now().Add(time.Hour * 2)
	if _, _, err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: plusTwoHour,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	timerBoundaryEvent2 := psAssert.IsWaitingAt("timerBoundaryEvent")

	timerBoundaryEvent2.HasTask(engine.TaskTriggerEvent)
	timerBoundaryEvent2.ExecuteTask()

	psAssert.IsWaitingAt("serviceTask").CompleteJob()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 9)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State) // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State) // timerBoundaryEvent #1
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State) // timerBoundaryEvent #2
	assert.Equal("endEventB", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[6].State) // timerBoundaryEvent #3
	assert.Equal("endEventB", elementInstances[7].BpmnElementId)
	assert.Equal("endEventA", elementInstances[8].BpmnElementId)

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSetTimer, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
}

func (x timerEventTest) catch(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/timer-catch.bpmn", "timerCatchTest")

	piAssert, psAssert := mustCreateProcessInstance2(t, x.e, process)

	triggerAt := time.Now().Add(time.Hour)

	timerCatchEvent := psAssert.IsWaitingAt("timerCatchEvent")

	timerCatchEvent.HasJob(engine.JobSetTimer)
	timerCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				Time: triggerAt,
			},
		},
	})

	if _, _, err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: triggerAt,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	timerCatchEvent.HasTask(engine.TaskTriggerEvent)
	timerCatchEvent.ExecuteTask()

	piAssert.IsCompleted()
}

// catchWithTimer tests that for a timer catch event with timer, no SET_TIMER job is created.
func (x timerEventTest) catchWithTimer(t *testing.T) {
	triggerAt := time.Now().Add(time.Hour)

	process := mustCreateProcess(t, x.e, "event/timer-catch.bpmn", "timerCatchTest", engine.CreateProcessCmd{
		Timers: []engine.TimerDefinition{
			{BpmnElementId: "timerCatchEvent", Timer: &engine.Timer{Time: triggerAt}},
		},
	})

	piAssert, psAssert := mustCreateProcessInstance2(t, x.e, process)

	if _, _, err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: triggerAt,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	psAssert.IsWaitingAt("timerCatchEvent").ExecuteTask()

	piAssert.IsCompleted()
}

func (x timerEventTest) start(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/timer-start.bpmn", "timerStartTest", engine.CreateProcessCmd{
		Timers: []engine.TimerDefinition{
			{BpmnElementId: "timerStartEvent", Timer: &engine.Timer{TimeCycle: "0 * * * *"}},
		},
	})

	piAssert1, _ := engine.AsserTimerStart2(t, x.e, process, "timerStartEvent")
	piAssert1.IsCompleted()

	piAssert2, _ := engine.AsserTimerStart2(t, x.e, process, "timerStartEvent")
	piAssert2.IsCompleted()

	assert.NotEqual(t, piAssert1.ProcessInstance().String(), piAssert2.ProcessInstance().String())
}
