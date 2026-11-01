package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type multiInstanceTest struct {
	e engine.Engine
}

func (x multiInstanceTest) parallelCallActivity(t *testing.T) {
	require := require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "multi-instance/parallel-call-activity.bpmn", "parallelCallActivityTest"),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 3)

	callActivity := psAssert.IsWaitingAt("callActivity")

	callActivity.HasState(engine.InstanceCreated)
	callActivity.HasJob(engine.JobStartMultiInstance)
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	callActivity.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 6)

	children := callActivity.Children()
	require.Len(children, 3)

	for _, child := range children {
		child.HasState(engine.InstanceStarted)
	}

	for _, child := range children {
		child.HasJob(engine.JobCallProcess)
		child.CompleteJob(engine.CompleteJobCmd{
			Completion: &engine.JobCompletion{
				CalledProcess: &engine.CalledProcess{
					BpmnProcessId: subProcess.BpmnProcessId,
					Version:       subProcess.Version,
				},
			},
		})
	}

	for _, child := range children {
		child.HasState(engine.InstanceStarted)
		child.HasJob(engine.JobPassVariables)
		child.CompleteJob()
	}

	callActivity.HasState(engine.InstanceStarted)
	callActivity.HasJob(engine.JobEndMultiInstance)
	callActivity.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.Children(), 3)
	require.Len(piAssert.ElementInstances(), 7)
	require.Len(piAssert.Jobs(), 8)
}

func (x multiInstanceTest) parallelCallActivityErrorBoundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "multi-instance/parallel-call-activity-boundary.bpmn", "parallelCallActivityBoundaryTest", engine.CreateProcessCmd{
			Errors: []engine.ErrorDefinition{
				{BpmnElementId: "errorBoundaryEvent", ErrorCode: t.Name()},
			},
			Escalations: []engine.EscalationDefinition{
				{BpmnElementId: "escalationBoundaryEvent", EscalationCode: t.Name()},
			},
		}),
		mustCreateProcess(t, x.e, "user-task/start-end.bpmn", "userTaskStartEndTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 5)

	callActivity := psAssert.IsWaitingAt("callActivity")

	callActivity.HasState(engine.InstanceCreated)
	callActivity.HasJob(engine.JobStartMultiInstance)
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	callActivity.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 8)

	multiInstanceActivities := callActivity.Children()
	require.Len(multiInstanceActivities, 3)

	for _, multiInstanceActivity := range multiInstanceActivities {
		multiInstanceActivity.HasState(engine.InstanceStarted)
	}

	for _, multiInstanceActivity := range multiInstanceActivities {
		multiInstanceActivity.HasJob(engine.JobCallProcess)
		multiInstanceActivity.CompleteJob(engine.CompleteJobCmd{
			Completion: &engine.JobCompletion{
				CalledProcess: &engine.CalledProcess{
					BpmnProcessId: subProcess.BpmnProcessId,
					Version:       subProcess.Version,
				},
			},
		})
	}

	children := piAssert.Children()
	require.Len(piAssert.Children(), 3)

	// when user task #1 is completed
	_, childScope, ok := piAssert.Child()
	require.True(ok)

	childScope.IsWaitingAt("userTask").CompleteUserTask()

	multiInstanceActivities[0].HasJob(engine.JobPassVariables)
	multiInstanceActivities[0].CompleteJob()

	// when user task #2 throws a BPMN error
	_, childScope, ok = piAssert.Child()
	require.True(ok)

	userTask := childScope.IsWaitingAt("userTask").UserTask()

	updatedUserTask, err := x.e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{
		Partition: userTask.Partition,
		Id:        userTask.Id,

		Revision: userTask.Revision,

		ErrorCode: t.Name(),
		WorkerId:  testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to update user task: %v", err)
	}

	// then
	assert.Equal(engine.UserTaskTerminated, updatedUserTask.State)

	children[0].IsCompleted()
	children[1].IsTerminated()
	children[2].HasState(engine.InstanceStarted)

	// when error boundary event is triggered
	errorBoundaryEvent := psAssert.IsWaitingAt("errorBoundaryEvent")
	errorBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	errorBoundaryEvent.ExecuteTask()

	// then
	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 9)
	require.Len(piAssert.Jobs(), 5)
	require.Len(piAssert.Tasks(), 1)

	// when terminated process instance task is executed
	children[2].ExecuteTasks()
	children[2].IsTerminated()
}

