package mem

import (
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
)

type elementRepository struct {
	entities []internal.ElementEntity
}

func (r *elementRepository) Insert(entities []*internal.ElementEntity) error {
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

func (r *elementRepository) Query(c engine.ElementCriteria, o engine.QueryOptions) ([]any, error) {
	var (
		results []any
		offset  int
		limit   int
	)

	for _, e := range r.entities {
		if c.ProcessId != 0 && c.ProcessId != e.ProcessId {
			continue
		}

		if offset < o.Offset {
			offset++
			continue
		}

		results = append(results, e.Element())
		limit++

		if o.Limit > 0 && limit == o.Limit {
			break
		}
	}

	return results, nil
}
