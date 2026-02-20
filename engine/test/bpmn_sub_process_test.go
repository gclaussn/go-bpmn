package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type subProcessTest struct {
	e engine.Engine
}

func (x subProcessTest) startEnd(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "sub-process/start-end.bpmn", "startEndTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 6)

	assert.Equal("startEndTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("startEvent", elementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[1].State)
	assert.Equal("subProcess", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)
	assert.Equal("subProcessStartEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("subProcessEndEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
	assert.Equal("endEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
}

func (x subProcessTest) boundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "sub-process/boundary.bpmn", "boundaryTest", engine.CreateProcessCmd{
		Signals: []engine.SignalDefinition{
			{BpmnElementId: "signalBoundaryEvent", SignalName: t.Name()},
		},
	})

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageName:           t.Name(),
			MessageCorrelationKey: t.Name(),
		},
	})

	piAssert.IsWaitingAt("timerBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				TimeDuration: "PT1H",
			},
		},
	})

	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.ExecuteTask()
	piAssert.HasPassed("signalEnd")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 11)

	assert.Equal("boundaryTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("subProcess", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("messageBoundaryEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[3].State)
	assert.Equal("signalBoundaryEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
	assert.Equal("timerBoundaryEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[5].State)
	assert.Equal("serviceTaskA", elementInstances[8].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[8].State)
	assert.Equal("serviceTaskB", elementInstances[9].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[9].State)
}

func (x subProcessTest) boundaryTerminated(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "sub-process/boundary.bpmn", "boundaryTest", engine.CreateProcessCmd{
		Signals: []engine.SignalDefinition{
			{BpmnElementId: "signalBoundaryEvent", SignalName: t.Name()},
		},
	})

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageName:           t.Name(),
			MessageCorrelationKey: t.Name(),
		},
	})

	piAssert.IsWaitingAt("timerBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				TimeDuration: "PT1H",
			},
		},
	})

	piAssert.IsWaitingAt("serviceTaskA")
	piAssert.CompleteJob()

	piAssert.IsWaitingAt("serviceTaskB")
	piAssert.CompleteJob()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 15)

	assert.Equal("boundaryTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("subProcess", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)
	assert.Equal("messageBoundaryEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[3].State)
	assert.Equal("signalBoundaryEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State)
	assert.Equal("timerBoundaryEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[5].State)
}

// nested tests that a nested sub-process is completed.
func (x subProcessTest) nested(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "sub-process/nested.bpmn", "nestedTest", engine.CreateProcessCmd{
		Signals: []engine.SignalDefinition{
			{BpmnElementId: "signalBoundaryEvent", SignalName: t.Name()},
		},
	})

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 11)

	assert.Equal("nestedTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("subProcess", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)
	assert.Equal("signalBoundaryEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[3].State)
	assert.Equal("nestedSubProcess", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
}

// nestedTerminated tests that an active nested sub-process is terminated when a parent sub-process is terminated.
func (x subProcessTest) nestedTerminated(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "sub-process/nested.bpmn", "nestedTest", engine.CreateProcessCmd{
		Signals: []engine.SignalDefinition{
			{BpmnElementId: "signalBoundaryEvent", SignalName: t.Name()},
		},
	})

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("serviceTask")

	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 9)

	assert.Equal("nestedTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("subProcess", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("signalBoundaryEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("subProcessStartEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
	assert.Equal("nestedSubProcess", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[5].State)
	assert.Equal("nestedSubProcessStartEvent", elementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[6].State)
	assert.Equal("serviceTask", elementInstances[7].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[7].State)
}
