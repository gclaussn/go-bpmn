package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type callActivityTest struct {
	e engine.Engine
}

func (x callActivityTest) startEnd(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/start-end.bpmn", "callActivityStartEndTest"),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	callActivity := psAssert.IsWaitingAt("callActivity")
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId:  subProcess.BpmnProcessId,
				CorrelationKey: "ck",
				Tags: []engine.Tag{
					{Name: "n1", Value: "v1"},
					{Name: "n2", Value: "v2"},
				},
				Variables: []engine.ProcessVariable{
					{Name: "a", Data: &engine.Data{Value: "av"}},
					{Name: "b", Data: &engine.Data{Value: "bv"}},
				},
				Version: subProcess.Version,
			},
		},
	})

	callActivity.HasJob(engine.JobPassVariables)
	callActivity.CompleteJob()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 4)

	assert.Equal("callActivityStartEndTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("startEvent", elementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[1].State)
	assert.Equal("callActivity", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)
	assert.Equal("endEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)

	children := piAssert.Children()
	require.Len(children, 1)

	subProcessInstance := children[0].ProcessInstance()
	assert.Equal(piAssert.ProcessInstance().Id, subProcessInstance.ParentId)
	assert.Equal(piAssert.ProcessInstance().Id, subProcessInstance.RootId)

	assert.Equal(subProcess.BpmnProcessId, subProcessInstance.BpmnProcessId)
	assert.Equal("ck", subProcessInstance.CorrelationKey)
	assert.Equal(engine.InstanceCompleted, subProcessInstance.State)
	assert.Equal(subProcess.Version, subProcessInstance.Version)

	require.Len(subProcessInstance.Tags, 2)
	assert.Equal("n1", subProcessInstance.Tags[0].Name)
	assert.Equal("v1", subProcessInstance.Tags[0].Value)
	assert.Equal("n2", subProcessInstance.Tags[1].Name)
	assert.Equal("v2", subProcessInstance.Tags[1].Value)

	variables, err := x.e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
		Partition:         subProcessInstance.Partition,
		ProcessInstanceId: subProcessInstance.Id,
	})
	if err != nil {
		t.Fatalf("failed to get child process variables: %v", err)
	}

	require.Len(variables, 2)
	assert.Equal("a", variables[0].Name)
	assert.Equal("av", variables[0].Data.Value)
	assert.Equal("b", variables[1].Name)
	assert.Equal("bv", variables[1].Data.Value)

	subElementInstances := children[0].ElementInstances()

	require.Len(subElementInstances, 3)
	assert.Equal("startEndTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("endEvent", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[2].State)
}

