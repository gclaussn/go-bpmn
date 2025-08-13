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
	ExpiresAt      time.Time
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

		eventTask = &TaskEntity{
			Partition: messageSubscription.Partition,

			ElementId:         pgtype.Int4{Int32: messageSubscription.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: messageSubscription.ElementInstanceId, Valid: true},
			ProcessId:         pgtype.Int4{Int32: messageSubscription.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: messageSubscription.ProcessInstanceId, Valid: true},

			CreatedAt: ctx.Time(),
			CreatedBy: cmd.WorkerId,
			DueAt:     ctx.Time(),
			Type:      engine.TaskTriggerEvent,

			Instance: TriggerEventTask{},
		}
	}

	if messageSubscription == nil {
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

			eventTask = &TaskEntity{
				Partition: messageSubscription.Partition,

				ElementId: pgtype.Int4{Int32: eventDefinition.ElementId, Valid: true},
				ProcessId: pgtype.Int4{Int32: eventDefinition.ProcessId, Valid: true},

				CreatedAt: ctx.Time(),
				CreatedBy: cmd.WorkerId,
				DueAt:     ctx.Time(),
				Type:      engine.TaskTriggerEvent,

				Instance: TriggerEventTask{},
			}
		}
	}

	if event != nil {
		// insert event
		if err := ctx.Events().Insert(event); err != nil {
			return engine.MessageCorrelation{}, err
		}

		// insert event task
		eventTask.EventId = pgtype.Int4{Int32: event.Id, Valid: true}

		if err := ctx.Tasks().Insert(eventTask); err != nil {
			return engine.MessageCorrelation{}, err
		}

		// insert event variables
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

		return engine.MessageCorrelation{
			IsCorrelated: true,

			CorrelationKey: cmd.CorrelationKey,
			Name:           cmd.Name,

			ElementId:         eventTask.ElementId.Int32,
			ElementInstanceId: eventTask.ElementInstanceId.Int32,
			ProcessId:         eventTask.ProcessId.Int32,
			ProcessInstanceId: eventTask.ProcessInstanceId.Int32,
		}, nil
	}

	// insert message
	message := MessageEntity{
		CorrelationKey: cmd.CorrelationKey,
		CreatedAt:      ctx.Time(),
		CreatedBy:      cmd.WorkerId,
		ExpiresAt:      cmd.ExpirationTimer.Calculate(ctx.Time()),
		Name:           cmd.Name,
	}

	if err := ctx.Messages().Insert(&message); err != nil {
		return engine.MessageCorrelation{}, err
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
