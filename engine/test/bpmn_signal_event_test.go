package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type signalEventTest struct {
	e engine.Engine
}

func (x signalEventTest) boundary(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "event/signal-boundary.bpmn", "signalBoundaryTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	signalBoundaryEvent := psAssert.IsWaitingAt("signalBoundaryEvent")
	signalBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	serviceTask.HasState(engine.InstanceStarted)
	serviceTask.HasJob(engine.JobExecute)

	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	signalBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	signalBoundaryEvent.ExecuteTask()

	psAssert.HasPassed("signalBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	serviceTask.IsTerminated()
	signalBoundaryEvent.IsCompleted()

	require.Len(piAssert.Jobs(), 2)
}

func (x signalEventTest) boundaryNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/signal-boundary-non-interrupting.bpmn", "signalBoundaryNonInterruptingTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	signalBoundaryEvent := psAssert.IsWaitingAt("signalBoundaryEvent")
	signalBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	serviceTask.HasState(engine.InstanceStarted)
	serviceTask.HasJob(engine.JobExecute)

	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	signalBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	signalBoundaryEvent.ExecuteTask()

	serviceTask.HasJob(engine.JobExecute)
	serviceTask.CompleteJob()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)  // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // signalBoundaryEvent #1
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State) // signalBoundaryEvent #2

	require.Len(piAssert.Jobs(), 2)
}

func (x signalEventTest) catch(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "event/signal-catch.bpmn", "signalCatchTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process, engine.CreateProcessInstanceCmd{
		Variables: []engine.ProcessVariable{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: &engine.Data{Encoding: "encoding-b", Value: "value-b"}},
		},
	})

	signalCatchEvent := psAssert.IsWaitingAt("signalCatchEvent")
	signalCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	// when signal sent
	signal, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name: t.Name(),
		Variables: []engine.ProcessVariable{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: nil},
			{Name: "c", Data: nil},
		},
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// then
	assert.NotEmpty(signal.Id)

	assert.NotEmpty(signal.CreatedAt)
	assert.Equal(testWorkerId, signal.CreatedBy)
	assert.Equal(t.Name(), signal.Name)
	assert.Equal(1, signal.SubscriberCount)

	// when signal sent again
	signal, err = x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// then
	assert.NotEmpty(signal.Id)

	assert.NotEmpty(signal.CreatedAt)
	assert.Equal(testWorkerId, signal.CreatedBy)
	assert.Equal(t.Name(), signal.Name)
	assert.Equal(0, signal.SubscriberCount)

	signalCatchEvent.HasTask(engine.TaskTriggerEvent)
	signalCatchEvent.ExecuteTask()

	piAssert.HasVariable("a")
	piAssert.HasNoVariable("b")
	piAssert.HasNoVariable("c")

	piAssert.IsCompleted()
}

func (x signalEventTest) catchDefinition(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "event/signal-catch-definition.bpmn", "signalCatchDefinitionTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	signalCatchEvent := psAssert.IsWaitingAt("signalCatchEvent")
	signalCatchEvent.HasState(engine.InstanceStarted)

	// when signal sent
	signal, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     "catchSignalName",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// then
	assert.NotEmpty(signal.Id)

	assert.NotEmpty(signal.CreatedAt)
	assert.Equal(testWorkerId, signal.CreatedBy)
	assert.Equal("catchSignalName", signal.Name)
	assert.Equal(1, signal.SubscriberCount)

	signalCatchEvent.HasTask(engine.TaskTriggerEvent)
	signalCatchEvent.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) end(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-end.bpmn", "signalEndTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	signalEndEvent := psAssert.IsWaitingAt("signalEndEvent")
	signalEndEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	signalEndEvent.HasTask(engine.TaskTriggerEvent)
	signalEndEvent.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) endDefinition(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-end-definition.bpmn", "signalEndDefinitionTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	signalEndEvent := psAssert.IsWaitingAt("signalEndEvent")
	signalEndEvent.HasState(engine.InstanceStarted)

	signalEndEvent.HasTask(engine.TaskTriggerEvent)
	signalEndEvent.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) start(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "event/signal-start.bpmn", "signalStartTest", engine.CreateProcessCmd{
		Signals: []engine.SignalDefinition{
			{BpmnElementId: "signalStartEvent", SignalName: t.Name()},
		},
	})

	piAssert1, _ := engine.AssertSignalStart(t, x.e, process, engine.SendSignalCmd{
		Name: t.Name(),
		Variables: []engine.ProcessVariable{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: &engine.Data{Encoding: "encoding-b", Value: "value-b"}},
			{Name: "c", Data: nil},
		},
		WorkerId: testWorkerId,
	})

	piAssert1.HasVariable("a")
	piAssert1.HasVariable("b")
	piAssert1.HasNoVariable("c")

	piAssert1.IsCompleted()

	piAssert2, _ := engine.AssertSignalStart(t, x.e, process, engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})

	piAssert2.IsCompleted()

	assert.NotEqual(piAssert1.ProcessInstance().String(), piAssert2.ProcessInstance().String())
}

func (x signalEventTest) startEventDefinition(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-start-definition.bpmn", "signalStartDefinitionTest")

	piAssert, _ := engine.AssertSignalStart(t, x.e, process, engine.SendSignalCmd{
		Name: "startSignalName",
	})

	piAssert.IsCompleted()
}

