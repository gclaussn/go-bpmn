package internal

import (
	"time"

	"github.com/gclaussn/go-bpmn/engine"
)

type SignalEntity struct {
	Partition time.Time
	Id        int32

	Name            string
	SentAt          time.Time
	SentBy          string
	SubscriberCount int
}

func (e SignalEntity) Signal() engine.Signal {
	return engine.Signal{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		Name:            e.Name,
		SentAt:          e.SentAt,
		SentBy:          e.SentBy,
		SubscriberCount: e.SubscriberCount,
	}
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
	InsertBatch([]*SignalEventEntity) error
	SelectByBpmnProcessId(bpmnProcessId string) ([]*SignalEventEntity, error)
	SelectByNameAndNotSuspended(name string) ([]*SignalEventEntity, error)
	UpdateBatch([]*SignalEventEntity) error
}

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

func SendSignal(ctx Context, cmd engine.SendSignalCmd) (engine.Signal, error) {
	signalSubscriptions, err := ctx.SignalSubscriptions().DeleteByName(cmd.Name)
	if err != nil {
		return engine.Signal{}, err
	}

	signalEvents, err := ctx.SignalEvents().SelectByNameAndNotSuspended(cmd.Name)
	if err != nil {
		return engine.Signal{}, err
	}

	signal := SignalEntity{
		Partition: ctx.Date(),

		Name:            cmd.Name,
		SentAt:          ctx.Time(),
		SentBy:          cmd.WorkerId,
		SubscriberCount: len(signalSubscriptions) + len(signalEvents),
	}

	if err := ctx.Signals().Insert(&signal); err != nil {
		return engine.Signal{}, err
	}

	return signal.Signal(), nil
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

	return ctx.SignalEvents().UpdateBatch(suspendedEvents)
}