func (x callActivityTest) suspensionAndQueueing(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/start-end.bpmn", "callActivityStartEndTest"),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest", engine.CreateProcessCmd{
			Parallelism: 1, // only one instance at a time
		})

	// given
	piAssert1, psAssert1 := mustCreateProcessInstance(t, x.e, process)
	piAssert2, psAssert2 := mustCreateProcessInstance(t, x.e, process)

	processInstance1 := piAssert1.ProcessInstance()

	// when process instance #1 is suspended and process is called
	if err := x.e.SuspendProcessInstance(context.Background(), engine.SuspendProcessInstanceCmd{
		Partition: processInstance1.Partition,
		Id:        processInstance1.Id,
		WorkerId:  testWorkerId,
	}); err != nil {
		t.Fatalf("failed to suspend process instance: %v", err)
	}

	callActivity1 := psAssert1.IsWaitingAt("callActivity")
	callActivity1.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	// then sub process instance #1 is suspended
	child1, _, ok := piAssert1.Child()
	require.True(ok)

	child1.HasState(engine.InstanceSuspended)

	subElementInstances1 := child1.ElementInstances()
	assert.Equal("startEndTest", subElementInstances1[0].BpmnElementId)
	assert.Equal(engine.InstanceSuspended, subElementInstances1[0].State)
	assert.Equal("startEvent", subElementInstances1[1].BpmnElementId)
	assert.Equal(engine.InstanceSuspended, subElementInstances1[1].State)

	// when another process is called
	callActivity2 := psAssert2.IsWaitingAt("callActivity")
	callActivity2.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	// then sub process instance #2 is queued
	child2, _, ok := piAssert2.Child()
	require.True(ok)

	child2.HasState(engine.InstanceQueued)

	subElementInstances2 := child2.ElementInstances()
	assert.Equal(engine.InstanceQueued, subElementInstances2[0].State)
	assert.Equal(engine.InstanceQueued, subElementInstances2[1].State)

	// when sub process instance #1 is resumed and call activity is completed
	subProcessInstance1 := child1.ProcessInstance()
	if err := x.e.ResumeProcessInstance(context.Background(), engine.ResumeProcessInstanceCmd{
		Partition: subProcessInstance1.Partition,
		Id:        subProcessInstance1.Id,
		WorkerId:  testWorkerId,
	}); err != nil {
		t.Fatalf("failed to resume process instance: %v", err)
	}

	callActivity1.HasJob(engine.JobPassVariables)
	callActivity1.CompleteJob()

	// then process instance #1 is still suspended, but sub process instance #1 is completed
	piAssert1.HasState(engine.InstanceSuspended)

	child1.IsCompleted()

	subElementInstances1 = child1.ElementInstances()
	assert.Equal(engine.InstanceCompleted, subElementInstances1[0].State)
	assert.Equal(engine.InstanceCompleted, subElementInstances1[1].State)
	assert.Equal(engine.InstanceCompleted, subElementInstances1[2].State)

	// when sub process instance #2 is started and call activity is completed
	child2.ExecuteTasks()

	callActivity2.HasJob(engine.JobPassVariables)
	callActivity2.CompleteJob()

	// then process instance #2 and sub process instance #2 are completed
	piAssert2.IsCompleted()

	child2.IsCompleted()

	subElementInstances2 = child2.ElementInstances()
	assert.Equal(engine.InstanceCompleted, subElementInstances2[0].State)
	assert.Equal(engine.InstanceCompleted, subElementInstances2[1].State)
	assert.Equal(engine.InstanceCompleted, subElementInstances2[2].State)
}

func (x callActivityTest) calledElement(t *testing.T) {
	process, _ :=
		mustCreateProcess(t, x.e, "call-activity/called-element.bpmn", "callActivityCalledElementTest"),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest", engine.CreateProcessCmd{
			Version: "called-element-test", // version specified in the calledElement attribute
		})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	callActivity := psAssert.IsWaitingAt("callActivity")

	callActivity.HasJob(engine.JobCallProcess)
	callActivity.CompleteJob()

	callActivity.HasJob(engine.JobPassVariables)
	callActivity.CompleteJob()

	piAssert.IsCompleted()
}

func (x callActivityTest) calledElementLatest(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, _, subProcessV2 :=
		mustCreateProcess(t, x.e, "call-activity/called-element-latest.bpmn", "callActivityCalledElementLatestTest"),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest", engine.CreateProcessCmd{
			Version: t.Name() + "v1",
		}),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest", engine.CreateProcessCmd{
			Version: t.Name() + "v2",
		})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	callActivity := psAssert.IsWaitingAt("callActivity")

	callActivity.HasJob(engine.JobCallProcess)
	callActivity.CompleteJob()

	callActivity.HasJob(engine.JobPassVariables)
	callActivity.CompleteJob()

	piAssert.IsCompleted()

	children := piAssert.Children()
	require.Len(children, 1)

	subProcessInstance := children[0].ProcessInstance()
	assert.Equal(subProcessV2.Version, subProcessInstance.Version)
}

