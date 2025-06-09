package internal

import (
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProcessInstanceQueueEntity struct {
	BpmnProcessId string

	Parallelism   int // Parallelism of the latest process version
	ActiveCount   int // Number of active process instances (started or due StartProcessInstanceTask exists)
	QueuedCount   int // Number of queued process instances
	HeadPartition pgtype.Date
	HeadId        pgtype.Int4
	TailPartition pgtype.Date
	TailId        pgtype.Int4
}

// MustDequeue determines if a process instance must be dequeued and started.
// This is true, when a queued process instance exists and capacity for an active process instance is available.
func (e *ProcessInstanceQueueEntity) MustDequeue() bool {
	return e.QueuedCount > 0 && (e.Parallelism <= 0 || e.ActiveCount < e.Parallelism)
}

// MustEnqueue determines if a process instance must be enqueued.
// This is true, when no capacity for an active process instance is available.
func (e *ProcessInstanceQueueEntity) MustEnqueue() bool {
	return e.Parallelism > 0 && e.ActiveCount >= e.Parallelism
}

type ProcessInstanceQueueElementEntity struct {
	Partition time.Time
	Id        int32

	ProcessId int32

	NextPartition pgtype.Date
	NextId        pgtype.Int4
	PrevPartition pgtype.Date
	PrevId        pgtype.Int4
}

type ProcessInstanceQueueRepository interface {
	InsertElement(*ProcessInstanceQueueElementEntity) error

	// Select selects and locks the queue for a BPMN process ID.
	Select(bpmnProcessId string) (*ProcessInstanceQueueEntity, error)

	SelectElement(partition time.Time, id int32) (*ProcessInstanceQueueElementEntity, error)
	Update(*ProcessInstanceQueueEntity) error
	UpdateElement(*ProcessInstanceQueueElementEntity) error

	// Upsert inserts or updates the queue for a BPMN process ID.
	// If the queue already exists, only the parallelism value is updated.
	// Moreover the current active and queued process instance counts are returned and made available through the entity.
	Upsert(*ProcessInstanceQueueEntity) error
}

func enqueueProcessInstance(ctx Context, processInstance *ProcessInstanceEntity) error {
	queue, err := ctx.ProcessInstanceQueues().Select(processInstance.BpmnProcessId)
	if err != nil {
		return err
	}

	if queue.TailId.Valid {
		oldTail, err := ctx.ProcessInstanceQueues().SelectElement(queue.TailPartition.Time, queue.TailId.Int32)
		if err != nil {
			return err
		}

		oldTail.NextPartition = pgtype.Date{Time: processInstance.Partition, Valid: true}
		oldTail.NextId = pgtype.Int4{Int32: processInstance.Id, Valid: true}

		if err := ctx.ProcessInstanceQueues().UpdateElement(oldTail); err != nil {
			return err
		}

		newTail := ProcessInstanceQueueElementEntity{
			Partition: processInstance.Partition,
			Id:        processInstance.Id,

			ProcessId: processInstance.ProcessId,

			PrevPartition: pgtype.Date{Time: oldTail.Partition, Valid: true},
			PrevId:        pgtype.Int4{Int32: oldTail.Id, Valid: true},
		}

		if err := ctx.ProcessInstanceQueues().InsertElement(&newTail); err != nil {
			return err
		}
	} else if queue.MustEnqueue() {
		newHead := ProcessInstanceQueueElementEntity{
			Partition: processInstance.Partition,
			Id:        processInstance.Id,

			ProcessId: processInstance.ProcessId,
		}

		if err := ctx.ProcessInstanceQueues().InsertElement(&newHead); err != nil {
			return err
		}

		queue.HeadPartition = pgtype.Date{Time: processInstance.Partition, Valid: true}
		queue.HeadId = pgtype.Int4{Int32: processInstance.Id, Valid: true}
	}

	if queue.HeadId.Valid {
		processInstance.StartedAt = pgtype.Timestamp{}
		processInstance.State = engine.InstanceQueued

		if err := ctx.ProcessInstances().Update(processInstance); err != nil {
			return err
		}

		queue.QueuedCount = queue.QueuedCount + 1
		queue.TailPartition = pgtype.Date{Time: processInstance.Partition, Valid: true}
		queue.TailId = pgtype.Int4{Int32: processInstance.Id, Valid: true}
	} else {
		queue.ActiveCount = queue.ActiveCount + 1
	}

	return ctx.ProcessInstanceQueues().Update(queue)
}

func dequeueProcessInstance(ctx Context, processInstance *ProcessInstanceEntity) error {
	queue, err := ctx.ProcessInstanceQueues().Select(processInstance.BpmnProcessId)
	if err != nil {
		return err
	}

	queue.ActiveCount = queue.ActiveCount - 1

	if queue.MustDequeue() {
		head, err := ctx.ProcessInstanceQueues().SelectElement(queue.HeadPartition.Time, queue.HeadId.Int32)
		if err != nil {
			return err
		}

		if head.NextId.Valid {
			queue.HeadId = head.NextId
			queue.HeadPartition = head.NextPartition
		} else {
			queue.HeadId = pgtype.Int4{}
			queue.HeadPartition = pgtype.Date{}
			queue.TailId = pgtype.Int4{}
			queue.TailPartition = pgtype.Date{}
		}

		startProcessInstance := TaskEntity{
			Partition: head.Partition,

			ProcessId:         pgtype.Int4{Int32: head.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: head.Id, Valid: true},

			CreatedAt: ctx.Time(),
			CreatedBy: processInstance.StateChangedBy,
			DueAt:     ctx.Time(),
			Type:      engine.TaskStartProcessInstance,

			Instance: StartProcessInstanceTask{},
		}

		if err := ctx.Tasks().Insert(&startProcessInstance); err != nil {
			return err
		}

		queue.ActiveCount = queue.ActiveCount + 1
		queue.QueuedCount = queue.QueuedCount - 1
	}

	return ctx.ProcessInstanceQueues().Update(queue)
}
