package internal

import (
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
	messageSubscription, err := ctx.MessageSubscriptions().DeleteByNameAndCorrelationKey(cmd.Name, cmd.CorrelationKey)
	if err != nil && err != pgx.ErrNoRows {
		return engine.MessageCorrelation{}, err
	}

	if err == pgx.ErrNoRows {

	}

	eventDefinition, err := ctx.EventDefinitions().SelectByMessageName(cmd.Name)
	if err != nil && err != pgx.ErrNoRows {
		return engine.MessageCorrelation{}, err
	}

	if err == pgx.ErrNoRows {

	}
}
