package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type SignalSubscriptionEntity struct {
	Id int64

	ElementId         int32
	ElementInstanceId int32
	Partition         time.Time // Partition of the related process and element instance
	ProcessId         int32
	ProcessInstanceId int32

	CreatedAt time.Time
	CreatedBy string
	Name      string
}

type SignalSubscriptionRepository interface {
	DeleteByName(name string) ([]*SignalSubscriptionEntity, error)
	Insert(*SignalSubscriptionEntity) error
}

func SendSignal(ctx Context, cmd engine.SendSignalCmd) (engine.SignalEvent, error) {
	signalSubscriptions, err := ctx.SignalSubscriptions().DeleteByName(cmd.Name)
	if err != nil {
		return engine.SignalEvent{}, err
	}

	eventDefinitions, err := ctx.EventDefinitions().SelectBySignalName(cmd.Name)
	if err != nil {
		return engine.SignalEvent{}, err
	}

	// encrypt variables
	encryption := ctx.Options().Encryption

	for variableName, data := range cmd.Variables {
		if data == nil {
			continue
		}
		if err := encryption.EncryptData(data); err != nil {
			return engine.SignalEvent{}, fmt.Errorf("failed to encrypt variable %s: %v", variableName, err)
		}
	}

	// insert events
	events := make(map[string]*EventEntity)
	for _, eventDefinition := range eventDefinitions {
		if eventDefinition.BpmnElementType != model.ElementSignalStartEvent {
			continue
		}

		key := ctx.Date().Format(time.DateOnly)
		if event, ok := events[key]; ok {
			event.SignalSubscriberCount = pgtype.Int4{Int32: event.SignalSubscriberCount.Int32 + 1, Valid: true}
			continue
		}

		events[key] = &EventEntity{
			Partition: ctx.Date(),

			CreatedAt:             ctx.Time(),
			CreatedBy:             cmd.WorkerId,
			SignalName:            pgtype.Text{String: cmd.Name, Valid: true},
			SignalSubscriberCount: pgtype.Int4{Int32: 1, Valid: true},
		}
	}
	for _, signalSubscription := range signalSubscriptions {
		key := signalSubscription.Partition.Format(time.DateOnly)
		if event, ok := events[key]; ok {
			event.SignalSubscriberCount = pgtype.Int4{Int32: event.SignalSubscriberCount.Int32 + 1, Valid: true}
			continue
		}

		events[key] = &EventEntity{
			Partition: signalSubscription.Partition,

			CreatedAt:             ctx.Time(),
			CreatedBy:             cmd.WorkerId,
			SignalName:            pgtype.Text{String: cmd.Name, Valid: true},
			SignalSubscriberCount: pgtype.Int4{Int32: 1, Valid: true},
		}
	}

	mainEvent := EventEntity{
		Partition: ctx.Date(),

		CreatedAt:             ctx.Time(),
		CreatedBy:             cmd.WorkerId,
		SignalName:            pgtype.Text{String: cmd.Name, Valid: true},
		SignalSubscriberCount: pgtype.Int4{Int32: int32(len(events)), Valid: true},
	}

	if err := ctx.Events().Insert(&mainEvent); err != nil {
		return engine.SignalEvent{}, err
	}

	for _, event := range events {
		if err := ctx.Events().Insert(event); err != nil {
			return engine.SignalEvent{}, err
		}
	}

	// insert trigger event tasks
	triggerEventTasks := make([]*TaskEntity, 0, len(signalSubscriptions)+len(eventDefinitions))
	for _, signalSubscription := range signalSubscriptions {
		key := signalSubscription.Partition.Format(time.DateOnly)
		event := events[key]

		triggerEventTasks = append(triggerEventTasks, &TaskEntity{
			Partition: signalSubscription.Partition,

			ElementId:         pgtype.Int4{Int32: signalSubscription.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: signalSubscription.ElementInstanceId, Valid: true},
			EventId:           pgtype.Int4{Int32: event.Id, Valid: true},
			ProcessId:         pgtype.Int4{Int32: signalSubscription.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: signalSubscription.ProcessInstanceId, Valid: true},

			CreatedAt: event.CreatedAt,
			CreatedBy: event.CreatedBy,
			DueAt:     event.CreatedAt,
			Type:      engine.TaskTriggerEvent,

			Instance: TriggerEventTask{},
		})
	}
	for _, eventDefinition := range eventDefinitions {
		if eventDefinition.BpmnElementType != model.ElementSignalStartEvent {
			continue
		}

		key := ctx.Date().Format(time.DateOnly)
		event := events[key]

		triggerEventTasks = append(triggerEventTasks, &TaskEntity{
			Partition: ctx.Date(),

			ElementId: pgtype.Int4{Int32: eventDefinition.ElementId, Valid: true},
			EventId:   pgtype.Int4{Int32: event.Id, Valid: true},
			ProcessId: pgtype.Int4{Int32: eventDefinition.ProcessId, Valid: true},

			CreatedAt: event.CreatedAt,
			CreatedBy: event.CreatedBy,
			DueAt:     event.CreatedAt,
			Type:      engine.TaskTriggerEvent,

			Instance: TriggerEventTask{},
		})
	}

	if err := ctx.Tasks().InsertBatch(triggerEventTasks); err != nil {
		return engine.SignalEvent{}, err
	}

	// insert variables per event
	variables := make([]*EventVariableEntity, 0, len(events)*len(cmd.Variables))
	for _, event := range events {
		for variableName, data := range cmd.Variables {
			if data != nil {
				variables = append(variables, &EventVariableEntity{
					Partition: event.Partition,

					EventId: event.Id,

					Encoding:    pgtype.Text{String: data.Encoding, Valid: true},
					IsEncrypted: pgtype.Bool{Bool: data.IsEncrypted, Valid: true},
					Name:        variableName,
					Value:       pgtype.Text{String: data.Value, Valid: true},
				})
			} else {
				variables = append(variables, &EventVariableEntity{
					Partition: event.Partition,

					EventId: event.Id,

					Name: variableName,
				})
			}
		}
	}

	if err := ctx.EventVariables().InsertBatch(variables); err != nil {
		return engine.SignalEvent{}, err
	}

	return mainEvent.SignalEvent(), nil
}

