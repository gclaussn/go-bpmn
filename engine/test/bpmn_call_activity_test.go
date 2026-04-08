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

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId:  subProcess.BpmnProcessId,
				CorrelationKey: "ck",
				Tags: []engine.Tag{
					{Name: "n1", Value: "v1"},
					{Name: "n2", Value: "v2"},
				},
				Variables: []engine.VariableData{
					{Name: "a", Data: &engine.Data{Value: "av"}},
					{Name: "b", Data: &engine.Data{Value: "bv"}},
				},
				Version: subProcess.Version,
			},
		},
	})

	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob()

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

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobCallProcess, jobs[0].Type)
	assert.Equal(engine.JobPassVariables, jobs[1].Type)

	subProcessInstance, subElementInstances := x.querySubProcessInstance(t, piAssert)

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

	require.Len(subElementInstances, 3)
	assert.Equal("startEndTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("endEvent", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[2].State)
}

func (x callActivityTest) suspensionAndQueueing(t *testing.T) {
	assert := assert.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/start-end.bpmn", "callActivityStartEndTest"),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest", engine.CreateProcessCmd{
			Parallelism: 1, // only one instance at a time
		})

	// given
	piAssert1 := mustCreateProcessInstance(t, x.e, process)
	piAssert2 := mustCreateProcessInstance(t, x.e, process)

	pi1 := piAssert1.ProcessInstance()

	// when pi1 is suspended and process is called
	if err := x.e.SuspendProcessInstance(context.Background(), engine.SuspendProcessInstanceCmd{
		Partition: pi1.Partition,
		Id:        pi1.Id,
		WorkerId:  testWorkerId,
	}); err != nil {
		t.Fatalf("failed to suspend process instance: %v", err)
	}

	piAssert1.IsWaitingAt("callActivity")
	piAssert1.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	// then subPi1 is suspended
	subProcessInstance1, subElementInstances1 := x.querySubProcessInstance(t, piAssert1)

	assert.Equal(engine.InstanceSuspended, subProcessInstance1.State)

	assert.Equal(engine.InstanceSuspended, subElementInstances1[0].State)
	assert.Equal(engine.InstanceSuspended, subElementInstances1[1].State)

	// when another process is called
	piAssert2.IsWaitingAt("callActivity")
	piAssert2.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	// then subPi2 is queued
	subProcessInstance2, subElementInstances2 := x.querySubProcessInstance(t, piAssert2)

	assert.Equal(engine.InstanceQueued, subProcessInstance2.State)

	assert.Equal(engine.InstanceQueued, subElementInstances2[0].State)
	assert.Equal(engine.InstanceQueued, subElementInstances2[1].State)

	// when subPi1 is resumed and call activity is completed
	if err := x.e.ResumeProcessInstance(context.Background(), engine.ResumeProcessInstanceCmd{
		Partition: subProcessInstance1.Partition,
		Id:        subProcessInstance1.Id,
		WorkerId:  testWorkerId,
	}); err != nil {
		t.Fatalf("failed to resume process instance: %v", err)
	}

	piAssert1.IsWaitingAt("callActivity")
	piAssert1.CompleteJob()

	// then pi1 is still suspended, but subPi1 is completed
	piAssert1.IsNotCompleted()

	subProcessInstance1a, subElementInstances1a := x.querySubProcessInstance(t, piAssert1)

	assert.Equal(engine.InstanceCompleted, subProcessInstance1a.State)

	assert.Equal(engine.InstanceCompleted, subElementInstances1a[0].State)
	assert.Equal(engine.InstanceCompleted, subElementInstances1a[1].State)

	// when pi2 is started and call activity is completed
	piAssert4 := engine.Assert(t, x.e, subProcessInstance2)
	piAssert4.ExecuteTasks()

	piAssert2.IsWaitingAt("callActivity")
	piAssert2.CompleteJob()

	// then pi2 and subPi2 are completed
	piAssert2.IsCompleted()

	subProcessInstance2a, subElementInstances2a := x.querySubProcessInstance(t, piAssert1)

	assert.Equal(engine.InstanceCompleted, subProcessInstance2a.State)

	assert.Equal(engine.InstanceCompleted, subElementInstances2a[0].State)
	assert.Equal(engine.InstanceCompleted, subElementInstances2a[1].State)
}

func (x callActivityTest) calledElement(t *testing.T) {
	process, _ :=
		mustCreateProcess(t, x.e, "call-activity/called-element.bpmn", "callActivityCalledElementTest"),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest", engine.CreateProcessCmd{
			Version: "called-element-test", // version specified in the calledElement attribute
		})

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob()

	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob()

	piAssert.IsCompleted()
}

