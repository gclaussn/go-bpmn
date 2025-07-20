package mem

import (
	"time"

	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type eventRepository struct {
	partitions map[string][]internal.EventEntity
}

func (r *eventRepository) Insert(entity *internal.EventEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]

	entity.Id = int32(len(entities) + 1)

	r.partitions[key] = append(entities, *entity)
	return nil
}

type eventDefinitionRepository struct {
	entities map[int32]internal.EventDefinitionEntity
}

func (r *eventDefinitionRepository) InsertBatch(entities []*internal.EventDefinitionEntity) error {
	for _, entity := range entities {
		r.entities[entity.ElementId] = *entity
	}
	return nil
}

func (r *eventDefinitionRepository) Select(elementId int32) (*internal.EventDefinitionEntity, error) {
	for _, e := range r.entities {
		if e.ElementId == elementId {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *eventDefinitionRepository) SelectByBpmnProcessId(bpmnProcessId string) ([]*internal.EventDefinitionEntity, error) {
	var results []*internal.EventDefinitionEntity
	for _, e := range r.entities {
		if e.BpmnProcessId == bpmnProcessId {
			results = append(results, &e)
		}
	}
	return results, nil
}

func (r *eventDefinitionRepository) SelectBySignalName(signalName string) ([]*internal.EventDefinitionEntity, error) {
	var results []*internal.EventDefinitionEntity
	for _, e := range r.entities {
		if e.SignalName.Valid && e.SignalName.String == signalName && !e.IsSuspended {
			results = append(results, &e)
		}
	}
	return results, nil
}

func (r *eventDefinitionRepository) UpdateBatch(entities []*internal.EventDefinitionEntity) error {
	for _, e := range entities {
		r.entities[e.ElementId] = *e
	}
	return nil
}

type eventVariableRepository struct {
	partitions map[string][]internal.EventVariableEntity
}

func (r *eventVariableRepository) InsertBatch(entities []*internal.EventVariableEntity) error {
	for _, entity := range entities {
		key := entity.Partition.Format(time.DateOnly)
		entities := r.partitions[key]

		entity.Id = int32(len(entities) + 1)

		r.partitions[key] = append(entities, *entity)
	}
	return nil
}

func (r *eventVariableRepository) SelectByEvent(partition time.Time, eventId int32) ([]*internal.EventVariableEntity, error) {
	var results []*internal.EventVariableEntity

	key := partition.Format(time.DateOnly)
	for _, e := range r.partitions[key] {
		if e.EventId == eventId {
			results = append(results, &e)
		}
	}

	return results, nil
}