func (x multiInstanceTest) parallelCallActivityEscalationBoundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "multi-instance/parallel-call-activity-boundary.bpmn", "parallelCallActivityBoundaryTest", engine.CreateProcessCmd{
			Errors: []engine.ErrorDefinition{
				{BpmnElementId: "errorBoundaryEvent", ErrorCode: t.Name()},
			},
			Escalations: []engine.EscalationDefinition{
				{BpmnElementId: "escalationBoundaryEvent", EscalationCode: t.Name()},
			},
		}),
		mustCreateProcess(t, x.e, "user-task/start-end.bpmn", "userTaskStartEndTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 5)

	callActivity := psAssert.IsWaitingAt("callActivity")

	callActivity.HasState(engine.InstanceCreated)
	callActivity.HasJob(engine.JobStartMultiInstance)
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	callActivity.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 8)

	multiInstanceActivities := callActivity.Children()
	require.Len(multiInstanceActivities, 3)

	for _, multiInstanceActivity := range multiInstanceActivities {
		multiInstanceActivity.HasState(engine.InstanceStarted)
	}

	for _, multiInstanceActivity := range multiInstanceActivities {
		multiInstanceActivity.HasJob(engine.JobCallProcess)
		multiInstanceActivity.CompleteJob(engine.CompleteJobCmd{
			Completion: &engine.JobCompletion{
				CalledProcess: &engine.CalledProcess{
					BpmnProcessId: subProcess.BpmnProcessId,
					Version:       subProcess.Version,
				},
			},
		})
	}

	children := piAssert.Children()
	require.Len(piAssert.Children(), 3)

	// when user task #1 is completed
	_, childScope, ok := piAssert.Child()
	require.True(ok)

	childScope.IsWaitingAt("userTask").CompleteUserTask()

	multiInstanceActivities[0].HasJob(engine.JobPassVariables)
	multiInstanceActivities[0].CompleteJob()

	// when user task #2 is escalated
	_, childScope, ok = piAssert.Child()
	require.True(ok)

	userTask := childScope.IsWaitingAt("userTask").UserTask()

	updatedUserTask, err := x.e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{
		Partition: userTask.Partition,
		Id:        userTask.Id,

		Revision: userTask.Revision,

		EscalationCode: t.Name(),
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to update user task: %v", err)
	}

	// then
	assert.Equal(engine.UserTaskTerminated, updatedUserTask.State)

	children[0].IsCompleted()
	children[1].IsTerminated()
	children[2].HasState(engine.InstanceStarted)

	// when escalation boundary event is triggered
	escalationBoundaryEvent := psAssert.IsWaitingAt("escalationBoundaryEvent")
	escalationBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	escalationBoundaryEvent.ExecuteTask()

	// then
	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 9)
	require.Len(piAssert.Jobs(), 5)
	require.Len(piAssert.Tasks(), 1)

	// when terminated process instance task is executed
	children[2].ExecuteTasks()
	children[2].IsTerminated()
}

func (x multiInstanceTest) parallelServiceTask(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "multi-instance/parallel-service-task.bpmn", "parallelServiceTaskTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 3)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	serviceTask.HasState(engine.InstanceCreated)
	serviceTask.HasJob(engine.JobStartMultiInstance)
	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	serviceTask.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 6)

	children := serviceTask.Children()
	require.Len(children, 3)

	for _, child := range children {
		child.HasState(engine.InstanceStarted)
	}

	for _, child := range children {
		child.HasJob(engine.JobExecute)
		child.CompleteJob()

		child.HasState(engine.InstanceCompleted)
	}

	serviceTask.HasState(engine.InstanceStarted)
	serviceTask.HasJob(engine.JobEndMultiInstance)
	serviceTask.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 7)
	require.Len(piAssert.Jobs(), 5)
}