func (x callActivityTest) boundaryEvent(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/signal-boundary.bpmn", "callActivitySignalBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	signalBoundaryEvent := psAssert.IsWaitingAt("signalBoundaryEvent")
	signalBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	callActivity := psAssert.IsWaitingAt("callActivity")
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	if _, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	}); err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	child, _, ok := piAssert.Child()
	require.True(ok)

	signalBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	signalBoundaryEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)
	assert.Equal("callActivitySignalBoundaryTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("startEvent", elementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[1].State)
	assert.Equal("callActivity", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("signalBoundaryEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("signalEnd", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)

	subTasks := child.ExecuteTasks()
	require.Len(subTasks, 1)
	assert.Equal(engine.TaskTerminateProcessInstance, subTasks[0].Type)

	subProcessInstance := child.ProcessInstance()
	assert.NotNil(subProcessInstance.EndedAt)
	assert.Equal(engine.InstanceTerminated, subProcessInstance.State)

	subElementInstances := child.ElementInstances()
	require.Len(subElementInstances, 3)
	assert.Equal("serviceTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[0].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("serviceTask", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[2].State)
}

// boundaryEventWithRecursiveTermination tests that the execution of a [TerminateProcessInstanceTask] instance terminates active process instances recursively,
// by the creation of additional [TerminateProcessInstanceTask] instances for each active call activity.
func (x callActivityTest) boundaryEventWithRecursiveTermination(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// given
	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/signal-boundary.bpmn", "callActivitySignalBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	signalBoundaryEvent1 := psAssert.IsWaitingAt("signalBoundaryEvent")
	signalBoundaryEvent1.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	// when callActivityBoundaryTest process is called
	callActivity1 := psAssert.IsWaitingAt("callActivity")
	callActivity1.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: process.BpmnProcessId,
				Version:       process.Version,
			},
		},
	})

	// given
	child1, childScope1, ok := piAssert.Child()
	require.True(ok)

	signalBoundaryEvent2 := childScope1.IsWaitingAt("signalBoundaryEvent")
	signalBoundaryEvent2.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name() + "sub",
		},
	})

	// when serviceTest process is called
	callActivity2 := childScope1.IsWaitingAt("callActivity")
	callActivity2.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	// given
	child2, _, ok := child1.Child()
	require.True(ok)

	// when signalBoundarEvent of root process instance is triggered
	if _, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	}); err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	signalBoundaryEvent1.HasTask(engine.TaskTriggerEvent)
	signalBoundaryEvent1.ExecuteTask()

	piAssert.IsCompleted()

	// when
	subTasks1 := child1.ExecuteTasks()

	// then
	require.Len(subTasks1, 1)
	assert.Equal(engine.TaskTerminateProcessInstance, subTasks1[0].Type)

	// when
	subTasks2 := child2.ExecuteTasks()

	// then
	require.Len(subTasks2, 1)
	assert.Equal(engine.TaskTerminateProcessInstance, subTasks2[0].Type)
}

