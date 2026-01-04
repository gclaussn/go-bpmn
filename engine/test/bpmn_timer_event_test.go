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

		boundaryProcess:                mustCreateProcess(t, e, "event/timer-boundary.bpmn", "timerBoundaryTest"),
		boundaryNonInterruptingProcess: mustCreateProcess(t, e, "event/timer-boundary-non-interrupting.bpmn", "timerBoundaryNonInterruptingTest"),
		catchProcess:                   mustCreateProcess(t, e, "event/timer-catch.bpmn", "timerCatchTest"),
	}
}

type timerEventTest struct {
	e engine.Engine

	boundaryProcess                engine.Process
	boundaryNonInterruptingProcess engine.Process
	catchProcess                   engine.Process
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

// boundaryWithTimer tests that for a timer boundary event with timer, no SET_TIMER job is created.
func (x timerEventTest) boundaryWithTimer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	bpmnXml := mustReadBpmnFile(t, "event/timer-boundary.bpmn")

	triggerAt := time.Now().Add(time.Hour)

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "timerBoundaryTest",
		BpmnXml:       bpmnXml,
		Timers: []engine.TimerDefinition{
			{BpmnElementId: "timerBoundaryEvent", Timer: &engine.Timer{Time: triggerAt}},
		},
		Version:  t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	piAssert := mustCreateProcessInstance(t, x.e, process)

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
	require.Len(jobs, 1)

	assert.Equal(engine.JobExecute, jobs[0].Type)
}

func (x timerEventTest) boundaryNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryNonInterruptingProcess)

	piAssert.IsWaitingAt("timerBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				TimeDuration: engine.ISO8601Duration("PT1H"),
			},
		},
	})

	// #1
	if err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	piAssert.IsWaitingAt("timerBoundaryEvent")
	piAssert.ExecuteTask()

	// #2
	if err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: time.Now().Add(time.Hour * 2),
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	piAssert.IsWaitingAt("timerBoundaryEvent")
	piAssert.ExecuteTask()

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob()

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

// catchWithTimer tests that for a timer catch event with timer, no SET_TIMER job is created.
func (x timerEventTest) catchWithTimer(t *testing.T) {
	bpmnXml := mustReadBpmnFile(t, "event/timer-catch.bpmn")

	triggerAt := time.Now().Add(time.Hour)

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "timerCatchTest",
		BpmnXml:       bpmnXml,
		Timers: []engine.TimerDefinition{
			{BpmnElementId: "timerCatchEvent", Timer: &engine.Timer{Time: triggerAt}},
		},
		Version:  t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	piAssert := mustCreateProcessInstance(t, x.e, process)

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
		Timers: []engine.TimerDefinition{
			{BpmnElementId: "timerStartEvent", Timer: &engine.Timer{TimeCycle: "0 * * * *"}},
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
