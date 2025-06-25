package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func newParallelGatewayTest(t *testing.T, e engine.Engine) parallelGatewayTest {
	return parallelGatewayTest{
		e: e,

		parallelServiceTasksTest: mustCreateProcess(t, e, "gateway/parallel-service-tasks.bpmn", "parallelServiceTasksTest"),
		parallelTest:             mustCreateProcess(t, e, "gateway/parallel.bpmn", "parallelTest"),
	}
}

type parallelGatewayTest struct {
	e engine.Engine

	parallelServiceTasksTest engine.Process
	parallelTest             engine.Process
}

func (x parallelGatewayTest) gateway(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.parallelTest)

	piAssert.IsWaitingAt("join")
	piAssert.ExecuteTask()

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

	piAssert := mustCreateProcessInstance(t, x.e, x.parallelServiceTasksTest)

	piAssert.IsWaitingAt("serviceTaskA")
	piAssert.CompleteJob()
	piAssert.IsWaitingAt("serviceTaskB")
	piAssert.CompleteJob()

	piAssert.IsWaitingAt("join")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 8)

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 8)
}