func (x multiInstanceTest) parallelServiceTaskLoopCardinalityDefined(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "multi-instance/parallel-service-task-loop-cardinality.bpmn", "parallelServiceTaskLoopCardinalityTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 6)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.HasState(engine.InstanceStarted)

	children := serviceTask.Children()
	require.Len(children, 3)

	for _, child := range children {
		child.HasState(engine.InstanceStarted)
	}

	for _, child := range children {
		child.HasJob(engine.JobExecute)
		child.CompleteJob()

		child.HasState(engine.InstanceCompleted)
	}

	serviceTask.HasState(engine.InstanceStarted)
	serviceTask.HasJob(engine.JobEndMultiInstance)
	serviceTask.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 7)
	require.Len(piAssert.Jobs(), 4)
}

func (x multiInstanceTest) parallelServiceTaskLoopCardinalityZero(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "multi-instance/parallel-service-task.bpmn", "parallelServiceTaskTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 3)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	serviceTask.HasState(engine.InstanceCreated)
	serviceTask.HasJob(engine.JobStartMultiInstance)
	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 0,
			},
		},
	})

	serviceTask.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 3)

	children := serviceTask.Children()
	require.Len(children, 0)

	serviceTask.HasState(engine.InstanceStarted)
	serviceTask.HasJob(engine.JobEndMultiInstance)
	serviceTask.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 4)
	require.Len(piAssert.Jobs(), 2)
}

func (x multiInstanceTest) parallelSubProcess(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "multi-instance/parallel-sub-process.bpmn", "parallelSubProcessTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 3)

	subProcess := psAssert.IsWaitingAt("subProcess")

	subProcess.HasState(engine.InstanceCreated)
	subProcess.HasJob(engine.JobStartMultiInstance)
	subProcess.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	subProcess.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 6)

	children := subProcess.Children()
	require.Len(children, 3)

	for _, child := range children {
		child.HasState(engine.InstanceStarted)
	}

	for _, child := range children {
		child.HasTask(engine.TaskContinueExecution)
		child.ExecuteTask()

		child.HasState(engine.InstanceCompleted)
	}

	require.Len(piAssert.ElementInstances(), 12)

	subProcess.HasJob(engine.JobEndMultiInstance)
	subProcess.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 13)
	require.Len(piAssert.Jobs(), 2)
	require.Len(piAssert.Tasks(), 3)
}

func (x multiInstanceTest) parallelSubProcessMessageBoundary(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "multi-instance/parallel-sub-process-boundary.bpmn", "parallelSubProcessBoundaryTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 4)

	subProcess := psAssert.IsWaitingAt("subProcess")
	subProcess.HasState(engine.InstanceCreated)

	messageBoundaryEvent := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent.HasJob(engine.JobSubscribeMessage)
	messageBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "ck",
			MessageName:           t.Name(),
		},
	})

	subProcess.HasJob(engine.JobStartMultiInstance)
	subProcess.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	subProcess.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 7)

	// when sub process #1 continues execution
	multiInstanceActivity, ok := subProcess.Child()
	require.True(ok)

	multiInstanceActivity.HasState(engine.InstanceStarted)

	multiInstanceActivity.HasTask(engine.TaskContinueExecution)
	multiInstanceActivity.ExecuteTask()

	// then
	children := subProcess.Children()

	children[0].IsCompleted()
	children[1].HasState(engine.InstanceStarted)
	children[2].HasState(engine.InstanceStarted)

	// when message is sent
	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// when message boundary event is triggered
	messageBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	messageBoundaryEvent.ExecuteTask()

	// then
	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 10)
	require.Len(piAssert.Jobs(), 2)
	require.Len(piAssert.Tasks(), 4)

	children[1].IsTerminated()
	children[2].IsTerminated()
}