func triggerSignalCatchEvent(ctx Context, task *TaskEntity, process *ProcessEntity) error {
	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to trigger signal catch event",
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

func triggerSignalStartEvent(ctx Context, task *TaskEntity, process *ProcessEntity) error {
	eventDefinition, err := ctx.EventDefinitions().Select(task.ElementId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to trigger signal start event",
			Detail: fmt.Sprintf("event definition %d could not be found", task.ElementId.Int32),
		}
	}
	if err != nil {
		return err
	}

	if eventDefinition.IsSuspended {
		return nil
	}

	processInstance := ProcessInstanceEntity{
		Partition: ctx.Date(),

		ProcessId: process.Id,

		BpmnProcessId:  process.BpmnProcessId,
		CreatedAt:      ctx.Time(),
		CreatedBy:      ctx.Options().EngineId,
		StartedAt:      pgtype.Timestamp{Time: ctx.Time(), Valid: true},
		State:          engine.InstanceStarted,
		StateChangedBy: ctx.Options().EngineId,
		Version:        process.Version,
	}

	if err := ctx.ProcessInstances().Insert(&processInstance); err != nil {
		return err
	}

	if err := enqueueProcessInstance(ctx, &processInstance); err != nil {
		return fmt.Errorf("failed to enqueue process instance: %v", err)
	}

	scope := process.graph.createProcessScope(&processInstance)

	execution, err := process.graph.createExecutionAt(&scope, eventDefinition.BpmnElementId)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create execution",
			Detail: err.Error(),
		}
	}

	execution.EventId = task.EventId

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		process:          process,
		processInstance:  &processInstance,
	}

	executions := []*ElementInstanceEntity{&scope, &execution}
	if err := ec.continueExecutions(ctx, executions); err != nil {
		return err
	}

	variables, err := ctx.EventVariables().SelectByEvent(task.Partition, task.EventId.Int32)
	if err != nil {
		return err
	}

	for _, variable := range variables {
		if !variable.Value.Valid {
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
