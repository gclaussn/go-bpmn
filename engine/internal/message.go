package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type MessageEntity struct {
	Id int64

	CorrelationKey string
	CreatedAt      time.Time
	CreatedBy      string
	ExpiresAt      pgtype.Timestamp
	Name           string
}

type MessageRepository interface {
	Insert(*MessageEntity) error
}

type MessageSubscriptionEntity struct {
	Id int64

	Partition time.Time // Partition of the related process and element instance

	ElementId         int32
	ElementInstanceId int32
	ProcessId         int32
	ProcessInstanceId int32

	CorrelationKey string
	CreatedAt      time.Time
	CreatedBy      string
	Name           string
}

type MessageSubscriptionRepository interface {
	// DeleteByNameAndCorrelationKey deletes a message subscription by name and correlation key.
	//
	// If no such message subscription exists, [pgx.ErrNoRows] is returned.
	DeleteByNameAndCorrelationKey(name string, correlationKey string) (*MessageSubscriptionEntity, error)

	Insert(*MessageSubscriptionEntity) error
}

type MessageVariableEntity struct {
	Id int64

	MessageId int64

	Encoding    pgtype.Text // NULL, when a process instance variable should be deleted
	IsEncrypted pgtype.Bool // NULL, when a process instance variable should be deleted
	Name        string
	Value       pgtype.Text // NULL, when a process instance variable should be deleted
}

type MessageVariableRepository interface {
	DeleteByMessageId(message int64) ([]*MessageVariableEntity, error)
	InsertBatch([]*MessageVariableEntity) error
}

