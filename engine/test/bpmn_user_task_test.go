package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userTaskTest struct {
	e engine.Engine
}

func (x userTaskTest) startEnd(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "user-task/start-end.bpmn", "userTaskStartEndTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("userTask")

	userTask := piAssert.UserTask()

	updatedUserTask, err := x.e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{
		Partition: userTask.Partition,
		Id:        userTask.Id,

		Revision: userTask.Revision,

		IsCompleted: true,
		WorkerId:    testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to update user task: %v", err)
	}

	assert.Equal(engine.UserTaskCompleted, updatedUserTask.State)

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 4)

	assert.Equal("userTask", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)
	assert.Equal("endEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
}

func (x userTaskTest) errorBoundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "user-task/error-boundary.bpmn", "userTaskErrorBoundaryTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("userTask")

	userTask := piAssert.UserTask()

	updatedUserTask, err := x.e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{
		Partition: userTask.Partition,
		Id:        userTask.Id,

		Revision: userTask.Revision,

		ErrorCode: "testErrorCode",
		WorkerId:  testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to update user task: %v", err)
	}

	assert.Equal(engine.UserTaskTerminated, updatedUserTask.State)

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal("userTask", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("errorBoundaryEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("errorEnd", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
}

func (x userTaskTest) escalationBoundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "user-task/escalation-boundary.bpmn", "userTaskEscalationBoundaryTest", engine.CreateProcessCmd{
		Escalations: []engine.EscalationDefinition{
			{BpmnElementId: "escalationBoundaryEvent", EscalationCode: "interrupting"},
			{BpmnElementId: "escalationBoundaryEventNonInterrupting", EscalationCode: "non-interrupting"},
		},
	})

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("userTask")

	userTask := piAssert.UserTask()

	updatedUserTask, err := x.e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{
		Partition: userTask.Partition,
		Id:        userTask.Id,

		Revision: userTask.Revision,

		EscalationCode: "interrupting",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to update user task: %v", err)
	}

	assert.Equal(engine.UserTaskTerminated, updatedUserTask.State)

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 6)

	assert.Equal("userTask", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[3].State)
	assert.Equal("escalationBoundaryEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
	assert.Equal("escalationEndEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
}

func (x userTaskTest) escalationBoundaryNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "user-task/escalation-boundary.bpmn", "userTaskEscalationBoundaryTest", engine.CreateProcessCmd{
		Escalations: []engine.EscalationDefinition{
			{BpmnElementId: "escalationBoundaryEvent", EscalationCode: "interrupting"},
			{BpmnElementId: "escalationBoundaryEventNonInterrupting", EscalationCode: "non-interrupting"},
		},
	})

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("userTask")

	userTask := piAssert.UserTask()

	updatedUserTask, err := x.e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{
		Partition: userTask.Partition,
		Id:        userTask.Id,

		Revision: userTask.Revision,

		EscalationCode: "non-interrupting",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to update user task: %v", err)
	}

	assert.Equal(engine.UserTaskStarted, updatedUserTask.State)

	piAssert.IsNotCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal("userTask", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceStarted, elementInstances[2].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("escalationBoundaryEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[4].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[5].State)
	assert.Equal("escalationEndNonInterrupting", elementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[6].State)
}
