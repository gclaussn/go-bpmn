package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSignalEventTest(t *testing.T, e engine.Engine) signalEventTest {
	return signalEventTest{
		e: e,

		boundaryProcess:                mustCreateProcess(t, e, "event/signal-boundary.bpmn", "signalBoundaryTest"),
		boundaryNonInterruptingProcess: mustCreateProcess(t, e, "event/signal-boundary-non-interrupting.bpmn", "signalBoundaryNonInterruptingTest"),
		catchProcess:                   mustCreateProcess(t, e, "event/signal-catch.bpmn", "signalCatchTest"),
		catchDefinitionProcess:         mustCreateProcess(t, e, "event/signal-catch-definition.bpmn", "signalCatchDefinitionTest"),
		startDefinitionProcess:         mustCreateProcess(t, e, "event/signal-start-definition.bpmn", "signalStartDefinitionTest"),
	}
}

type signalEventTest struct {
	e engine.Engine

	boundaryProcess                engine.Process
	boundaryNonInterruptingProcess engine.Process
	catchProcess                   engine.Process
	catchDefinitionProcess         engine.Process
	startDefinitionProcess         engine.Process
}

func (x signalEventTest) boundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: "boundary-signal",
		},
	})

	piAssert.IsWaitingAt("serviceTask")

	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     "boundary-signal",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.ExecuteTask()
	piAssert.HasPassed("signalBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal(engine.InstanceTerminated, elementInstances[2].State) // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // signalBoundaryEvent

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSubscribeSignal, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
}

func (x signalEventTest) boundaryNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryNonInterruptingProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: "boundary-signal",
		},
	})

	piAssert.IsWaitingAt("serviceTask")

	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     "boundary-signal",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.ExecuteTask()

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)  // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // signalBoundaryEvent #1
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State) // signalBoundaryEvent #2

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSubscribeSignal, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
}

func (x signalEventTest) catch(t *testing.T) {
	assert := assert.New(t)

	processInstance, err := x.e.CreateProcessInstance(context.Background(), engine.CreateProcessInstanceCmd{
		BpmnProcessId: x.catchProcess.BpmnProcessId,
		Variables: []engine.ProcessVariable{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: &engine.Data{Encoding: "encoding-b", Value: "value-b"}},
		},
		Version:  x.catchProcess.Version,
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := engine.Assert(t, x.e, processInstance)

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: "catch-signal",
		},
	})

	// when signal sent
	signal, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name: "catch-signal",
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
	assert.Equal("catch-signal", signal.Name)
	assert.Equal(1, signal.SubscriberCount)

	// when signal sent again
	signal, err = x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     "catch-signal",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// then
	assert.NotEmpty(signal.Id)

	assert.NotEmpty(signal.CreatedAt)
	assert.Equal(testWorkerId, signal.CreatedBy)
	assert.Equal("catch-signal", signal.Name)
	assert.Equal(0, signal.SubscriberCount)

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
	piAssert.HasProcessVariable("a")
	piAssert.HasNoProcessVariable("b")
	piAssert.HasNoProcessVariable("c")
}

