package internal

import (
	"github.com/jackc/pgx/v5/pgtype"
)

type TimerEventEntity struct {
	ElementId int32

	ProcessId int32

	BpmnElementId string
	BpmnProcessId string
	IsSuspended   bool
	Time          pgtype.Timestamp
	TimeCycle     pgtype.Text
	TimeDuration  pgtype.Text
	Version       string
}

type TimerEventRepository interface {
	Insert([]*TimerEventEntity) error
	Select(elementId int32) (*TimerEventEntity, error)
	SelectByBpmnProcessId(bpmnProcessId string) ([]*TimerEventEntity, error)
	Update([]*TimerEventEntity) error
}

func suspendTimerEvents(ctx Context, bpmnProcessId string) error {
	events, err := ctx.TimerEvents().SelectByBpmnProcessId(bpmnProcessId)
	if err != nil {
		return err
	}

	var suspendedEvents []*TimerEventEntity
	for _, event := range events {
		if event.IsSuspended {
			continue
		}

		event.IsSuspended = true
		suspendedEvents = append(suspendedEvents, event)
	}

	return ctx.TimerEvents().Update(suspendedEvents)
}