func SendMessage(ctx Context, cmd engine.SendMessageCmd) (engine.MessageCorrelation, error) {
	// encrypt variables
	encryption := ctx.Options().Encryption

	for variableName, data := range cmd.Variables {
		if data == nil {
			continue
		}
		if err := encryption.EncryptData(data); err != nil {
			return engine.MessageCorrelation{}, fmt.Errorf("failed to encrypt variable %s: %v", variableName, err)
		}
	}

	var (
		event     *EventEntity
		eventTask *TaskEntity
	)

	// insert message
	expiresAt := cmd.ExpirationTimer.Calculate(ctx.Time())

	message := MessageEntity{
		CorrelationKey: cmd.CorrelationKey,
		CreatedAt:      ctx.Time(),
		CreatedBy:      cmd.WorkerId,
		ExpiresAt:      pgtype.Timestamp{Time: expiresAt, Valid: !cmd.ExpirationTimer.IsZero()},
		Name:           cmd.Name,
	}

	if err := ctx.Messages().Insert(&message); err != nil {
		return engine.MessageCorrelation{}, err
	}

	// find message subscription
	messageSubscription, err := ctx.MessageSubscriptions().DeleteByNameAndCorrelationKey(cmd.Name, cmd.CorrelationKey)
	if err != nil && err != pgx.ErrNoRows {
		return engine.MessageCorrelation{}, err
	}

	if messageSubscription != nil {
		event = &EventEntity{
			Partition: messageSubscription.Partition,

			CreatedAt:             ctx.Time(),
			CreatedBy:             cmd.WorkerId,
			MessageCorrelationKey: pgtype.Text{String: cmd.CorrelationKey, Valid: true},
			MessageName:           pgtype.Text{String: cmd.Name, Valid: true},
		}

		if err := ctx.Events().Insert(event); err != nil {
			return engine.MessageCorrelation{}, err
		}

		eventVariables := make([]*EventVariableEntity, 0, len(cmd.Variables))
		for variableName, data := range cmd.Variables {
			if data != nil {
				eventVariables = append(eventVariables, &EventVariableEntity{
					Partition: event.Partition,

					EventId: event.Id,

					Encoding:    pgtype.Text{String: data.Encoding, Valid: true},
					IsEncrypted: pgtype.Bool{Bool: data.IsEncrypted, Valid: true},
					Name:        variableName,
					Value:       pgtype.Text{String: data.Value, Valid: true},
				})
			} else {
				eventVariables = append(eventVariables, &EventVariableEntity{
					Partition: event.Partition,

					EventId: event.Id,

					Name: variableName,
				})
			}
		}

		if err := ctx.EventVariables().InsertBatch(eventVariables); err != nil {
			return engine.MessageCorrelation{}, err
		}

		eventTask = &TaskEntity{
			Partition: event.Partition,

			ElementId:         pgtype.Int4{Int32: messageSubscription.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: messageSubscription.ElementInstanceId, Valid: true},
			EventId:           pgtype.Int4{Int32: event.Id, Valid: true},
			ProcessId:         pgtype.Int4{Int32: messageSubscription.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: messageSubscription.ProcessInstanceId, Valid: true},

			CreatedAt: ctx.Time(),
			CreatedBy: cmd.WorkerId,
			DueAt:     ctx.Time(),
			Type:      engine.TaskTriggerEvent,

			Instance: TriggerEventTask{},
		}

		if err := ctx.Tasks().Insert(eventTask); err != nil {
			return engine.MessageCorrelation{}, err
		}

		return engine.MessageCorrelation{
			IsCorrelated: true,

			CorrelationKey: cmd.CorrelationKey,
			Name:           cmd.Name,

			ElementId:         messageSubscription.ElementId,
			ElementInstanceId: messageSubscription.ElementInstanceId,
			ProcessId:         messageSubscription.ProcessId,
			ProcessInstanceId: messageSubscription.ProcessInstanceId,
		}, nil
	}

	// find message definition
	eventDefinition, err := ctx.EventDefinitions().SelectByMessageName(cmd.Name)
	if err != nil && err != pgx.ErrNoRows {
		return engine.MessageCorrelation{}, err
	}

	if eventDefinition != nil {
		event = &EventEntity{
			Partition: ctx.Date(),

			CreatedAt:             ctx.Time(),
			CreatedBy:             cmd.WorkerId,
			MessageCorrelationKey: pgtype.Text{String: cmd.CorrelationKey, Valid: true},
			MessageName:           pgtype.Text{String: cmd.Name, Valid: true},
		}

		if err := ctx.Events().Insert(event); err != nil {
			return engine.MessageCorrelation{}, err
		}

		process, err := ctx.ProcessCache().GetOrCacheById(ctx, eventDefinition.ProcessId)
		if err != nil {
			return engine.MessageCorrelation{}, err
		}

		processInstance := ProcessInstanceEntity{
			Partition: ctx.Date(),

			MessageId: pgtype.Int8{Int64: message.Id, Valid: true},
			ProcessId: eventDefinition.ProcessId,

			BpmnProcessId:  process.BpmnProcessId,
			CorrelationKey: pgtype.Text{String: cmd.CorrelationKey, Valid: true},
			CreatedAt:      ctx.Time(),
			CreatedBy:      cmd.WorkerId,
			StartedAt:      pgtype.Timestamp{Time: ctx.Time(), Valid: true},
			State:          engine.InstanceStarted,
			StateChangedBy: cmd.WorkerId,
			Version:        process.Version,
		}

		if err := ctx.ProcessInstances().Insert(&processInstance); err != nil {
			return engine.MessageCorrelation{}, err
		}

		if err := enqueueProcessInstance(ctx, &processInstance); err != nil {
			return engine.MessageCorrelation{}, fmt.Errorf("failed to enqueue process instance: %v", err)
		}

		scope := process.graph.createProcessScope(&processInstance)

		execution, err := process.graph.createExecutionAt(&scope, eventDefinition.BpmnElementId)
		if err != nil {
			return engine.MessageCorrelation{}, engine.Error{
				Type:   engine.ErrorProcessModel,
				Title:  "failed to create execution",
				Detail: err.Error(),
			}
		}

		execution.EventId = pgtype.Int4{Int32: event.Id, Valid: true}

		ec := executionContext{
			engineOrWorkerId: ctx.Options().EngineId,
			process:          process,
			processInstance:  &processInstance,
		}

		executions := []*ElementInstanceEntity{&scope, &execution}
		if err := ec.continueExecutions(ctx, executions); err != nil {
			return engine.MessageCorrelation{}, err
		}

		for variableName, data := range cmd.Variables {
			if data == nil {
				continue
			}

			if err := ctx.Variables().Insert(&VariableEntity{
				Partition: processInstance.Partition,

				ProcessId:         processInstance.ProcessId,
				ProcessInstanceId: processInstance.Id,

				CreatedAt:   processInstance.CreatedAt,
				CreatedBy:   processInstance.CreatedBy,
				Encoding:    data.Encoding,
				IsEncrypted: data.IsEncrypted,
				Name:        variableName,
				UpdatedAt:   processInstance.CreatedAt,
				UpdatedBy:   processInstance.CreatedBy,
				Value:       data.Value,
			}); err != nil {
				return engine.MessageCorrelation{}, err
			}
		}

		return engine.MessageCorrelation{
			IsCorrelated: true,

			CorrelationKey: cmd.CorrelationKey,
			Name:           cmd.Name,

			ElementId:         execution.ElementId,
			ElementInstanceId: execution.Id,
			ProcessId:         execution.ProcessId,
			ProcessInstanceId: execution.ProcessInstanceId,
		}, nil
	}

	// insert message variables
	messageVariables := make([]*MessageVariableEntity, 0, len(cmd.Variables))
	for variableName, data := range cmd.Variables {
		if data != nil {
			messageVariables = append(messageVariables, &MessageVariableEntity{
				MessageId: message.Id,

				Encoding:    pgtype.Text{String: data.Encoding, Valid: true},
				IsEncrypted: pgtype.Bool{Bool: data.IsEncrypted, Valid: true},
				Name:        variableName,
				Value:       pgtype.Text{String: data.Value, Valid: true},
			})
		} else {
			messageVariables = append(messageVariables, &MessageVariableEntity{
				MessageId: message.Id,

				Name: variableName,
			})
		}
	}

	if err := ctx.MessageVariables().InsertBatch(messageVariables); err != nil {
		return engine.MessageCorrelation{}, err
	}

	return engine.MessageCorrelation{
		IsCorrelated: false,

		CorrelationKey: cmd.CorrelationKey,
		Name:           cmd.Name,
	}, nil
}

