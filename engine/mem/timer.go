package mem

import (
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type timerEventRepository struct {
	entities map[int32]internal.TimerEventEntity
}

func (r *timerEventRepository) Insert(entities []*internal.TimerEventEntity) error {
	for _, entity := range entities {
		r.entities[entity.ElementId] = *entity
	}
	return nil
}

func (r *timerEventRepository) Select(elementId int32) (*internal.TimerEventEntity, error) {
	for _, e := range r.entities {
		if e.ElementId == elementId {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *timerEventRepository) SelectByBpmnProcessId(bpmnProcessId string) ([]*internal.TimerEventEntity, error) {
	var results []*internal.TimerEventEntity
	for _, e := range r.entities {
		if e.BpmnProcessId == bpmnProcessId {
			results = append(results, &e)
		}
	}
	return results, nil
}

func (r *timerEventRepository) Update(entities []*internal.TimerEventEntity) error {
	for _, e := range entities {
		r.entities[e.ElementId] = *e
	}
	return nil
}
