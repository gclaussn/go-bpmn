package mem

import (
	"fmt"
	"sort"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type taskRepository struct {
	partitions map[string][]internal.TaskEntity

	engineId string
}

func (r *taskRepository) Insert(entity *internal.TaskEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]

	entity.Id = int32(len(entities) + 1)

	r.partitions[key] = append(entities, *entity)
	return nil
}

func (r *taskRepository) InsertBatch(entities []*internal.TaskEntity) error {
	for _, entity := range entities {
		_ = r.Insert(entity)
	}
	return nil
}

func (r *taskRepository) Select(partition time.Time, id int32) (*internal.TaskEntity, error) {
	key := partition.Format(time.DateOnly)
	for _, e := range r.partitions[key] {
		if e.Id == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("failed to select task %s/%d: %v", key, id, pgx.ErrNoRows)
}

func (r *taskRepository) Update(entity *internal.TaskEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]
	for i, e := range entities {
		if e.Id == entity.Id {
			entities[i] = *entity
			return nil
		}
	}
	return fmt.Errorf("failed to update task %s/%d: %v", key, entity.Id, pgx.ErrNoRows)
}

func (r *taskRepository) Query(c engine.TaskCriteria, o engine.QueryOptions) ([]engine.Task, error) {
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

	results := make([]engine.Task, 0)
	for i := 0; i < len(keys); i++ {
		if partitionKey != "" && partitionKey != keys[i] {
			continue
		}

		for _, e := range r.partitions[keys[i]] {
			if c.Id != 0 && c.Id != e.Id {
				continue
			}

			if c.ElementId != 0 && c.ElementId != e.ElementId.Int32 {
				continue
			}
			if c.ElementInstanceId != 0 && c.ElementInstanceId != e.ElementInstanceId.Int32 {
				continue
			}
			if c.ProcessId != 0 && c.ProcessId != e.ProcessId.Int32 {
				continue
			}
			if c.ProcessInstanceId != 0 && c.ProcessInstanceId != e.ProcessInstanceId.Int32 {
				continue
			}

			if c.Type != 0 && c.Type != e.Type {
				continue
			}

			if offset < o.Offset {
				offset++
				continue
			}

			results = append(results, e.Task())
			limit++

			if o.Limit > 0 && limit == o.Limit {
				i = len(keys) // break outer loop
				break
			}
		}
	}

	return results, nil
}

func (r *taskRepository) Lock(cmd engine.ExecuteTasksCmd, lockedAt time.Time) ([]*internal.TaskEntity, error) {
	if cmd.Limit <= 0 {
		cmd.Limit = 1
	}

	var results []*internal.TaskEntity

	var partitionKey string
	if !cmd.Partition.IsZero() {
		partitionKey = cmd.Partition.String()
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

		entities := r.partitions[keys[i]]
		for j, e := range entities {
			if cmd.Limit == len(results) {
				i = len(keys) // break outer loop
				break
			}

			if e.LockedAt.Valid || lockedAt.Before(e.DueAt) {
				continue
			}

			if cmd.Id != 0 && cmd.Id != e.Id {
				continue
			}

			if cmd.ProcessId != 0 && cmd.ProcessId != e.ProcessId.Int32 {
				continue
			}
			if cmd.ProcessInstanceId != 0 && cmd.ProcessInstanceId != e.ProcessInstanceId.Int32 {
				continue
			}

			if cmd.Type != 0 && cmd.Type != e.Type {
				continue
			}

			e.LockedAt = pgtype.Timestamp{Time: lockedAt, Valid: true}
			e.LockedBy = pgtype.Text{String: r.engineId, Valid: true}
			entities[j] = e

			results = append(results, &e)
		}
	}

	return results, nil
}

func (r *taskRepository) Unlock(cmd engine.UnlockTasksCmd) (int, error) {
	count := 0

	var partitionKey string
	if !cmd.Partition.IsZero() {
		partitionKey = cmd.Partition.String()
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

		entities := r.partitions[keys[i]]
		for j, e := range entities {
			if e.CompletedAt.Valid || !e.LockedBy.Valid {
				continue
			}

			if cmd.EngineId != e.LockedBy.String {
				continue
			}

			if cmd.Id != 0 && cmd.Id != e.Id {
				continue
			}

			e.LockedAt = pgtype.Timestamp{}
			e.LockedBy = pgtype.Text{}
			entities[j] = e
			count++
		}
	}

	return count, nil
}
