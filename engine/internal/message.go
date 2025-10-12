package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type MessageEntity struct {
	Id int64

	CorrelationKey string
	CreatedAt      time.Time
	CreatedBy      string
	ExpiresAt      pgtype.Timestamp
	IsConflict     bool
	IsCorrelated   bool
	Name           string
	UniqueKey      pgtype.Text
}

func (e MessageEntity) Message() engine.Message {
	return engine.Message{
		Id: e.Id,

		CorrelationKey: e.CorrelationKey,
		CreatedAt:      e.CreatedAt,
		CreatedBy:      e.CreatedBy,
		ExpiresAt:      timeOrNil(e.ExpiresAt),
		IsCorrelated:   e.IsCorrelated,
		Name:           e.Name,
		UniqueKey:      e.UniqueKey.String,
	}
}

type MessageRepository interface {
	Insert(*MessageEntity) error
	Select(id int64) (*MessageEntity, error)

	// SelectBuffered selects a buffered (not correlated and not expired) message by name and correlation key.
	//
	// If no such message exists, [pgx.ErrNoRows] is returned.
	SelectBuffered(name string, correlationKey string, now time.Time) (*MessageEntity, error)

	Update(*MessageEntity) error

	Query(engine.MessageCriteria, engine.QueryOptions, time.Time) ([]engine.Message, error)
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
	Delete(*MessageSubscriptionEntity) error
	Insert(*MessageSubscriptionEntity) error

	// SelectByNameAndCorrelationKey selects a message subscription by name and correlation key.
	//
	// If no such message subscription exists, [pgx.ErrNoRows] is returned.
	SelectByNameAndCorrelationKey(name string, correlationKey string) (*MessageSubscriptionEntity, error)
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
	InsertBatch([]*MessageVariableEntity) error
	SelectByMessageId(messageId int64) ([]*MessageVariableEntity, error)
}

func SendMessage(ctx Context, cmd engine.SendMessageCmd) (engine.Message, error) {
	// encrypt variables
	encryption := ctx.Options().Encryption

	for variableName, data := range cmd.Variables {
		if data == nil {
			continue
		}
		if err := encryption.EncryptData(data); err != nil {
			return engine.Message{}, fmt.Errorf("failed to encrypt variable %s: %v", variableName, err)
		}
	}

	// insert message
	expiresAt := ctx.Time()
	if cmd.ExpirationTimer != nil {
		v, err := evaluateTimer(*cmd.ExpirationTimer, ctx.Time())
		if err != nil {
			return engine.Message{}, engine.Error{
				Type:   engine.ErrorValidation,
				Title:  "failed to evaluate expiration timer",
				Detail: err.Error(),
			}
		}

		expiresAt = v
	}

	message := &MessageEntity{
		CorrelationKey: cmd.CorrelationKey,
		CreatedAt:      ctx.Time(),
		CreatedBy:      cmd.WorkerId,
		ExpiresAt:      pgtype.Timestamp{Time: expiresAt, Valid: true},
		Name:           cmd.Name,
		UniqueKey:      pgtype.Text{String: cmd.UniqueKey, Valid: cmd.UniqueKey != ""},
	}

	// insert message
	if err := ctx.Messages().Insert(message); err != nil {
		return engine.Message{}, err
	}

	if message.IsConflict {
		return message.Message(), nil
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
		return engine.Message{}, err
	}

	// find subscriber
	messageSubscription, eventDefinition, err := findMessageSubscriber(ctx, cmd)
	if err != nil {
		return engine.Message{}, err
	}

	if messageSubscription == nil && eventDefinition == nil {
		return message.Message(), nil
	}

	// insert trigger event task
	var triggerEventTask *TaskEntity
	if messageSubscription != nil {
		triggerEventTask = &TaskEntity{
			Partition: messageSubscription.Partition,

			ElementId:         pgtype.Int4{Int32: messageSubscription.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: messageSubscription.ElementInstanceId, Valid: true},
			ProcessId:         pgtype.Int4{Int32: messageSubscription.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: messageSubscription.ProcessInstanceId, Valid: true},

			CreatedAt: message.CreatedAt,
			CreatedBy: message.CreatedBy,
			DueAt:     message.CreatedAt,
			Type:      engine.TaskTriggerEvent,

			Instance: TriggerEventTask{MessageId: message.Id},
		}

		// delete message subscription
		if err := ctx.MessageSubscriptions().Delete(messageSubscription); err != nil {
			return engine.Message{}, err
		}
	} else {
		triggerEventTask = &TaskEntity{
			Partition: ctx.Date(),

			ElementId: pgtype.Int4{Int32: eventDefinition.ElementId, Valid: true},
			ProcessId: pgtype.Int4{Int32: eventDefinition.ProcessId, Valid: true},

			CreatedAt: message.CreatedAt,
			CreatedBy: message.CreatedBy,
			DueAt:     message.CreatedAt,
			Type:      engine.TaskTriggerEvent,

			Instance: TriggerEventTask{MessageId: message.Id, Timer: cmd.ExpirationTimer},
		}
	}

	if err := ctx.Tasks().Insert(triggerEventTask); err != nil {
		return engine.Message{}, err
	}

	// update message
	message.ExpiresAt = pgtype.Timestamp{}
	message.IsCorrelated = true

	if err := ctx.Messages().Update(message); err != nil {
		return engine.Message{}, err
	}

	return message.Message(), nil
}