func triggerMessageCatchEvent(ctx Context, task *TaskEntity, process *ProcessEntity) error {
	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to trigger message catch event",
			Detail: fmt.Sprintf("process instance %s/%d could not be found", task.Partition.Format(time.DateOnly), task.ProcessInstanceId.Int32),
		}
	}
	if err != nil {
		return err
	}

	if processInstance.EndedAt.Valid {
		return nil
	}

	execution, err := ctx.ElementInstances().Select(task.Partition, task.ElementInstanceId.Int32)
	if err != nil {
		return err
	}

	if execution.EndedAt.Valid {
		return nil
	}

	execution.EventId = task.EventId

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		process:          process,
		processInstance:  processInstance,
	}

	if err := ec.continueExecutions(ctx, []*ElementInstanceEntity{execution}); err != nil {
		if _, ok := err.(engine.Error); ok {
			task.Error = pgtype.Text{String: err.Error(), Valid: true}
		} else {
			return fmt.Errorf("failed to continue execution %+v: %v", execution, err)
		}
	}

	variables, err := ctx.EventVariables().SelectByEvent(task.Partition, task.EventId.Int32)
	if err != nil {
		return err
	}

	for _, variable := range variables {
		if !variable.Value.Valid {
			if err := ctx.Variables().Delete(&VariableEntity{ // with fields, needed for deletion
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
				Name:              variable.Name,
			}); err != nil {
				return err
			}

			continue
		}

		if err := ctx.Variables().Insert(&VariableEntity{
			Partition: processInstance.Partition,

			ProcessId:         processInstance.ProcessId,
			ProcessInstanceId: processInstance.Id,

			CreatedAt:   processInstance.CreatedAt,
			CreatedBy:   processInstance.CreatedBy,
			Encoding:    variable.Encoding.String,
			IsEncrypted: variable.IsEncrypted.Bool,
			Name:        variable.Name,
			UpdatedAt:   processInstance.CreatedAt,
			UpdatedBy:   processInstance.CreatedBy,
			Value:       variable.Value.String,
		}); err != nil {
			return err
		}
	}

	return nil
}
