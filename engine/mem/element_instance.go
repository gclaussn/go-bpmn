package mem

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type elementInstanceRepository struct {
	partitions map[string][]internal.ElementInstanceEntity
}

func (r *elementInstanceRepository) Insert(entity *internal.ElementInstanceEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]

	entity.Id = int32(len(entities) + 1)

	r.partitions[key] = append(entities, *entity)
	return nil
}

func (r *elementInstanceRepository) Select(partition time.Time, id int32) (*internal.ElementInstanceEntity, error) {
	key := partition.Format(time.DateOnly)
	for _, e := range r.partitions[key] {
		if e.Id == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("failed to select element instance %s/%d: %v", key, id, pgx.ErrNoRows)
}

func (r *elementInstanceRepository) SelectByProcessInstanceAndState(processInstance *internal.ProcessInstanceEntity) ([]*internal.ElementInstanceEntity, error) {
	var results []*internal.ElementInstanceEntity

	key := processInstance.Partition.Format(time.DateOnly)
	for _, e := range r.partitions[key] {
		if e.ProcessInstanceId == processInstance.Id && e.State == processInstance.State {
			results = append(results, &e)
		}
	}

	return results, nil
}

func (r *elementInstanceRepository) SelectBoundaryEvents(execution *internal.ElementInstanceEntity) ([]*internal.ElementInstanceEntity, error) {
	var results []*internal.ElementInstanceEntity

	key := execution.Partition.Format(time.DateOnly)
	for _, e := range r.partitions[key] {
		if e.PrevId.Int32 == execution.Id {
			results = append(results, &e)
		}
	}

	return results, nil
}

func (r *elementInstanceRepository) SelectParallelGateways(execution *internal.ElementInstanceEntity) ([]*internal.ElementInstanceEntity, error) {
	var results []*internal.ElementInstanceEntity

	key := execution.Partition.Format(time.DateOnly)
	for _, e := range r.partitions[key] {
		if e.ProcessInstanceId != execution.ProcessInstanceId {
			continue
		}
		if e.State != execution.State {
			continue
		}
		if e.ElementId != execution.ElementId {
			continue
		}
		if e.ParentId.Int32 != execution.ParentId.Int32 {
			continue
		}
		results = append(results, &e)
	}

	return results, nil
}

func (r *elementInstanceRepository) Update(entity *internal.ElementInstanceEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]
	for i, e := range entities {
		if e.Id == entity.Id {
			entities[i] = *entity
			return nil
		}
	}
	return fmt.Errorf("failed to update element instance %s/%d: %v", key, entity.Id, pgx.ErrNoRows)
}

func (r *elementInstanceRepository) Query(c engine.ElementInstanceCriteria, o engine.QueryOptions) ([]engine.ElementInstance, error) {
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

	results := make([]engine.ElementInstance, 0)
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
			if c.ProcessInstanceId != 0 && c.ProcessInstanceId != e.ProcessInstanceId {
				continue
			}

			if c.BpmnElementId != "" && c.BpmnElementId != e.BpmnElementId {
				continue
			}
			if len(c.States) != 0 && !slices.Contains(c.States, e.State) {
				continue
			}

			if offset < o.Offset {
				offset++
				continue
			}

			results = append(results, e.ElementInstance())
			limit++

			if o.Limit > 0 && limit == o.Limit {
				i = len(keys) // break outer loop
				break
			}
		}
	}

	return results, nil
}
