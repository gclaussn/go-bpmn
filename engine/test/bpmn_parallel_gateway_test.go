package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

type parallelGatewayTest struct {
	e engine.Engine
}

func (x parallelGatewayTest) gateway(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/parallel.bpmn", "parallelTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	join := psAssert.IsWaitingAt("join")
	join.HasTask(engine.TaskJoinParallelGateway)
	join.ExecuteTask()

	// execute remaining task
	tasks := piAssert.ExecuteTasks()
	assert.Len(tasks, 1)
	assert.Equal(engine.TaskJoinParallelGateway, tasks[0].Type)
	assert.Empty(tasks[0].Error)

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 6)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 6)
}

func (x parallelGatewayTest) serviceTasks(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "gateway/parallel-service-tasks.bpmn", "parallelServiceTasksTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	psAssert.IsWaitingAt("serviceTaskA").CompleteJob()
	psAssert.IsWaitingAt("serviceTaskB").CompleteJob()

	join := psAssert.IsWaitingAt("join")
	join.HasTask(engine.TaskJoinParallelGateway)
	join.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 8)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 8)
}
