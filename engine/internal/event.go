package internal

import (
	"time"

	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

type EventEntity struct {
	Partition time.Time

	ElementInstanceId int32

	CreatedAt             time.Time
	CreatedBy             string
	MessageCorrelationKey pgtype.Text
	MessageName           pgtype.Text
	SignalName            pgtype.Text
	Time                  pgtype.Timestamp
	TimeCycle             pgtype.Text
	TimeDuration          pgtype.Text
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
	MessageName     pgtype.Text
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

	// SelectByMessageName selects a not suspended event definition for the message name.
	//
	// If no such event definition exists, [pgx.ErrNoRows] is returned.
	SelectByMessageName(messageName string) (*EventDefinitionEntity, error)

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
		case model.ElementMessageStartEvent, model.ElementSignalStartEvent, model.ElementTimerStartEvent:
			eventDefinition.IsSuspended = true
		default:
			continue
		}

		suspended = append(suspended, eventDefinition)
	}

	return ctx.EventDefinitions().UpdateBatch(suspended)
}