func findMessageSubscriber(ctx Context, cmd engine.SendMessageCmd) (*MessageSubscriptionEntity, *EventDefinitionEntity, error) {
	messageSubscription, err := ctx.MessageSubscriptions().SelectByNameAndCorrelationKey(cmd.Name, cmd.CorrelationKey)
	if err != nil && err != pgx.ErrNoRows {
		return nil, nil, err
	}

	if messageSubscription != nil {
		return messageSubscription, nil, nil
	}

	eventDefinition, err := ctx.EventDefinitions().SelectByMessageName(cmd.Name)
	if err != nil && err != pgx.ErrNoRows {
		return nil, nil, err
	}

	if eventDefinition != nil && eventDefinition.BpmnElementType == model.ElementMessageStartEvent {
		return nil, eventDefinition, nil
	}

	return nil, nil, nil
}

func triggerMessageCatchEvent(ctx Context, task *TaskEntity, messageId int64) error {
	message, err := ctx.Messages().Select(messageId)
	if err != nil {
		return err
	}

	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err != nil {
		return err
	}

	if processInstance.EndedAt.Valid {
		message.ExpiresAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		return ctx.Messages().Update(message)
	}

	execution, err := ctx.ElementInstances().Select(task.Partition, task.ElementInstanceId.Int32)
	if err != nil {
		return err
	}

	if execution.EndedAt.Valid {
		message.ExpiresAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		return ctx.Messages().Update(message)
	}

	process, err := ctx.ProcessCache().GetOrCacheById(ctx, task.ProcessId.Int32)
	if err != nil {
		return err
	}

	ec := executionContext{
		engineOrWorkerId: message.CreatedBy,
		process:          process,
		processInstance:  processInstance,
	}

	if err := ec.continueExecutions(ctx, []*ElementInstanceEntity{execution}); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue execution %+v: %v", execution, err)
		}
	}

	messageVariables, err := ctx.MessageVariables().SelectByMessageId(messageId)
	if err != nil {
		return err
	}

	for _, variable := range messageVariables {
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

			CreatedAt:   ctx.Time(),
			CreatedBy:   message.CreatedBy,
			Encoding:    variable.Encoding.String,
			IsEncrypted: variable.IsEncrypted.Bool,
			Name:        variable.Name,
			UpdatedAt:   ctx.Time(),
			UpdatedBy:   message.CreatedBy,
			Value:       variable.Value.String,
		}); err != nil {
			return err
		}
	}

	event := EventEntity{
		Partition: execution.Partition,

		ElementInstanceId: execution.Id,

		CreatedAt:             ctx.Time(),
		CreatedBy:             message.CreatedBy,
		MessageCorrelationKey: pgtype.Text{String: message.CorrelationKey, Valid: true},
		MessageName:           pgtype.Text{String: message.Name, Valid: true},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	message.ExpiresAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
	return ctx.Messages().Update(message)
}

func triggerMessageStartEvent(ctx Context, task *TaskEntity, messageId int64, expireMessage bool) error {
	process, err := ctx.ProcessCache().GetOrCacheById(ctx, task.ProcessId.Int32)
	if err != nil {
		return err
	}

	startElement := process.graph.elementByElementId(task.ElementId.Int32)
	if startElement == nil {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to find start node",
			Detail: fmt.Sprintf("start node with ID %d could not be found", task.ElementId.Int32),
		}
	}

	message, err := ctx.Messages().Select(messageId)
	if err != nil {
		return err
	}

	var processInstance *ProcessInstanceEntity
	if task.ProcessInstanceId.Valid {
		selectedProcessInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
		if err != nil {
			return err
		}

		if selectedProcessInstance.EndedAt.Valid {
			return nil
		}

		processInstance = selectedProcessInstance
	} else {
		processInstance = &ProcessInstanceEntity{
			Partition: task.Partition,

			MessageId: pgtype.Int8{Int64: message.Id, Valid: true},
			ProcessId: process.Id,

			BpmnProcessId:  process.BpmnProcessId,
			CorrelationKey: pgtype.Text{String: message.CorrelationKey, Valid: !expireMessage},
			CreatedAt:      ctx.Time(),
			CreatedBy:      message.CreatedBy,
			StartedAt:      pgtype.Timestamp{Time: ctx.Time(), Valid: true},
			State:          engine.InstanceStarted,
			StateChangedBy: message.CreatedBy,
			Version:        process.Version,
		}

		if err := ctx.ProcessInstances().Insert(processInstance); err != nil {
			return err
		}

		// set ID of inserted process instance for a possible retry
		task.ProcessInstanceId = pgtype.Int4{Int32: processInstance.Id}

		if err := enqueueProcessInstance(ctx, processInstance); err != nil {
			return fmt.Errorf("failed to enqueue process instance: %v", err)
		}
	}

	scope := process.graph.createProcessScope(processInstance)

	execution, err := process.graph.createExecutionAt(&scope, startElement.Id)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create execution",
			Detail: err.Error(),
		}
	}

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		process:          process,
		processInstance:  processInstance,
	}

	executions := []*ElementInstanceEntity{&scope, &execution}
	if err := ec.continueExecutions(ctx, executions); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", executions, err)
		}
	}

	messagesVariables, err := ctx.MessageVariables().SelectByMessageId(messageId)
	if err != nil {
		return err
	}

	for _, variable := range messagesVariables {
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

	event := EventEntity{
		Partition: execution.Partition,

		ElementInstanceId: execution.Id,

		CreatedAt:             ctx.Time(),
		CreatedBy:             message.CreatedBy,
		MessageCorrelationKey: pgtype.Text{String: message.CorrelationKey, Valid: true},
		MessageName:           pgtype.Text{String: message.Name, Valid: true},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	if !expireMessage {
		return nil
	}

	message.ExpiresAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
	return ctx.Messages().Update(message)
}
