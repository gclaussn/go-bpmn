package mem

import (
	"slices"
	"sort"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
)

type variableRepository struct {
	partitions map[string][]internal.VariableEntity
}

func (r *variableRepository) Delete(entity *internal.VariableEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]
	for i, e := range entities {
		if e.Id == -1 {
			continue // skip deleted variable
		}

		if entity.ProcessInstanceId != 0 && e.ProcessInstanceId != entity.ProcessInstanceId {
			continue
		}
		if e.ElementInstanceId.Int32 != entity.ElementInstanceId.Int32 {
			continue
		}

		if e.Name == entity.Name {
			e.Id = -1
			e.Value = ""
			entities[i] = e
			break
		}
	}
	return nil
}

func (r *variableRepository) Insert(entity *internal.VariableEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]

	entity.Id = int32(len(entities) + 1)

	r.partitions[key] = append(entities, *entity)
	return nil
}

func (r *variableRepository) SelectByElementInstance(cmd engine.GetElementVariablesCmd) ([]*internal.VariableEntity, error) {
	var results []*internal.VariableEntity

	key := cmd.Partition.String()
	for _, e := range r.partitions[key] {
		if e.Id == -1 {
			continue // skip deleted variable
		}

		if cmd.ElementInstanceId != 0 && cmd.ElementInstanceId != e.ElementInstanceId.Int32 {
			continue
		}

		if len(cmd.Names) != 0 && !slices.Contains(cmd.Names, e.Name) {
			continue
		}

		results = append(results, &e)
	}

	return results, nil
}

func (r *variableRepository) SelectByProcessInstance(cmd engine.GetProcessVariablesCmd) ([]*internal.VariableEntity, error) {
	var results []*internal.VariableEntity

	key := cmd.Partition.String()
	for _, e := range r.partitions[key] {
		if e.Id == -1 {
			continue // skip deleted variable
		}

		if e.ElementInstanceId.Valid {
			continue // skip element variable
		}

		if cmd.ProcessInstanceId != 0 && cmd.ProcessInstanceId != e.ProcessInstanceId {
			continue
		}

		if len(cmd.Names) != 0 && !slices.Contains(cmd.Names, e.Name) {
			continue
		}

		results = append(results, &e)
	}

	return results, nil
}

func (r *variableRepository) Upsert(entity *internal.VariableEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]
	for i, e := range entities {
		if e.Id == -1 { // skip deleted variable
			continue
		}

		if e.ProcessInstanceId == entity.ProcessInstanceId && e.ElementInstanceId.Int32 == entity.ElementInstanceId.Int32 && e.Name == entity.Name {
			entity.CreatedAt = e.CreatedAt
			entity.CreatedBy = e.CreatedBy
			entities[i] = *entity
			return nil
		}
	}

	return r.Insert(entity)
}

func (r *variableRepository) Query(c engine.VariableCriteria, o engine.QueryOptions) ([]engine.Variable, error) {
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

	results := make([]engine.Variable, 0)
	for i := 0; i < len(keys); i++ {
		if partitionKey != "" && partitionKey != keys[i] {
			continue
		}

		for _, e := range r.partitions[keys[i]] {
			if e.Id == -1 {
				continue
			}

			if c.ElementInstanceId != 0 && c.ElementInstanceId != e.ElementInstanceId.Int32 {
				continue
			}
			if c.ProcessInstanceId != 0 && c.ProcessInstanceId != e.ProcessInstanceId {
				continue
			}

			if len(c.Names) != 0 && !slices.Contains(c.Names, e.Name) {
				continue
			}

			if offset < o.Offset {
				offset++
				continue
			}

			results = append(results, e.Variable())
			limit++

			if o.Limit > 0 && limit == o.Limit {
				i = len(keys) // break outer loop
				break
			}
		}
	}

	return results, nil
}
