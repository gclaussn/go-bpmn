package internal

import (
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

type EventEntity struct {
	Partition time.Time
	Id        int32

	CreatedAt         time.Time
	CreatedBy         string
	SignalName        pgtype.Text
	SignalSubscribers pgtype.Int4
	Time              pgtype.Timestamp
	TimeCycle         pgtype.Text
	TimeDuration      pgtype.Text
}

func (e EventEntity) SignalEvent() engine.SignalEvent {
	return engine.SignalEvent{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		Name:            e.SignalName.String,
		SentAt:          e.CreatedAt,
		SentBy:          e.CreatedBy,
		SubscriberCount: int(e.SignalSubscribers.Int32),
	}
}

type EventRepository interface {
	Insert(*EventEntity) error
}

type EventDefinitionEntity struct {
	ElementId int32

	ProcessId int32

	BpmnElementId   string
	BpmnElementType model.ElementType
	BpmnProcessId   string
	IsSuspended     bool
	SignalName      pgtype.Text
	Time            pgtype.Timestamp
	TimeCycle       pgtype.Text
	TimeDuration    pgtype.Text
	Version         string
}

type EventDefinitionRepository interface {
	InsertBatch([]*EventDefinitionEntity) error
	Select(elementId int32) (*EventDefinitionEntity, error)
	SelectByBpmnProcessId(bpmnProcessId string) ([]*EventDefinitionEntity, error)

	// SelectBySignalName selects all not suspended event definitions for the signal name.
	SelectBySignalName(signalName string) ([]*EventDefinitionEntity, error)

	UpdateBatch([]*EventDefinitionEntity) error
}

func suspendEventDefinitions(ctx Context, bpmnProcessId string) error {
	eventDefinitions, err := ctx.EventDefinitions().SelectByBpmnProcessId(bpmnProcessId)
	if err != nil {
		return err
	}

	var suspended []*EventDefinitionEntity
	for _, eventDefinition := range eventDefinitions {
		if eventDefinition.IsSuspended {
			continue
		}

		switch eventDefinition.BpmnElementType {
		case model.ElementSignalStartEvent, model.ElementTimerStartEvent:
			eventDefinition.IsSuspended = true
		default:
			continue
		}

		suspended = append(suspended, eventDefinition)
	}

	return ctx.EventDefinitions().UpdateBatch(suspended)
}
