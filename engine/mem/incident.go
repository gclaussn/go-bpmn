package mem

import (
	"fmt"
	"sort"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type incidentRepository struct {
	partitions map[string][]internal.IncidentEntity
}

func (r *incidentRepository) Insert(entity *internal.IncidentEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]

	entity.Id = int32(len(entities) + 1)

	r.partitions[key] = append(entities, *entity)
	return nil
}

func (r *incidentRepository) Select(partition time.Time, id int32) (*internal.IncidentEntity, error) {
	key := partition.Format(time.DateOnly)
	for _, e := range r.partitions[key] {
		if e.Id == id {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *incidentRepository) Update(entity *internal.IncidentEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]
	for i, e := range entities {
		if e.Id == entity.Id {
			entities[i] = *entity
			return nil
		}
	}
	return fmt.Errorf("failed to update incident %s/%d: %v", key, entity.Id, pgx.ErrNoRows)
}

func (r *incidentRepository) Query(c engine.IncidentCriteria, o engine.QueryOptions) ([]any, error) {
	var (
		results []any
		offset  int
		limit   int

		partitionKey string
	)

	if !c.Partition.IsZero() {
		partitionKey = c.Partition.String()
	}

	keys := make([]string, 0, len(r.partitions))
	for key := range r.partitions {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for i := 0; i < len(keys); i++ {
		if partitionKey != "" && partitionKey != keys[i] {
			continue
		}

		for _, e := range r.partitions[keys[i]] {
			if c.Id != 0 && c.Id != e.Id {
				continue
			}

			if c.JobId != 0 && c.JobId != e.JobId.Int32 {
				continue
			}
			if c.ProcessInstanceId != 0 && c.ProcessInstanceId != e.ProcessInstanceId.Int32 {
				continue
			}
			if c.TaskId != 0 && c.TaskId != e.TaskId.Int32 {
				continue
			}

			if offset < o.Offset {
				offset++
				continue
			}

			results = append(results, e.Incident())
			limit++

			if o.Limit > 0 && limit == o.Limit {
				i = len(keys) // break outer loop
				break
			}
		}
	}

	return results, nil
}