func (x callActivityTest) calledElementLatest(t *testing.T) {
	assert := assert.New(t)

	process, _, subProcessV2 :=
		mustCreateProcess(t, x.e, "call-activity/called-element-latest.bpmn", "callActivityCalledElementLatestTest"),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest", engine.CreateProcessCmd{
			Version: t.Name() + "v1",
		}),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest", engine.CreateProcessCmd{
			Version: t.Name() + "v2",
		})

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob()

	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob()

	piAssert.IsCompleted()

	subProcessInstance, _ := x.querySubProcessInstance(t, piAssert)
	assert.Equal(subProcessV2.Version, subProcessInstance.Version)
}

func (x callActivityTest) boundaryEvent(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/signal-boundary.bpmn", "callActivitySignalBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob(engine.CompleteJobCmd{
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

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.ExecuteTask()

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

	subProcessInstance, _ := x.querySubProcessInstance(t, piAssert)
	subPiAssert := engine.Assert(t, x.e, subProcessInstance)

	subTasks := subPiAssert.ExecuteTasks()
	require.Len(subTasks, 1)
	assert.Equal(engine.TaskTerminateProcessInstance, subTasks[0].Type)

	terminatedSubProcessInstance, subElementInstances := x.querySubProcessInstance(t, piAssert)
	assert.NotNil(terminatedSubProcessInstance.EndedAt)
	assert.Equal(engine.InstanceTerminated, terminatedSubProcessInstance.State)

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

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	// when callActivityBoundaryTest process is called
	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: process.BpmnProcessId,
				Version:       process.Version,
			},
		},
	})

	// given
	subProcessInstance, _ := x.querySubProcessInstance(t, piAssert)
	subPiAssert := engine.Assert(t, x.e, subProcessInstance)

	subPiAssert.IsWaitingAt("signalBoundaryEvent")
	subPiAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name() + "sub",
		},
	})

	// when serviceTest process is called
	subPiAssert.IsWaitingAt("callActivity")
	subPiAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	// when signalBoundarEvent of root process instance is triggered
	if _, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	}); err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()

	// when
	subTasks := subPiAssert.ExecuteTasks()

	// then
	require.Len(subTasks, 1)
	assert.Equal(engine.TaskTerminateProcessInstance, subTasks[0].Type)

	// given
	subSubProcessInstance, _ := x.querySubProcessInstance(t, subPiAssert)
	subSubPiAssert := engine.Assert(t, x.e, subSubProcessInstance)

	// when
	subSubTasks := subSubPiAssert.ExecuteTasks()

	// then
	require.Len(subSubTasks, 1)
	assert.Equal(engine.TaskTerminateProcessInstance, subSubTasks[0].Type)
}

func (x callActivityTest) executeWithErrorCode(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/error-boundary.bpmn", "callActivityErrorBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

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

	subProcessInstance, _ := x.querySubProcessInstance(t, piAssert)
	subPiAssert := engine.Assert(t, x.e, subProcessInstance)

	subPiAssert.IsWaitingAt("serviceTask")
	subPiAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "testErrorCode",
		},
	})

	subElementInstances := subPiAssert.ElementInstances()
	require.Len(subElementInstances, 3)
	assert.Equal("serviceTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[0].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("serviceTask", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[2].State)

	piAssert.IsWaitingAt("errorBoundaryEvent")
	piAssert.ExecuteTask()

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

	piAssert.IsCompleted()
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

	subProcessInstance, _ := x.querySubProcessInstance(t, piAssert)
	subPiAssert := engine.Assert(t, x.e, subProcessInstance)

	subPiAssert.IsWaitingAt("errorEndEvent")
	subPiAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "testErrorCode",
		},
	})

	subPiAssert.IsWaitingAt("errorEndEvent")
	subPiAssert.ExecuteTask()

	subElementInstances := subPiAssert.ElementInstances()
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

	piAssert.IsWaitingAt("errorBoundaryEvent")
	piAssert.ExecuteTask()

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

	piAssert.IsCompleted()
}

func (x callActivityTest) executeWithEscalationCode(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/escalation-boundary.bpmn", "callActivityEscalationBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

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

	subProcessInstance, _ := x.querySubProcessInstance(t, piAssert)
	subPiAssert := engine.Assert(t, x.e, subProcessInstance)

	subPiAssert.IsWaitingAt("serviceTask")
	subPiAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "testEscalationCode",
		},
	})

	subElementInstances := subPiAssert.ElementInstances()
	require.Len(subElementInstances, 3)
	assert.Equal("serviceTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[0].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("serviceTask", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, subElementInstances[2].State)

	piAssert.IsWaitingAt("escalationBoundaryEvent")
	piAssert.ExecuteTask()

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

	piAssert.IsCompleted()
}

