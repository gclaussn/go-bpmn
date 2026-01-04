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
	for _, e := range r.entities {
		if e.BpmnProcessId == entity.BpmnProcessId && e.Version == entity.Version {
			return pgx.ErrNoRows // indicates a conflict
		}
	}

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

func (r *processRepository) Query(c engine.ProcessCriteria, o engine.QueryOptions) ([]engine.Process, error) {
	var (
		offset int
		limit  int
	)

	results := make([]engine.Process, 0)
	for _, e := range r.entities {
		if c.Id != 0 && c.Id != e.Id {
			continue
		}

		if len(c.Tags) != 0 {
			if !e.Tags.Valid {
				continue
			}

			var tagMap map[string]string
			if e.Tags.Valid {
				_ = json.Unmarshal([]byte(e.Tags.String), &tagMap)
			}

			matches := true
			for _, tag := range c.Tags {
				value, ok := tagMap[tag.Name]
				matches = matches && ok && value == tag.Value
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
