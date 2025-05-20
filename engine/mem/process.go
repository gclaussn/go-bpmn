package mem

import (
	"encoding/json"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type processRepository struct {
	entities []internal.ProcessEntity
}

func (r *processRepository) Insert(entity *internal.ProcessEntity) error {
	entity.Id = int32(len(r.entities) + 1)
	r.entities = append(r.entities, *entity)
	return nil
}

func (r *processRepository) Select(id int32) (*internal.ProcessEntity, error) {
	for _, e := range r.entities {
		if e.Id == id {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *processRepository) SelectByBpmnProcessIdAndVersion(bpmnProcessId string, version string) (*internal.ProcessEntity, error) {
	for _, e := range r.entities {
		if e.BpmnProcessId == bpmnProcessId && e.Version == version {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *processRepository) Query(c engine.ProcessCriteria, o engine.QueryOptions) ([]any, error) {
	var (
		results []any
		offset  int
		limit   int
	)

	for _, e := range r.entities {
		if c.Id != 0 && c.Id != e.Id {
			continue
		}

		if len(c.Tags) != 0 {
			if !e.Tags.Valid {
				continue
			}

			var tags map[string]string
			if e.Tags.Valid {
				_ = json.Unmarshal([]byte(e.Tags.String), &tags)
			}

			matches := true
			for name, value := range c.Tags {
				v, ok := tags[name]
				matches = matches && ok && v == value
			}

			if !matches {
				continue
			}
		}

		if offset < o.Offset {
			offset++
			continue
		}

		results = append(results, e.Process())
		limit++

		if o.Limit > 0 && limit == o.Limit {
			break
		}
	}

	return results, nil
}