func (x multiInstanceTest) parallelSubProcessSuspension(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "multi-instance/parallel-sub-process-boundary.bpmn", "parallelSubProcessBoundaryTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	processInstance := piAssert.ProcessInstance()

	require.Len(piAssert.ElementInstances(), 4)

	subProcess := psAssert.IsWaitingAt("subProcess")
	subProcess.HasState(engine.InstanceCreated)

	messageBoundaryEvent := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent.HasJob(engine.JobSubscribeMessage)
	messageBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "ck",
			MessageName:           t.Name(),
		},
	})

	subProcess.HasJob(engine.JobStartMultiInstance)
	subProcess.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	subProcess.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 7)

	if err := x.e.SuspendProcessInstance(context.Background(), engine.SuspendProcessInstanceCmd{
		Partition: processInstance.Partition,
		Id:        processInstance.Id,
		WorkerId:  testWorkerId,
	}); err != nil {
		t.Fatalf("failed to suspend process engine: %v", err)
	}

	// when sub process #1 continues execution
	multiInstanceActivity, ok := subProcess.Child()
	require.True(ok)

	multiInstanceActivity.HasState(engine.InstanceStarted)

	multiInstanceActivity.HasTask(engine.TaskContinueExecution)
	multiInstanceActivity.ExecuteTask()

	// then
	children := subProcess.Children()

	children[0].HasState(engine.InstanceSuspended)
	children[1].HasState(engine.InstanceStarted)
	children[2].HasState(engine.InstanceStarted)

	if err := x.e.ResumeProcessInstance(context.Background(), engine.ResumeProcessInstanceCmd{
		Partition: processInstance.Partition,
		Id:        processInstance.Id,
		WorkerId:  testWorkerId,
	}); err != nil {
		t.Fatalf("failed to resume process engine: %v", err)
	}

	// then
	children[0].IsCompleted()
	children[1].HasState(engine.InstanceStarted)
	children[2].HasState(engine.InstanceStarted)

	// when message is sent
	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// when message boundary event is triggered
	messageBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	messageBoundaryEvent.ExecuteTask()

	// then
	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 10)
	require.Len(piAssert.Jobs(), 2)
	require.Len(piAssert.Tasks(), 4)

	children[1].IsTerminated()
	children[2].IsTerminated()
}

func (x multiInstanceTest) parallelUserTask(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "multi-instance/parallel-user-task.bpmn", "parallelUserTaskTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 3)

	userTask := psAssert.IsWaitingAt("userTask")

	userTask.HasState(engine.InstanceCreated)
	userTask.HasJob(engine.JobStartMultiInstance)
	userTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	userTask.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 6)
	require.Len(piAssert.UserTasks(), 3)

	children := userTask.Children()
	require.Len(children, 3)

	for _, child := range children {
		child.HasState(engine.InstanceStarted)
	}

	for _, child := range children {
		child.CompleteUserTask()

		child.HasState(engine.InstanceCompleted)
	}

	userTask.HasState(engine.InstanceStarted)
	userTask.HasJob(engine.JobEndMultiInstance)
	userTask.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 7)
	require.Len(piAssert.Jobs(), 2)
	require.Len(piAssert.UserTasks(), 3)
}

func (x multiInstanceTest) sequentialCallActivity(t *testing.T) {
	require := require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "multi-instance/sequential-call-activity.bpmn", "sequentialCallActivityTest"),
		mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 3)

	callActivity := psAssert.IsWaitingAt("callActivity")

	callActivity.HasState(engine.InstanceCreated)
	callActivity.HasJob(engine.JobStartMultiInstance)
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	callActivity.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 6)

	children := callActivity.Children()
	require.Len(children, 3)

	children[0].HasState(engine.InstanceStarted)
	children[1].HasState(engine.InstanceCreated)
	children[2].HasState(engine.InstanceCreated)

	children[0].CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	children[0].HasJob(engine.JobPassVariables)
	children[0].CompleteJob()

	children[0].HasState(engine.InstanceCompleted)
	children[1].HasState(engine.InstanceStarted)
	children[2].HasState(engine.InstanceCreated)

	children[1].CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	children[1].HasJob(engine.JobPassVariables)
	children[1].CompleteJob()

	children[0].HasState(engine.InstanceCompleted)
	children[1].HasState(engine.InstanceCompleted)
	children[2].HasState(engine.InstanceStarted)

	children[2].CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	children[2].HasJob(engine.JobPassVariables)
	children[2].CompleteJob()

	callActivity.HasState(engine.InstanceStarted)
	callActivity.HasJob(engine.JobEndMultiInstance)
	callActivity.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.Children(), 3)
	require.Len(piAssert.ElementInstances(), 7)
	require.Len(piAssert.Jobs(), 8)
}

