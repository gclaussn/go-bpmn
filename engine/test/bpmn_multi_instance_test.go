package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/require"
)

type multiInstanceTest struct {
	e engine.Engine
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
		child.CompleteJob()
		child.HasState(engine.InstanceCompleted)
	}

	serviceTask.HasState(engine.InstanceStarted)
	serviceTask.HasJob(engine.JobEndMultiInstance)
	serviceTask.CompleteJob()

	serviceTask.HasState(engine.InstanceCompleted)

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)
}
