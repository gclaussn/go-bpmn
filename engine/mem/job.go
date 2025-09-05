package mem

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type jobRepository struct {
	partitions map[string][]internal.JobEntity
}

func (r *jobRepository) Insert(entity *internal.JobEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]

	entity.Id = int32(len(entities) + 1)

	r.partitions[key] = append(entities, *entity)
	return nil
}

func (r *jobRepository) Select(partition time.Time, id int32) (*internal.JobEntity, error) {
	key := partition.Format(time.DateOnly)
	for _, e := range r.partitions[key] {
		if e.Id == id {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("failed to select job %s/%d: %v", key, id, pgx.ErrNoRows)
}

func (r *jobRepository) Update(entity *internal.JobEntity) error {
	key := entity.Partition.Format(time.DateOnly)
	entities := r.partitions[key]
	for i, e := range entities {
		if e.Id == entity.Id {
			entities[i] = *entity
			return nil
		}
	}
	return fmt.Errorf("failed to update job %s/%d: %v", key, entity.Id, pgx.ErrNoRows)
}

func (r *jobRepository) Query(c engine.JobCriteria, o engine.QueryOptions) ([]engine.Job, error) {
	var (
		results []engine.Job
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

			if c.ElementInstanceId != 0 && c.ElementInstanceId != e.ElementInstanceId {
				continue
			}
			if c.ProcessId != 0 && c.ProcessId != e.ProcessId {
				continue
			}
			if c.ProcessInstanceId != 0 && c.ProcessInstanceId != e.ProcessInstanceId {
				continue
			}

			if offset < o.Offset {
				offset++
				continue
			}

			results = append(results, e.Job())
			limit++

			if o.Limit > 0 && limit == o.Limit {
				i = len(keys) // break outer loop
				break
			}
		}
	}

	return results, nil
}

func (r *jobRepository) Lock(cmd engine.LockJobsCmd, lockedAt time.Time) ([]*internal.JobEntity, error) {
	var results []*internal.JobEntity

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

			if len(cmd.BpmnElementIds) != 0 && !slices.Contains(cmd.BpmnElementIds, e.BpmnElementId) {
				continue
			}
			if cmd.ElementInstanceId != 0 && cmd.ElementInstanceId != e.ElementInstanceId {
				continue
			}
			if len(cmd.ProcessIds) != 0 && !slices.Contains(cmd.ProcessIds, e.ProcessId) {
				continue
			}
			if cmd.ProcessInstanceId != 0 && cmd.ProcessInstanceId != e.ProcessInstanceId {
				continue
			}

			e.LockedAt = pgtype.Timestamp{Time: lockedAt, Valid: true}
			e.LockedBy = pgtype.Text{String: cmd.WorkerId, Valid: true}
			entities[j] = e

			results = append(results, &e)
		}
	}

	return results, nil
}

func (r *jobRepository) Unlock(cmd engine.UnlockJobsCmd) (int, error) {
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

			if cmd.WorkerId != e.LockedBy.String {
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
