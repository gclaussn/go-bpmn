package mem

import (
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/gclaussn/go-bpmn/model"
)

type elementRepository struct {
	entities []internal.ElementEntity

	eventDefinitions eventDefinitionRepository
}

func (r *elementRepository) InsertBatch(entities []*internal.ElementEntity) error {
	for _, entity := range entities {
		entity.Id = int32(len(r.entities) + 1)
		r.entities = append(r.entities, *entity)
	}
	return nil
}

func (r *elementRepository) SelectByProcessId(processId int32) ([]*internal.ElementEntity, error) {
	var results []*internal.ElementEntity
	for _, e := range r.entities {
		if e.ProcessId == processId {
			results = append(results, &e)
		}
	}
	return results, nil
}

func (r *elementRepository) Query(c engine.ElementCriteria, o engine.QueryOptions) ([]engine.Element, error) {
	var (
		results []engine.Element
		offset  int
		limit   int
	)

	for _, e := range r.entities {
		if c.ProcessId != 0 && c.ProcessId != e.ProcessId {
			continue
		}

		if c.BpmnElementId != "" && c.BpmnElementId != e.BpmnElementId {
			continue
		}

		if offset < o.Offset {
			offset++
			continue
		}

		result := e.Element()
		result.EventDefinition = r.getEventDefinition(e.Id)

		results = append(results, result)
		limit++

		if o.Limit > 0 && limit == o.Limit {
			break
		}
	}

	return results, nil
}

func (r *elementRepository) getEventDefinition(elementId int32) *engine.EventDefinition {
	entity, ok := r.eventDefinitions.entities[elementId]
	if !ok {
		return nil
	}

	var timer *engine.Timer
	switch entity.BpmnElementType {
	case model.ElementTimerCatchEvent, model.ElementTimerStartEvent:
		timer = &engine.Timer{
			Time:         entity.Time.Time,
			TimeCycle:    entity.TimeCycle.String,
			TimeDuration: engine.ISO8601Duration(entity.TimeDuration.String),
		}
	}

	return &engine.EventDefinition{
		IsSuspended: entity.IsSuspended,
		MessageName: entity.MessageName.String,
		SignalName:  entity.SignalName.String,
		Timer:       timer,
	}
}