func (x signalEventTest) throw(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-throw.bpmn", "signalThrowTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	signalThrowEvent := psAssert.IsWaitingAt("signalThrowEvent")
	signalThrowEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	signalThrowEvent.HasTask(engine.TaskTriggerEvent)
	signalThrowEvent.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) throwDefinition(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-throw-definition.bpmn", "signalThrowDefinitionTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	signalThrowEvent := psAssert.IsWaitingAt("signalThrowEvent")
	signalThrowEvent.HasTask(engine.TaskTriggerEvent)
	signalThrowEvent.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) subscriptionCancelation(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "event/signal-subscription-cancelation.bpmn", "signalSubscriptionCancelationTest", engine.CreateProcessCmd{
			Signals: []engine.SignalDefinition{
				{BpmnElementId: "signalBoundaryEvent", SignalName: t.Name() + "1"},
				{BpmnElementId: "signalCatchEvent", SignalName: t.Name() + "2"},
				{BpmnElementId: "subProcessSignalCatchEvent", SignalName: t.Name() + "3"},
			},
		}),
		mustCreateProcess(t, x.e, "event/signal-catch.bpmn", "signalCatchTest", engine.CreateProcessCmd{
			Signals: []engine.SignalDefinition{
				{BpmnElementId: "signalCatchEvent", SignalName: t.Name() + "4"},
			},
		})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	callActivity := psAssert.IsWaitingAt("subProcess").IsWaitingAt("callActivity")
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	// when signalBoundaryEvent is triggered
	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name() + "1",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	signalBoundaryEvent := psAssert.IsWaitingAt("signalBoundaryEvent")
	signalBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	signalBoundaryEvent.ExecuteTask()

	// then subProcess is terminated
	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 11)
	assert.Equal("subProcess", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State)
	assert.Equal("signalBoundaryEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
	assert.Equal("callActivity", elementInstances[8].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[8].State)
	assert.Equal("subProcessSignalCatchEvent", elementInstances[9].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[9].State)

	// then signal subscription of subProcessSignalCatchEvent is canceled
	processInstance := piAssert.ProcessInstance()

	signalSubscriptions, err := x.e.CreateQuery().QuerySignalSubscriptions(context.Background(), engine.SignalSubscriptionCriteria{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
	})
	if err != nil {
		t.Fatalf("failed to query signal subscriptions: %v", err)
	}

	require.Len(signalSubscriptions, 1)
	assert.Equal(elementInstances[3].Id, signalSubscriptions[0].ElementInstanceId)
	assert.Equal("signalCatchEvent", signalSubscriptions[0].BpmnElementId)
	assert.Equal(t.Name()+"2", signalSubscriptions[0].Name)

	child, _, ok := piAssert.Child()
	require.True(ok)

	// when sub process instance is terminated
	subTasks := child.ExecuteTasks()

	// then
	require.Len(subTasks, 1)
	assert.Equal(engine.TaskTerminateProcessInstance, subTasks[0].Type)

	// then signal subscription of signalCatchEvent is canceled
	subProcessInstance := child.ProcessInstance()

	signalSubscriptions, err = x.e.CreateQuery().QuerySignalSubscriptions(context.Background(), engine.SignalSubscriptionCriteria{
		Partition:         subProcessInstance.Partition,
		ProcessInstanceId: subProcessInstance.Id,
	})
	if err != nil {
		t.Fatalf("failed to query signal subscriptions: %v", err)
	}

	require.Empty(signalSubscriptions)
}

func (x signalEventTest) triggerEventTaskCancelation(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/signal-boundary.bpmn", "callActivitySignalBoundaryTest", engine.CreateProcessCmd{
			Signals: []engine.SignalDefinition{
				{BpmnElementId: "signalBoundaryEvent", SignalName: t.Name()},
			},
		}),
		mustCreateProcess(t, x.e, "event/signal-catch.bpmn", "signalCatchTest", engine.CreateProcessCmd{
			Signals: []engine.SignalDefinition{
				{BpmnElementId: "signalCatchEvent", SignalName: t.Name()},
			},
		})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	callActivity := psAssert.IsWaitingAt("callActivity")
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	signal, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	require.Equal(2, signal.SubscriberCount)

	// when signalBoundaryEvent is triggered
	signalBoundaryEvent := psAssert.IsWaitingAt("signalBoundaryEvent")
	signalBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	signalBoundaryEvent.ExecuteTask()

	child, _, ok := piAssert.Child()
	require.True(ok)

	subTasks := child.Tasks()
	require.Len(subTasks, 2)

	assert.Equal(engine.TaskTriggerEvent, subTasks[0].Type)
	assert.Equal(engine.TaskTerminateProcessInstance, subTasks[1].Type)

	// when sub-process instance is terminated
	completedTasks, failedTasks, err := x.e.ExecuteTasks(context.Background(), engine.ExecuteTasksCmd{
		Partition: subTasks[1].Partition,
		Id:        subTasks[1].Id,
	})
	if err != nil {
		t.Fatalf("failed to execute task: %v", err)
	}

	// then
	require.Len(completedTasks, 1)
	require.Len(failedTasks, 0)

	assert.Equal(engine.WorkDone, completedTasks[0].State)

	// when signalCatchEvent is triggered
	completedTasks, failedTasks, err = x.e.ExecuteTasks(context.Background(), engine.ExecuteTasksCmd{
		Partition: subTasks[0].Partition,
		Id:        subTasks[0].Id,
	})
	if err != nil {
		t.Fatalf("failed to execute task: %v", err)
	}

	// then
	require.Len(completedTasks, 1)
	require.Len(failedTasks, 0)

	assert.Equal(engine.WorkCanceled, completedTasks[0].State)
}
