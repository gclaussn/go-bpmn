package mem

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type userTaskRepository struct {
	partitions map[string][]internal.UserTaskEntity
}

func (r *userTaskRepository) Insert(entity *internal.UserTaskEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]

	entity.Id = int32(len(entities) + 1)

	r.partitions[key] = append(entities, *entity)
	return nil
}

func (r *userTaskRepository) Select(partition time.Time, id int32) (*internal.UserTaskEntity, error) {
	key := partition.Format(time.DateOnly)
	for _, e := range r.partitions[key] {
		if e.Id == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("failed to select user task %s/%d: %v", key, id, pgx.ErrNoRows)
}

func (r *userTaskRepository) Update(entity *internal.UserTaskEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]
	for i, e := range entities {
		if e.Id == entity.Id {
			entities[i] = *entity
			return nil
		}
	}
	return fmt.Errorf("failed to update user task %s/%d: %v", key, entity.Id, pgx.ErrNoRows)
}

func (r *userTaskRepository) Query(c engine.UserTaskCriteria, o engine.QueryOptions) ([]engine.UserTask, error) {
	var (
		offset int
		limit  int

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

	results := make([]engine.UserTask, 0)
	for i := 0; i < len(keys); i++ {
		if partitionKey != "" && partitionKey != keys[i] {
			continue
		}

		for _, e := range r.partitions[keys[i]] {
			if c.Id != 0 && c.Id != e.Id {
				continue
			}

			if c.ElementInstanceId != 0 && c.ElementInstanceId != e.ElementInstanceId {
				continue
			}
			if c.ProcessId != 0 && c.ProcessId != e.ProcessId {
				continue
			}
			if c.ProcessInstanceId != 0 && c.ProcessInstanceId != e.ProcessInstanceId {
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
					matches = matches && ok && (tag.Value == "" || value == tag.Value)
				}

				if !matches {
					continue
				}
			}

			if offset < o.Offset {
				offset++
				continue
			}

			results = append(results, e.UserTask())
			limit++

			if o.Limit > 0 && limit == o.Limit {
				i = len(keys) // break outer loop
				break
			}
		}
	}

	return results, nil
}