func (x multiInstanceTest) sequentialCallActivityMessageBoundary(t *testing.T) {
	require := require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "multi-instance/sequential-call-activity-boundary.bpmn", "sequentialCallActivityBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 4)

	callActivity := psAssert.IsWaitingAt("callActivity")
	callActivity.HasState(engine.InstanceCreated)

	messageBoundaryEvent := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent.HasJob(engine.JobSubscribeMessage)
	messageBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "ck",
			MessageName:           t.Name(),
		},
	})

	callActivity.HasJob(engine.JobStartMultiInstance)
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	callActivity.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 7)

	// when call activity #1 calls process
	multiInstanceActivity, ok := callActivity.Child()
	require.True(ok)

	multiInstanceActivity.HasState(engine.InstanceStarted)

	multiInstanceActivity.HasJob(engine.JobCallProcess)
	multiInstanceActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	// when service task #1 is executed
	child, childScope, ok := piAssert.Child()
	require.True(ok)

	childScope.IsWaitingAt("serviceTask").CompleteJob()

	multiInstanceActivity.HasJob(engine.JobPassVariables)
	multiInstanceActivity.CompleteJob()

	// then
	child.IsCompleted()

	// when call activity #2 calls process
	multiInstanceActivity, ok = callActivity.Child()
	require.True(ok)

	multiInstanceActivity.HasState(engine.InstanceStarted)

	multiInstanceActivity.HasJob(engine.JobCallProcess)
	multiInstanceActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	// then
	child, _, ok = piAssert.Child()
	require.True(ok)

	child.HasState(engine.InstanceStarted)

	// when message is sent
	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// when message boundary event is triggered
	messageBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	messageBoundaryEvent.ExecuteTask()

	// then
	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 8)
	require.Len(piAssert.Jobs(), 5)
	require.Len(piAssert.Tasks(), 1)

	// when terminated process instance task is executed
	child.ExecuteTasks()
	child.IsTerminated()
}

func (x multiInstanceTest) sequentialCallActivityMessageSubscriptionCancelation(t *testing.T) {
	require := require.New(t)

	process, _ :=
		mustCreateProcess(t, x.e, "multi-instance/sequential-call-activity-boundary.bpmn", "sequentialCallActivityBoundaryTest"),
		mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 4)

	callActivity := psAssert.IsWaitingAt("callActivity")
	callActivity.HasState(engine.InstanceCreated)

	messageBoundaryEvent := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent.HasJob(engine.JobSubscribeMessage)
	messageBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "ck",
			MessageName:           t.Name(),
		},
	})

	callActivity.HasJob(engine.JobStartMultiInstance)
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 0,
			},
		},
	})

	callActivity.HasState(engine.InstanceStarted)

	callActivity.HasJob(engine.JobEndMultiInstance)
	callActivity.CompleteJob()

	callActivity.IsCompleted()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 5)
	require.Len(piAssert.Jobs(), 3)

	processInstance := piAssert.ProcessInstance()

	messageSubscriptions, err := x.e.CreateQuery().QueryMessageSubscriptions(context.Background(), engine.MessageSubscriptionCriteria{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
	})
	if err != nil {
		t.Fatalf("failed to query message subscriptions: %v", err)
	}

	require.Len(messageSubscriptions, 0)
}

