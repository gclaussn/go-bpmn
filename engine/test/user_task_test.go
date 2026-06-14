package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateUserTask(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	newUserTask := func(e engine.Engine) (engine.UserTask, engine.ProcessInstance, engine.Process) {
		process := mustCreateProcess(t, e, "user-task/start-end.bpmn", "userTaskStartEndTest")

		processInstance, err := e.CreateProcessInstance(context.Background(), engine.CreateProcessInstanceCmd{
			BpmnProcessId:  process.BpmnProcessId,
			CorrelationKey: "ck",
			Version:        process.Version,
			WorkerId:       process.CreatedBy,
		})
		if err != nil {
			t.Fatalf("failed to create process instance: %v", err)
		}

		results, err := e.CreateQuery().QueryUserTasks(context.Background(), engine.UserTaskCriteria{
			Partition:         processInstance.Partition,
			ProcessInstanceId: processInstance.Id,
		})
		if err != nil {
			t.Fatalf("failed to query user task: %v", err)
		}

		require.Len(results, 1, "expected one user task")

		return results[0], processInstance, process
	}

	t.Run("returns error when user task not exists", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// when
				_, err := e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{})

				// then
				assert.IsTypef(engine.Error{}, err, "expected engine error")

				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorNotFound, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
			})
		}
	})

	t.Run("returns error when user task is completed", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				userTask, _, _ := newUserTask(e)

				cmd := engine.UpdateUserTaskCmd{
					Partition:   userTask.Partition,
					Id:          userTask.Id,
					Revision:    userTask.Revision,
					IsCompleted: true,
					WorkerId:    testWorkerId,
				}

				// when
				userTask, err := e.UpdateUserTask(context.Background(), cmd)
				if err != nil {
					t.Fatalf("failed to update user task: %v", err)
				}

				cmd.Revision = userTask.Revision

				_, err = e.UpdateUserTask(context.Background(), cmd)

				// then
				assert.IsTypef(engine.Error{}, err, "expected engine error")

				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorConflict, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.Contains(engineErr.Detail, "not in state STARTED, but COMPLETED")
			})
		}
	})

	t.Run("returns error when user task is not in revision", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				userTask, _, _ := newUserTask(e)

				// when
				_, err := e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{
					Partition: userTask.Partition,
					Id:        userTask.Id,
					Revision:  userTask.Revision + 1,
					WorkerId:  testWorkerId,
				})

				// then
				assert.IsTypef(engine.Error{}, err, "expected engine error")

				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorConflict, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.Contains(engineErr.Detail, "not in revision 2, but 1")
			})
		}
	})

	t.Run("returns error when multiple actions are requested", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				userTask, _, _ := newUserTask(e)

				// when
				_, err := e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{
					Partition:      userTask.Partition,
					Id:             userTask.Id,
					Revision:       userTask.Revision,
					ErrorCode:      "test-error",
					EscalationCode: "test-escalation",
					IsCompleted:    true,
					WorkerId:       testWorkerId,
				})

				// then
				assert.IsTypef(engine.Error{}, err, "expected engine error")

				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorValidation, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
			})
		}
	})

	t.Run("returns error when no error boundary event found", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				userTask, _, _ := newUserTask(e)

				// when
				_, err := e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{
					Partition: userTask.Partition,
					Id:        userTask.Id,
					Revision:  userTask.Revision,
					ErrorCode: "test-error",
					WorkerId:  testWorkerId,
				})

				// then
				assert.IsTypef(engine.Error{}, err, "expected engine error")

				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorExecution, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.Contains(engineErr.Detail, "error code test-error")
			})
		}
	})

	t.Run("returns error when no escalation boundary event found", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				userTask, _, _ := newUserTask(e)

				// when
				_, err := e.UpdateUserTask(context.Background(), engine.UpdateUserTaskCmd{
					Partition:      userTask.Partition,
					Id:             userTask.Id,
					Revision:       userTask.Revision,
					EscalationCode: "test-escalation",
					WorkerId:       testWorkerId,
				})

				// then
				assert.IsTypef(engine.Error{}, err, "expected engine error")

				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorExecution, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.Contains(engineErr.Detail, "escalation code test-escalation")
			})
		}
	})

	t.Run("update", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				userTask, processInstance, process := newUserTask(e)

				assert.Equal(processInstance.Partition, userTask.Partition)
				assert.NotEmpty(userTask.Id)

				assert.Equal(int32(1), userTask.Revision)

				assert.NotEmpty(userTask.ElementId)
				assert.NotEmpty(userTask.ElementInstanceId)
				assert.Equal(process.Id, userTask.ProcessId)
				assert.Equal(processInstance.Id, userTask.ProcessInstanceId)

				assert.Equal("userTask", userTask.BpmnElementId)
				assert.Equal(processInstance.CorrelationKey, userTask.CorrelationKey)
				assert.Equal(processInstance.CreatedAt, userTask.CreatedAt)
				assert.Equal(testWorkerId, userTask.CreatedBy)
				assert.Equal(engine.UserTaskStarted, userTask.State)
				assert.Empty(userTask.Tags)
				assert.Equal(processInstance.CreatedAt, userTask.UpdatedAt)
				assert.Equal(testWorkerId, userTask.UpdatedBy)

				// given
				cmd := engine.UpdateUserTaskCmd{
					Partition: userTask.Partition,
					Id:        userTask.Id,
					Revision:  userTask.Revision,
					ElementVariables: []engine.ElementVariable{
						{Name: "a", Data: &engine.Data{
							Encoding: "encoding-ea",
							Value:    "value-ea",
						}},
						{BpmnElementId: "userTaskStartEndTest", Name: "a", Data: &engine.Data{
							Encoding: "encoding-ea*",
							Value:    "value-ea*",
						}},
					},
					ProcessVariables: []engine.ProcessVariable{
						{Name: "a", Data: &engine.Data{
							Encoding: "encoding-pa",
							Value:    "value-pa",
						}},
					},
					Tags: []engine.Tag{
						{Name: "a", Value: "b"},
						{Name: "x", Value: "y"},
					},
					WorkerId: "update-worker",
				}

				// when
				userTask2, err := e.UpdateUserTask(context.Background(), cmd)
				if err != nil {
					t.Fatalf("failed to update user task: %v", err)
				}

				// then
				assert.Equal(int32(2), userTask2.Revision)

				assert.Equal(engine.UserTaskStarted, userTask2.State)
				assert.Equal("update-worker", userTask2.UpdatedBy)

				require.Len(userTask2.Tags, 2)
				assert.Equal("a", userTask2.Tags[0].Name)
				assert.Equal("b", userTask2.Tags[0].Value)
				assert.Equal("x", userTask2.Tags[1].Name)
				assert.Equal("y", userTask2.Tags[1].Value)

				elementVariables, err := e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
					Partition:         userTask.Partition,
					ElementInstanceId: userTask.ElementInstanceId,
				})
				if err != nil {
					t.Fatalf("failed to get element variables: %v", err)
				}

				require.Len(elementVariables, 2)
				assert.Equal("userTask", elementVariables[0].BpmnElementId)
				assert.Equal("a", elementVariables[0].Name)
				assert.Equal("encoding-ea", elementVariables[0].Data.Encoding)
				assert.Equal("value-ea", elementVariables[0].Data.Value)
				assert.Equal("userTaskStartEndTest", elementVariables[1].BpmnElementId)
				assert.Equal("a", elementVariables[1].Name)
				assert.Equal("encoding-ea*", elementVariables[1].Data.Encoding)
				assert.Equal("value-ea*", elementVariables[1].Data.Value)

				processVariables, err := e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
					Partition:         userTask.Partition,
					ProcessInstanceId: userTask.ProcessInstanceId,
				})
				if err != nil {
					t.Fatalf("failed to get process variables: %v", err)
				}

				require.Len(processVariables, 1)
				assert.Equal("a", processVariables[0].Name)
				assert.Equal("encoding-pa", processVariables[0].Data.Encoding)
				assert.Equal("value-pa", processVariables[0].Data.Value)

				// given
				cmd.Revision = userTask2.Revision
				cmd.Tags = []engine.Tag{
					{Name: "a", Value: "b*"},
					{Name: "c", Value: "d"},
					{Name: "x"},
				}

				// when
				userTask3, err := e.UpdateUserTask(context.Background(), cmd)
				if err != nil {
					t.Fatalf("failed to update user task: %v", err)
				}

				// then
				assert.Equal(int32(3), userTask3.Revision)

				require.Len(userTask3.Tags, 2)
				assert.Equal("a", userTask3.Tags[0].Name)
				assert.Equal("b*", userTask3.Tags[0].Value)
				assert.Equal("c", userTask3.Tags[1].Name)
				assert.Equal("d", userTask3.Tags[1].Value)
			})
		}
	})
}
