package internal

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type SignalEntity struct {
	Partition time.Time
	Id        int32

	Name            string
	SentAt          time.Time
	SentBy          string
	SubscriberCount int
}

type SignalRepository interface {
	Insert(*SignalEntity) error
}

type SignalEventEntity struct {
	ElementId int32

	ProcessId int32

	BpmnElementId string
	BpmnProcessId string
	IsSuspended   bool
	Name          string
	Version       string
}

type SignalEventRepository interface {
	Insert([]*SignalEventEntity) error
	Select(elementId int32) (*SignalEventEntity, error)
	SelectByBpmnProcessId(bpmnProcessId string) ([]*SignalEventEntity, error)
	Update([]*SignalEventEntity) error
}

type SignalSubscriptionEntity struct {
	Id int64

	ElementId         int32
	ElementInstanceId pgtype.Int4
	Partition         pgtype.Date // Partition of the related process and element instance
	ProcessId         int32
	ProcessInstanceId pgtype.Int4

	CreatedAt time.Time
	CreatedBy string
	Name      string
}

type SignalSubscriptionRepository interface {
	Insert([]*SignalSubscriptionEntity) error
}

func suspendSignalEvents(ctx Context, bpmnProcessId string) error {
	events, err := ctx.SignalEvents().SelectByBpmnProcessId(bpmnProcessId)
	if err != nil {
		return err
	}

	var suspendedEvents []*SignalEventEntity
	for _, event := range events {
		if event.IsSuspended {
			continue
		}

		event.IsSuspended = true
		suspendedEvents = append(suspendedEvents, event)
	}

	return ctx.SignalEvents().Update(suspendedEvents)
}