func (x callActivityTest) executeWithErrorCode(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/error-boundary.bpmn", "callActivityErrorBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

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

	child, childScope, ok := piAssert.Child()
	require.True(ok)

	serviceTask := childScope.IsWaitingAt("serviceTask")
	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "testErrorCode",
		},
	})

	subElementInstances := child.ElementInstances()
	require.Len(subElementInstances, 3)
	assert.Equal("serviceTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[0].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("serviceTask", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[2].State)

	errorBoundaryEvent := psAssert.IsWaitingAt("errorBoundaryEvent")
	errorBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	errorBoundaryEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)
	assert.Equal("callActivityErrorBoundaryTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("startEvent", elementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[1].State)
	assert.Equal("callActivity", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("errorBoundaryEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("errorEnd", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
}

func (x callActivityTest) errorEnd(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/error-boundary.bpmn", "callActivityErrorBoundaryTest"),
		mustCreateProcess(t, x.e, "event/error-end.bpmn", "errorEndTest", engine.CreateProcessCmd{
			Errors: []engine.ErrorDefinition{
				{BpmnElementId: "errorBoundaryEvent", ErrorCode: "ignored"},
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

	child, childScope, ok := piAssert.Child()
	require.True(ok)

	errorEndEvent := childScope.IsWaitingAt("subProcess").IsWaitingAt("errorEndEvent")
	errorEndEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "testErrorCode",
		},
	})

	errorEndEvent.HasTask(engine.TaskTriggerEvent)
	errorEndEvent.ExecuteTask()

	subElementInstances := child.ElementInstances()
	require.Len(subElementInstances, 6)
	assert.Equal("errorEndTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[0].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("subProcess", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[2].State)
	assert.Equal("errorBoundaryEvent", subElementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[3].State)
	assert.Equal("subProcessStartEvent", subElementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[4].State)
	assert.Equal("errorEndEvent", subElementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[5].State)

	errorBoundaryEvent := psAssert.IsWaitingAt("errorBoundaryEvent")
	errorBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	errorBoundaryEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)
	assert.Equal("callActivityErrorBoundaryTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("startEvent", elementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[1].State)
	assert.Equal("callActivity", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("errorBoundaryEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("errorEnd", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
}

func (x callActivityTest) executeWithEscalationCode(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/escalation-boundary.bpmn", "callActivityEscalationBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

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

	child, childScope, ok := piAssert.Child()
	require.True(ok)

	serviceTask := childScope.IsWaitingAt("serviceTask")
	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "testEscalationCode",
		},
	})

	subElementInstances := child.ElementInstances()
	require.Len(subElementInstances, 3)
	assert.Equal("serviceTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[0].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("serviceTask", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[2].State)

	escalationBoundaryEvent := psAssert.IsWaitingAt("escalationBoundaryEvent")
	escalationBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	escalationBoundaryEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 6)
	assert.Equal("callActivityEscalationBoundaryTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("startEvent", elementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[1].State)
	assert.Equal("callActivity", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[3].State)
	assert.Equal("escalationBoundaryEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
	assert.Equal("escalationEnd", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
}

func (x callActivityTest) executeWithNonInterruptingEscalationCode(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/escalation-boundary.bpmn", "callActivityEscalationBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

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

	child, childScope, ok := piAssert.Child()
	require.True(ok)

	serviceTask := childScope.IsWaitingAt("serviceTask")
	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "testEscalationNonInterruptingCode",
		},
	})

	child.HasState(engine.InstanceStarted)

	subElementInstances := child.ElementInstances()
	require.Len(subElementInstances, 3)
	assert.Equal("serviceTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceStarted, subElementInstances[0].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("serviceTask", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceStarted, subElementInstances[2].State)

	escalationBoundaryEventNonInterrupting := psAssert.IsWaitingAt("escalationBoundaryEventNonInterrupting")
	escalationBoundaryEventNonInterrupting.HasTask(engine.TaskTriggerEvent)
	escalationBoundaryEventNonInterrupting.ExecuteTask()

	piAssert.HasState(engine.InstanceStarted)

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)
	assert.Equal("callActivityEscalationBoundaryTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceStarted, elementInstances[0].State)
	assert.Equal("startEvent", elementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[1].State)
	assert.Equal("callActivity", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceStarted, elementInstances[2].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("escalationBoundaryEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[4].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[5].State)
	assert.Equal("nonInterruptingEscalationEnd", elementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[6].State)
}

func (x callActivityTest) escalationEnd(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/escalation-boundary.bpmn", "callActivityEscalationBoundaryTest"),
		mustCreateProcess(t, x.e, "event/escalation-throw-end.bpmn", "escalationThrowEndTest", engine.CreateProcessCmd{
			Escalations: []engine.EscalationDefinition{
				{BpmnElementId: "escalationBoundaryEvent", EscalationCode: "ignored"},
				{BpmnElementId: "escalationBoundaryEventNonInterrupting", EscalationCode: "ignored"},
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

	child, childScope, ok := piAssert.Child()
	require.True(ok)

	escalationThrowEvent := childScope.IsWaitingAt("subProcess").IsWaitingAt("escalationThrowEvent")
	escalationThrowEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "not-existing",
		},
	})

	escalationThrowEvent.HasTask(engine.TaskTriggerEvent)
	escalationThrowEvent.ExecuteTask()

	escalationEndEvent := childScope.IsWaitingAt("subProcess").IsWaitingAt("escalationEndEvent")
	escalationEndEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "testEscalationCode",
		},
	})

	escalationEndEvent.HasTask(engine.TaskTriggerEvent)
	escalationEndEvent.ExecuteTask()

	subElementInstances := child.ElementInstances()
	require.Len(subElementInstances, 8)
	assert.Equal("escalationThrowEndTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[0].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("subProcess", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[2].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", subElementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[3].State)
	assert.Equal("escalationBoundaryEvent", subElementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[4].State)
	assert.Equal("subProcessStartEvent", subElementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[5].State)
	assert.Equal("escalationThrowEvent", subElementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[6].State)
	assert.Equal("escalationEndEvent", subElementInstances[7].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[7].State)

	escalationBoundaryEvent := psAssert.IsWaitingAt("escalationBoundaryEvent")
	escalationBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	escalationBoundaryEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 6)
	assert.Equal("callActivityEscalationBoundaryTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[0].State)
	assert.Equal("startEvent", elementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[1].State)
	assert.Equal("callActivity", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[3].State)
	assert.Equal("escalationBoundaryEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
	assert.Equal("escalationEnd", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
}

func (x callActivityTest) escalationThrow(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/escalation-boundary.bpmn", "callActivityEscalationBoundaryTest"),
		mustCreateProcess(t, x.e, "event/escalation-throw-end.bpmn", "escalationThrowEndTest", engine.CreateProcessCmd{
			Escalations: []engine.EscalationDefinition{
				{BpmnElementId: "escalationBoundaryEvent", EscalationCode: "ignored"},
				{BpmnElementId: "escalationBoundaryEventNonInterrupting", EscalationCode: "ignored"},
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

	child, childScope, ok := piAssert.Child()
	require.True(ok)

	escalationThrowEvent := childScope.IsWaitingAt("subProcess").IsWaitingAt("escalationThrowEvent")
	escalationThrowEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "testEscalationNonInterruptingCode",
		},
	})

	escalationThrowEvent.HasTask(engine.TaskTriggerEvent)
	escalationThrowEvent.ExecuteTask()

	subElementInstances := child.ElementInstances()
	require.Len(subElementInstances, 8)
	assert.Equal("escalationThrowEndTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceStarted, subElementInstances[0].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("subProcess", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceStarted, subElementInstances[2].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", subElementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCreated, subElementInstances[3].State)
	assert.Equal("escalationBoundaryEvent", subElementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCreated, subElementInstances[4].State)
	assert.Equal("subProcessStartEvent", subElementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[5].State)
	assert.Equal("escalationThrowEvent", subElementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[6].State)
	assert.Equal("escalationEndEvent", subElementInstances[7].BpmnElementId)
	assert.Equal(engine.InstanceCreated, subElementInstances[7].State)

	escalationBoundaryEventNonInterrupting := psAssert.IsWaitingAt("escalationBoundaryEventNonInterrupting")
	escalationBoundaryEventNonInterrupting.HasTask(engine.TaskTriggerEvent)
	escalationBoundaryEventNonInterrupting.ExecuteTask()

	piAssert.HasState(engine.InstanceStarted)

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)
	assert.Equal("callActivityEscalationBoundaryTest", elementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceStarted, elementInstances[0].State)
	assert.Equal("startEvent", elementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[1].State)
	assert.Equal("callActivity", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceStarted, elementInstances[2].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("escalationBoundaryEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[4].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[5].State)
	assert.Equal("nonInterruptingEscalationEnd", elementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[6].State)
}

func (x callActivityTest) errorProcessNotFound(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "call-activity/start-end.bpmn", "callActivityStartEndTest")

	_, psAssert := mustCreateProcessInstance(t, x.e, process)

	callActivity := psAssert.IsWaitingAt("callActivity")
	completedJob := callActivity.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: "not-existing",
				Version:       "1",
			},
		},
	})
	assert.Contains(completedJob.Error, "process not-existing:1 could not be found")
}

