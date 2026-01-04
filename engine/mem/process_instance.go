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

type processInstanceRepository struct {
	partitions map[string][]internal.ProcessInstanceEntity

	elementInstances elementInstanceRepository
	jobs             jobRepository
}

func (r *processInstanceRepository) Insert(entity *internal.ProcessInstanceEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]

	entity.Id = int32(len(entities) + 1)

	r.partitions[key] = append(entities, *entity)
	return nil
}

func (r *processInstanceRepository) Select(partition time.Time, id int32) (*internal.ProcessInstanceEntity, error) {
	key := partition.Format(time.DateOnly)
	for _, e := range r.partitions[key] {
		if e.Id == id {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *processInstanceRepository) SelectByElementInstance(partition time.Time, elementInstanceId int32) (*internal.ProcessInstanceEntity, error) {
	key := partition.Format(time.DateOnly)

	var elementInstance *internal.ElementInstanceEntity
	for _, e := range r.elementInstances.partitions[key] {
		if e.Id == elementInstanceId {
			elementInstance = &e
		}
	}

	if elementInstance == nil {
		return nil, pgx.ErrNoRows
	}

	processInstance, err := r.Select(partition, elementInstance.ProcessInstanceId)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to select process instance %s/%d", key, elementInstance.ProcessInstanceId)
	}

	return processInstance, nil
}

func (r *processInstanceRepository) SelectByJob(partition time.Time, jobId int32) (*internal.ProcessInstanceEntity, error) {
	key := partition.Format(time.DateOnly)

	var job *internal.JobEntity
	for _, e := range r.jobs.partitions[key] {
		if e.Id == jobId {
			job = &e
		}
	}

	if job == nil {
		return nil, pgx.ErrNoRows
	}

	processInstance, err := r.Select(partition, job.ProcessInstanceId)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to select process instance %s/%d", key, job.ProcessInstanceId)
	}

	return processInstance, nil
}

func (r *processInstanceRepository) Update(entity *internal.ProcessInstanceEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]
	for i, e := range entities {
		if e.Id == entity.Id {
			entities[i] = *entity
			return nil
		}
	}
	return fmt.Errorf("failed to update process instance %s/%d: %v", key, entity.Id, pgx.ErrNoRows)
}

func (r *processInstanceRepository) Query(c engine.ProcessInstanceCriteria, o engine.QueryOptions) ([]engine.ProcessInstance, error) {
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

	results := make([]engine.ProcessInstance, 0)
	for i := 0; i < len(keys); i++ {
		if partitionKey != "" && partitionKey != keys[i] {
			continue
		}

		for _, e := range r.partitions[keys[i]] {
			if c.Id != 0 && c.Id != e.Id {
				continue
			}

			if c.ProcessId != 0 && c.ProcessId != e.ProcessId {
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

			results = append(results, e.ProcessInstance())
			limit++

			if o.Limit > 0 && limit == o.Limit {
				i = len(keys) // break outer loop
				break
			}
		}
	}

	return results, nil
}