func (x callActivityTest) executeWithNonInterruptingEscalationCode(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/escalation-boundary.bpmn", "callActivityEscalationBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

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

	subProcessInstance, _ := x.querySubProcessInstance(t, piAssert)
	subPiAssert := engine.Assert(t, x.e, subProcessInstance)

	subPiAssert.IsWaitingAt("serviceTask")
	subPiAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "testEscalationNonInterruptingCode",
		},
	})

	subPiAssert.IsNotCompleted()

	subElementInstances := subPiAssert.ElementInstances()
	require.Len(subElementInstances, 3)
	assert.Equal("serviceTest", subElementInstances[0].BpmnElementId)
	assert.Equal(engine.InstanceStarted, subElementInstances[0].State)
	assert.Equal("startEvent", subElementInstances[1].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, subElementInstances[1].State)
	assert.Equal("serviceTask", subElementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceStarted, subElementInstances[2].State)

	piAssert.IsWaitingAt("escalationBoundaryEventNonInterrupting")
	piAssert.ExecuteTask()

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

	piAssert.IsNotCompleted()
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

	subProcessInstance, _ := x.querySubProcessInstance(t, piAssert)
	subPiAssert := engine.Assert(t, x.e, subProcessInstance)

	subPiAssert.IsWaitingAt("escalationThrowEvent")
	subPiAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "not-existing",
		},
	})

	subPiAssert.IsWaitingAt("escalationThrowEvent")
	subPiAssert.ExecuteTask()

	subPiAssert.IsWaitingAt("escalationEndEvent")
	subPiAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "testEscalationCode",
		},
	})

	subPiAssert.IsWaitingAt("escalationEndEvent")
	subPiAssert.ExecuteTask()

	subElementInstances := subPiAssert.ElementInstances()
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

	piAssert.IsWaitingAt("escalationBoundaryEvent")
	piAssert.ExecuteTask()

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

	piAssert.IsCompleted()
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

	subProcessInstance, _ := x.querySubProcessInstance(t, piAssert)
	subPiAssert := engine.Assert(t, x.e, subProcessInstance)

	subPiAssert.IsWaitingAt("escalationThrowEvent")
	subPiAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "testEscalationNonInterruptingCode",
		},
	})

	subPiAssert.IsWaitingAt("escalationThrowEvent")
	subPiAssert.ExecuteTask()

	subElementInstances := subPiAssert.ElementInstances()
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

	piAssert.IsWaitingAt("escalationBoundaryEventNonInterrupting")
	piAssert.ExecuteTask()

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

	piAssert.IsNotCompleted()
}

func (x callActivityTest) errorProcessNotFound(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "call-activity/start-end.bpmn", "callActivityStartEndTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("callActivity")
	completedJob := piAssert.CompleteJobWithError(engine.CompleteJobCmd{
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

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("callActivity")
	completedJob := piAssert.CompleteJobWithError(engine.CompleteJobCmd{
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

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("callActivity")
	completedJob := piAssert.CompleteJobWithError()
	assert.Contains(completedJob.Error, "process not-existing:1 could not be found")
}

func (x callActivityTest) errorCalledProcessNotFound(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "call-activity/called-process-not-found.bpmn", "callActivityCalledProcessNotFoundTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("callActivity")
	completedJob := piAssert.CompleteJobWithError()
	assert.Contains(completedJob.Error, "process not-existing could not be found")
}

func (x callActivityTest) querySubProcessInstance(t *testing.T, piAssert *engine.ProcessInstanceAssert) (engine.ProcessInstance, []engine.ElementInstance) {
	pi := piAssert.ProcessInstance()

	query := x.e.CreateQuery()

	processInstances, err := query.QueryProcessInstances(context.Background(), engine.ProcessInstanceCriteria{
		Partition: pi.Partition,
		ParentId:  pi.Id,
	})
	if err != nil {
		t.Fatalf("failed to query sub process instance: %v", err)
	}

	if len(processInstances) == 0 {
		t.Fatal("no sub process instance found")
	}

	subElementInstances, err := query.QueryElementInstances(context.Background(), engine.ElementInstanceCriteria{
		Partition:         processInstances[0].Partition,
		ProcessInstanceId: processInstances[0].Id,
	})
	if err != nil {
		t.Fatalf("failed to query sub element instances: %v", err)
	}

	return processInstances[0], subElementInstances
}
