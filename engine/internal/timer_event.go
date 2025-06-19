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
	timerEvents, err := ctx.TimerEvents().SelectByBpmnProcessId(bpmnProcessId)
	if err != nil {
		return err
	}

	var timerEventsToUpdate []*TimerEventEntity
	for _, timerEvent := range timerEvents {
		if timerEvent.IsSuspended {
			continue
		}

		timerEvent.IsSuspended = true
		timerEventsToUpdate = append(timerEventsToUpdate, timerEvent)
	}

	return ctx.TimerEvents().Update(timerEventsToUpdate)
}
