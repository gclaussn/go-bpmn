package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type UserTaskEntity struct {
	Partition time.Time
	Id        int32

	Revision int32

	ElementId         int32
	ElementInstanceId int32
	ProcessId         int32
	ProcessInstanceId int32

	BpmnElementId  string
	CorrelationKey pgtype.Text
	CreatedAt      time.Time
	CreatedBy      string
	State          engine.UserTaskState
	Tags           pgtype.Text
	UpdatedAt      time.Time
	UpdatedBy      string
}

func (e UserTaskEntity) UserTask() engine.UserTask {
	return engine.UserTask{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		Revision: e.Revision,

		ElementId:         e.ElementId,
		ElementInstanceId: e.ElementInstanceId,
		ProcessId:         e.ProcessId,
		ProcessInstanceId: e.ProcessInstanceId,

		BpmnElementId:  e.BpmnElementId,
		CorrelationKey: e.CorrelationKey.String,
		CreatedAt:      e.CreatedAt,
		CreatedBy:      e.CreatedBy,
		State:          e.State,
		Tags:           unmarshalTags(e.Tags),
		UpdatedAt:      e.UpdatedAt,
		UpdatedBy:      e.UpdatedBy,
	}
}

type UserTaskRepository interface {
	Insert(*UserTaskEntity) error
	Select(partition time.Time, id int32) (*UserTaskEntity, error)
	Update(*UserTaskEntity) error

	Query(engine.UserTaskCriteria, engine.QueryOptions) ([]engine.UserTask, error)
}

func UpdateUserTask(ctx Context, cmd engine.UpdateUserTaskCmd) (engine.UserTask, error) {
	processInstance, err := ctx.ProcessInstances().SelectByUserTask(time.Time(cmd.Partition), cmd.Id)
	if err == pgx.ErrNoRows {
		return engine.UserTask{}, engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to update user task",
			Detail: fmt.Sprintf("user task %s/%d could not be found", cmd.Partition, cmd.Id),
		}
	}
	if err != nil {
		return engine.UserTask{}, err
	}

	userTask, err := ctx.UserTasks().Select(processInstance.Partition, cmd.Id)
	if err != nil {
		return engine.UserTask{}, err
	}
	if userTask.State != engine.UserTaskStarted {
		return engine.UserTask{}, engine.Error{
			Type:  engine.ErrorConflict,
			Title: "failed to update user task",
			Detail: fmt.Sprintf(
				"user task %s/%d is not in state %s, but %s",
				cmd.Partition,
				cmd.Id,
				engine.UserTaskStarted,
				userTask.State,
			),
		}
	}
	if userTask.Revision != cmd.Revision {
		return engine.UserTask{}, engine.Error{
			Type:  engine.ErrorConflict,
			Title: "failed to update user task",
			Detail: fmt.Sprintf(
				"user task %s/%d is not in revision %d, but %d",
				cmd.Partition,
				cmd.Id,
				cmd.Revision,
				userTask.Revision,
			),
		}
	}

	actions := 0
	if cmd.IsCompleted {
		actions++
	}
	if cmd.ErrorCode != "" {
		actions++
	}
	if cmd.EscalationCode != "" {
		actions++
	}

	if actions > 1 {
		return engine.UserTask{}, engine.Error{
			Type:   engine.ErrorValidation,
			Title:  "failed to update user task",
			Detail: "expected to either complete user task, throw error or escalate user task",
		}
	}

	// update tags
	if len(cmd.Tags) != 0 {
		oldTags := unmarshalTags(userTask.Tags)
		newTags := append(oldTags, cmd.Tags...)

		tags, err := marshalTags(newTags)
		if err != nil {
			return engine.UserTask{}, err
		}

		userTask.Tags = tags
	}

	execution, err := ctx.ElementInstances().Select(userTask.Partition, userTask.ElementInstanceId)
	if err != nil {
		return engine.UserTask{}, err
	}

	elementVariables, err := mapElementVariables(ctx, execution, cmd.ElementVariables, cmd.WorkerId)
	if err != nil {
		return engine.UserTask{}, err
	}

	processVariables, err := mapProcessVariables(ctx, processInstance, cmd.ProcessVariables, cmd.WorkerId)
	if err != nil {
		return engine.UserTask{}, err
	}

	// continue execution
	if actions == 1 {
		process, err := ctx.ProcessCache().GetOrCacheById(ctx, execution.ProcessId)
		if err != nil {
			return engine.UserTask{}, err
		}

		scope, err := ctx.ElementInstances().Select(execution.Partition, execution.ParentId.Int32)
		if err != nil {
			return engine.UserTask{}, err
		}

		execution.parent = scope

		ec := executionContext{
			engineOrWorkerId: cmd.WorkerId,
			executions:       []*ElementInstanceEntity{scope, execution},
			process:          process,
			processInstance:  processInstance,
		}

		if err := ec.handleUserTask(ctx, userTask, cmd); err != nil {
			return engine.UserTask{}, err
		}
	}

	// set or delete element variables
	for _, variable := range elementVariables {
		if variable.ElementId.Valid {
			if err := ctx.Variables().Upsert(variable); err != nil {
				return engine.UserTask{}, err
			}
		} else {
			if err := ctx.Variables().Delete(variable); err != nil {
				return engine.UserTask{}, err
			}
		}
	}

	// set or delete process variables
	for _, variable := range processVariables {
		if variable.ProcessId != 0 {
			if err := ctx.Variables().Upsert(variable); err != nil {
				return engine.UserTask{}, err
			}
		} else {
			if err := ctx.Variables().Delete(variable); err != nil {
				return engine.UserTask{}, err
			}
		}
	}

	// update user task
	userTask.Revision++
	userTask.UpdatedAt = ctx.Time()
	userTask.UpdatedBy = cmd.WorkerId

	if err := ctx.UserTasks().Update(userTask); err != nil {
		return engine.UserTask{}, err
	}

	return userTask.UserTask(), nil
}