func (x callActivityTest) errorProcessHasNoNoneStart(t *testing.T) {
	assert := assert.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/start-end.bpmn", "callActivityStartEndTest"),
		mustCreateProcess(t, x.e, "event/signal-start.bpmn", "signalStartTest", engine.CreateProcessCmd{
			Signals: []engine.SignalDefinition{
				{BpmnElementId: "signalStartEvent", SignalName: t.Name()},
			},
			Version: t.Name(),
		})

	_, psAssert := mustCreateProcessInstance(t, x.e, process)

	callActivity := psAssert.IsWaitingAt("callActivity")
	completedJob := callActivity.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})
	assert.Contains(completedJob.Error, "has no none start event")
}

func (x callActivityTest) errorCalledProcessVersionNotFound(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "call-activity/called-process-version-not-found.bpmn", "callActivityCalledProcessVersionNotFoundTest")

	_, psAssert := mustCreateProcessInstance(t, x.e, process)

	callActivity := psAssert.IsWaitingAt("callActivity")
	completedJob := callActivity.CompleteJobWithError()
	assert.Contains(completedJob.Error, "process not-existing:1 could not be found")
}

func (x callActivityTest) errorCalledProcessNotFound(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "call-activity/called-process-not-found.bpmn", "callActivityCalledProcessNotFoundTest")

	_, psAssert := mustCreateProcessInstance(t, x.e, process)

	callActivity := psAssert.IsWaitingAt("callActivity")
	completedJob := callActivity.CompleteJobWithError()
	assert.Contains(completedJob.Error, "process not-existing could not be found")
}
