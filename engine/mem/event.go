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

func (r *eventDefinitionRepository) SelectByMessageName(messageName string) (*internal.EventDefinitionEntity, error) {
	for _, e := range r.entities {
		if e.MessageName.Valid && e.MessageName.String == messageName && !e.IsSuspended {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
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