func (x multiInstanceTest) sequentialServiceTask(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "multi-instance/sequential-service-task.bpmn", "sequentialServiceTaskTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 3)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	serviceTask.HasState(engine.InstanceCreated)
	serviceTask.HasJob(engine.JobStartMultiInstance)
	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	serviceTask.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 6)

	children := serviceTask.Children()
	require.Len(children, 3)

	children[0].HasState(engine.InstanceStarted)
	children[1].HasState(engine.InstanceCreated)
	children[2].HasState(engine.InstanceCreated)

	children[0].CompleteJob()

	children[0].HasState(engine.InstanceCompleted)
	children[1].HasState(engine.InstanceStarted)
	children[2].HasState(engine.InstanceCreated)

	children[1].CompleteJob()

	children[0].HasState(engine.InstanceCompleted)
	children[1].HasState(engine.InstanceCompleted)
	children[2].HasState(engine.InstanceStarted)

	children[2].CompleteJob()

	serviceTask.HasState(engine.InstanceStarted)
	serviceTask.HasJob(engine.JobEndMultiInstance)
	serviceTask.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 7)
	require.Len(piAssert.Jobs(), 5)
}

func (x multiInstanceTest) sequentialSubProcess(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "multi-instance/sequential-sub-process.bpmn", "sequentialSubProcessTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 3)

	subProcess := psAssert.IsWaitingAt("subProcess")

	subProcess.HasState(engine.InstanceCreated)
	subProcess.HasJob(engine.JobStartMultiInstance)
	subProcess.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	subProcess.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 6)

	children := subProcess.Children()
	require.Len(children, 3)

	children[0].HasState(engine.InstanceStarted)
	children[1].HasState(engine.InstanceCreated)
	children[2].HasState(engine.InstanceCreated)

	children[0].HasTask(engine.TaskContinueExecution)
	children[0].ExecuteTask()

	children[0].HasState(engine.InstanceCompleted)
	children[1].HasState(engine.InstanceStarted)
	children[2].HasState(engine.InstanceCreated)

	children[1].HasTask(engine.TaskContinueExecution)
	children[1].ExecuteTask()

	children[0].HasState(engine.InstanceCompleted)
	children[1].HasState(engine.InstanceCompleted)
	children[2].HasState(engine.InstanceStarted)

	children[2].HasTask(engine.TaskContinueExecution)
	children[2].ExecuteTask()

	subProcess.HasState(engine.InstanceStarted)
	subProcess.HasJob(engine.JobEndMultiInstance)
	subProcess.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 13)
	require.Len(piAssert.Jobs(), 2)
	require.Len(piAssert.Tasks(), 3)
}

func (x multiInstanceTest) sequentialUserTask(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "multi-instance/sequential-user-task.bpmn", "sequentialUserTaskTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	require.Len(piAssert.ElementInstances(), 3)

	userTask := psAssert.IsWaitingAt("userTask")

	userTask.HasState(engine.InstanceCreated)
	userTask.HasJob(engine.JobStartMultiInstance)
	userTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MultiInstance: &engine.MultiInstance{
				LoopCardinality: 3,
			},
		},
	})

	userTask.HasState(engine.InstanceStarted)

	require.Len(piAssert.ElementInstances(), 6)

	children := userTask.Children()
	require.Len(children, 3)

	children[0].HasState(engine.InstanceStarted)
	children[1].HasState(engine.InstanceCreated)
	children[2].HasState(engine.InstanceCreated)

	require.Len(piAssert.UserTasks(), 1)

	children[0].CompleteUserTask()

	children[0].HasState(engine.InstanceCompleted)
	children[1].HasState(engine.InstanceStarted)
	children[2].HasState(engine.InstanceCreated)

	require.Len(piAssert.UserTasks(), 2)

	children[1].CompleteUserTask()

	children[0].HasState(engine.InstanceCompleted)
	children[1].HasState(engine.InstanceCompleted)
	children[2].HasState(engine.InstanceStarted)

	require.Len(piAssert.UserTasks(), 3)

	children[2].CompleteUserTask()

	userTask.HasState(engine.InstanceStarted)
	userTask.HasJob(engine.JobEndMultiInstance)
	userTask.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 7)
	require.Len(piAssert.Jobs(), 2)
	require.Len(piAssert.UserTasks(), 3)
}
