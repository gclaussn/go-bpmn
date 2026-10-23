package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
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

	require.Len(piAssert.ElementInstances(), 12)

	subProcess.HasJob(engine.JobEndMultiInstance)
	subProcess.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 13)
	require.Len(piAssert.Jobs(), 2)
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

	require.Len(piAssert.ElementInstances(), 12)

	subProcess.HasState(engine.InstanceStarted)
	subProcess.HasJob(engine.JobEndMultiInstance)
	subProcess.CompleteJob()

	piAssert.IsCompleted()

	require.Len(piAssert.ElementInstances(), 13)
	require.Len(piAssert.Jobs(), 2)
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