func (x signalEventTest) catchDefinition(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.catchDefinitionProcess)

	// when signal sent
	signal, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     "testSignalName",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// then
	assert.NotEmpty(signal.Id)

	assert.NotEmpty(signal.CreatedAt)
	assert.Equal(testWorkerId, signal.CreatedBy)
	assert.Equal("testSignalName", signal.Name)
	assert.Equal(2, signal.SubscriberCount) // +1 for signal-start-definition.bpmn

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) end(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-end.bpmn", "signalEndTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("signalEndEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	piAssert.IsWaitingAt("signalEndEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) endDefinition(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-end-definition.bpmn", "signalEndDefinitionTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("signalEndEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) start(t *testing.T) {
	bpmnXml := mustReadBpmnFile(t, "event/signal-start.bpmn")

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "signalStartTest",
		BpmnXml:       bpmnXml,
		Signals: []engine.SignalDefinition{
			{BpmnElementId: "signalStartEvent", SignalName: "start-signal"},
		},
		Version:  "1",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	piAssert1 := engine.AssertSignalStart(t, x.e, process.Id, engine.SendSignalCmd{
		Name: "start-signal",
		Variables: []engine.ProcessVariable{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: &engine.Data{Encoding: "encoding-b", Value: "value-b"}},
			{Name: "c", Data: nil},
		},
		WorkerId: testWorkerId,
	})

	piAssert1.IsCompleted()
	piAssert1.HasProcessVariable("a")
	piAssert1.HasProcessVariable("b")
	piAssert1.HasNoProcessVariable("c")

	piAssert2 := engine.AssertSignalStart(t, x.e, process.Id, engine.SendSignalCmd{
		Name:     "start-signal",
		WorkerId: testWorkerId,
	})
	piAssert2.IsCompleted()

	assert := assert.New(t)
	assert.NotEqual(piAssert1.ProcessInstance().String(), piAssert2.ProcessInstance().String())
}

func (x signalEventTest) startEventDefinition(t *testing.T) {
	piAssert1 := engine.AssertSignalStart(t, x.e, x.startDefinitionProcess.Id, engine.SendSignalCmd{
		Name:     "testSignalName",
		WorkerId: testWorkerId,
	})

	piAssert1.IsCompleted()
}

func (x signalEventTest) throw(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-throw.bpmn", "signalThrowTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("signalThrowEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	piAssert.IsWaitingAt("signalThrowEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) throwDefinition(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-throw-definition.bpmn", "signalThrowDefinitionTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("signalThrowEvent")
	piAssert.ExecuteTask()

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

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name() + "1",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// when signalBoundaryEvent is triggered
	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.ExecuteTask()

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

	query := x.e.CreateQuery()

	pi := piAssert.ProcessInstance()

	// then signal subscription of subProcessSignalCatchEvent is canceled
	signalSubscriptions, err := query.QuerySignalSubscriptions(context.Background(), engine.SignalSubscriptionCriteria{
		Partition:         pi.Partition,
		ProcessInstanceId: pi.Id,
	})
	if err != nil {
		t.Fatalf("failed to query signal subscriptions: %v", err)
	}

	require.Len(signalSubscriptions, 1)
	assert.Equal(elementInstances[3].Id, signalSubscriptions[0].ElementInstanceId)
	assert.Equal("signalCatchEvent", signalSubscriptions[0].BpmnElementId)
	assert.Equal(t.Name()+"2", signalSubscriptions[0].Name)

	subProcessInstances, err := query.QueryProcessInstances(context.Background(), engine.ProcessInstanceCriteria{
		Partition: pi.Partition,
		ParentId:  pi.Id,
	})
	if err != nil {
		t.Fatalf("failed to query sub-process instance: %v", err)
	}

	if len(subProcessInstances) == 0 {
		t.Fatal("no sub-process instance found")
	}

	subPiAssert := engine.Assert(t, x.e, subProcessInstances[0])

	// when sub process instance is terminated
	subTasks := subPiAssert.ExecuteTasks()

	// then
	require.Len(subTasks, 1)
	assert.Equal(engine.TaskTerminateProcessInstance, subTasks[0].Type)

	// then signal subscription of signalCatchEvent is canceled
	signalSubscriptions, err = query.QuerySignalSubscriptions(context.Background(), engine.SignalSubscriptionCriteria{
		Partition:         subProcessInstances[0].Partition,
		ProcessInstanceId: subProcessInstances[0].Id,
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

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob(engine.CompleteJobCmd{
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
	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.ExecuteTask()

	pi := piAssert.ProcessInstance()

	subProcessInstances, err := x.e.CreateQuery().QueryProcessInstances(context.Background(), engine.ProcessInstanceCriteria{
		Partition: pi.Partition,
		ParentId:  pi.Id,
	})
	if err != nil {
		t.Fatalf("failed to query sub-process instance: %v", err)
	}

	if len(subProcessInstances) == 0 {
		t.Fatal("no sub-process instance found")
	}

	subPiAssert := engine.Assert(t, x.e, subProcessInstances[0])

	tasks := subPiAssert.Tasks()
	require.Len(tasks, 2)

	assert.Equal(engine.TaskTriggerEvent, tasks[0].Type)
	assert.Equal(engine.TaskTerminateProcessInstance, tasks[1].Type)

	// when sub-process instance is terminated
	completedTasks, failedTasks, err := x.e.ExecuteTasks(context.Background(), engine.ExecuteTasksCmd{
		Partition: tasks[1].Partition,
		Id:        tasks[1].Id,
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
		Partition: tasks[0].Partition,
		Id:        tasks[0].Id,
	})
	if err != nil {
		t.Fatalf("failed to execute task: %v", err)
	}

	// then
	require.Len(completedTasks, 1)
	require.Len(failedTasks, 0)

	assert.Equal(engine.WorkCanceled, completedTasks[0].State)
}
