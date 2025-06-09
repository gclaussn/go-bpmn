package mem

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type processInstanceQueueRepository struct {
	queues                 map[string]internal.ProcessInstanceQueueEntity
	queueElementPartitions map[string][]internal.ProcessInstanceQueueElementEntity
}

func (r *processInstanceQueueRepository) InsertElement(entity *internal.ProcessInstanceQueueElementEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.queueElementPartitions[key]
	r.queueElementPartitions[key] = append(entities, *entity)
	return nil
}

func (r *processInstanceQueueRepository) Select(bpmmProcessId string) (*internal.ProcessInstanceQueueEntity, error) {
	entity, ok := r.queues[bpmmProcessId]
	if !ok {
		return nil, fmt.Errorf("failed to select process instance queue %s: %v", bpmmProcessId, pgx.ErrNoRows)
	}
	return &entity, nil
}

func (r *processInstanceQueueRepository) SelectElement(partition time.Time, id int32) (*internal.ProcessInstanceQueueElementEntity, error) {
	key := partition.Format(time.DateOnly)
	for _, e := range r.queueElementPartitions[key] {
		if e.Id == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("failed to select process instance queue element %s/%d: %v", key, id, pgx.ErrNoRows)
}

func (r *processInstanceQueueRepository) Update(entity *internal.ProcessInstanceQueueEntity) error {
	r.queues[entity.BpmnProcessId] = *entity
	return nil
}

func (r *processInstanceQueueRepository) UpdateElement(entity *internal.ProcessInstanceQueueElementEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.queueElementPartitions[key]
	for i, e := range entities {
		if e.Id == entity.Id {
			entities[i] = *entity
			return nil
		}
	}
	return fmt.Errorf("failed to update process instance queue element %s/%d: %v", key, entity.Id, pgx.ErrNoRows)
}

func (r *processInstanceQueueRepository) Upsert(entity *internal.ProcessInstanceQueueEntity) error {
	queue, ok := r.queues[entity.BpmnProcessId]
	if ok {
		queue.Parallelism = entity.Parallelism
		r.queues[entity.BpmnProcessId] = queue

		entity.ActiveCount = queue.ActiveCount
		entity.QueuedCount = queue.QueuedCount
	} else {
		r.queues[entity.BpmnProcessId] = *entity
	}
	return nil
}
