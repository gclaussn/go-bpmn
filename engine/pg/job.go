package pg

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type jobRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r jobRepository) Insert(entity *internal.JobEntity, correlationKey pgtype.Text) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO job (
	partition,
	id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	created_at,
	created_by,
	due_at,
	retry_count,
	state,
	type
) VALUES (
	$1,
	nextval($2),

	$3,
	$4,
	$5,
	$6,

	$7,
	$8,
	$9,
	$10,
	$11,
	$12,
	$13,
	$14
) RETURNING id
`,
		entity.Partition,
		partitionSequence("job", entity.Partition),

		entity.ElementId,
		entity.ElementInstanceId,
		entity.ProcessId,
		entity.ProcessInstanceId,

		entity.BpmnElementId,
		entity.BpmnElementType.String(),
		entity.CreatedAt,
		entity.CreatedBy,
		entity.DueAt,
		entity.RetryCount,
		entity.State.String(),
		entity.Type.String(),
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert job %+v: %v", entity, err)
	}

	poolRow := r.tx.QueryRow(r.txCtx, `
INSERT INTO job_pool (
	partition,

	element_id,
	element_instance_id,
	job_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	correlation_key,
	due_at,
	retry_count,
	type
) VALUES (
	$1,

	$2,
	$3,
	$4,
	$5,
	$6,

	$7,
	$8,
	$9,
	$10,
	$11,
	$12
) RETURNING id
`,
		entity.Partition,

		entity.ElementId,
		entity.ElementInstanceId,
		entity.Id,
		entity.ProcessId,
		entity.ProcessInstanceId,

		entity.BpmnElementId,
		entity.BpmnElementType.String(),
		correlationKey,
		entity.DueAt,
		entity.RetryCount,
		entity.Type.String(),
	)

	var poolId int64 // for debugging
	if err := poolRow.Scan(&poolId); err != nil {
		return fmt.Errorf("failed to insert job into pool %+v: %v", entity, err)
	}

	return nil
}

func (r jobRepository) Select(partition time.Time, id int32) (*internal.JobEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	completed_at,
	created_at,
	created_by,
	due_at,
	error,
	locked_at,
	locked_by,
	retry_count,
	state,
	type
FROM
	job
WHERE
	partition = $1 AND
	id = $2
`, partition, id)

	var (
		entity internal.JobEntity

		bpmnElementTypeValue string
		stateValue           string
		typeValue            string
	)
	if err := row.Scan(
		&entity.ElementId,
		&entity.ElementInstanceId,
		&entity.ProcessId,
		&entity.ProcessInstanceId,

		&entity.BpmnElementId,
		&bpmnElementTypeValue,
		&entity.CompletedAt,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.DueAt,
		&entity.Error,
		&entity.LockedAt,
		&entity.LockedBy,
		&entity.RetryCount,
		&stateValue,
		&typeValue,
	); err != nil {
		return nil, fmt.Errorf("failed to select job %s/%d: %v", partition.Format(time.DateOnly), id, err)
	}

	entity.Partition = partition
	entity.Id = id
	entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
	entity.State = engine.MapWorkState(stateValue)
	entity.Type = engine.MapJobType(typeValue)

	return &entity, nil
}

func (r jobRepository) Update(entity *internal.JobEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	job
SET
	completed_at = $3,
	error = $4,
	state = $5
WHERE
	partition = $1 AND
	id = $2
`,
		entity.Partition,
		entity.Id,

		entity.CompletedAt,
		entity.Error,
		entity.State.String(),
	); err != nil {
		return fmt.Errorf("failed to update job %+v: %v", entity, err)
	}

	return nil
}

func (r jobRepository) Query(criteria engine.JobCriteria, options engine.QueryOptions, now time.Time) ([]engine.Job, error) {
	var sql bytes.Buffer
	if err := sqlJobQuery.Execute(&sql, map[string]any{
		"c": criteria,
		"o": options,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute job query template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return nil, fmt.Errorf("failed to execute job query: %v", err)
	}

	defer rows.Close()

	results := make([]engine.Job, 0)
	for rows.Next() {
		var (
			entity internal.JobEntity

			bpmnElementTypeValue string
			stateValue           string
			typeValue            string
		)

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.CompletedAt,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.DueAt,
			&entity.Error,
			&entity.LockedAt,
			&entity.LockedBy,
			&entity.RetryCount,
			&stateValue,
			&typeValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job row: %v", err)
		}

		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.State = engine.MapWorkState(stateValue)
		entity.Type = engine.MapJobType(typeValue)

		if entity.State == engine.WorkCreated {
			if entity.LockedAt.Valid {
				entity.State = engine.WorkLocked
			} else if !now.Before(entity.DueAt) {
				entity.State = engine.WorkDue
			}
		}

		results = append(results, entity.Job())
	}

	return results, nil
}

func (r jobRepository) Lock(cmd engine.LockJobsCmd, lockedAt time.Time) ([]internal.JobPoolEntity, error) {
	var sql bytes.Buffer
	if err := sqlJobLock.Execute(&sql, cmd); err != nil {
		return nil, fmt.Errorf("failed to execute job lock template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String(), lockedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to lock jobs: %v", err)
	}

	defer rows.Close()

	var entities []internal.JobPoolEntity
	for rows.Next() {
		var (
			entity internal.JobPoolEntity

			bpmnElementTypeValue string
			typeValue            string
		)

		if err := rows.Scan(
			&entity.Id,

			&entity.Partition,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.JobId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.CorrelationKey,
			&entity.DueAt,
			&entity.RetryCount,
			&typeValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job lock row: %v", err)
		}

		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.LockedAt = pgtype.Timestamp{Time: lockedAt, Valid: true}
		entity.LockedBy = pgtype.Text{String: cmd.WorkerId, Valid: true}
		entity.Type = engine.MapJobType(typeValue)

		entities = append(entities, entity)
	}

	if len(entities) == 0 {
		return entities, nil
	}

	// delete from pool
	idsToDelete := make([]string, len(entities))
	for i, entity := range entities {
		idsToDelete[i] = strconv.FormatInt(entity.Id, 10)
	}

	if _, err := r.tx.Exec(r.txCtx, fmt.Sprintf("DELETE FROM job_pool WHERE id IN (%s)", strings.Join(idsToDelete, ","))); err != nil {
		return nil, fmt.Errorf("failed to delete locked jobs from pool: %v", err)
	}

	// insert into pool lock
	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO job_pool_lock (
	id,

	partition,

	element_id,
	element_instance_id,
	job_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	correlation_key,
	due_at,
	locked_at,
	locked_by,
	retry_count,
	type
) VALUES (
	$1,

	$2,

	$3,
	$4,
	$5,
	$6,
	$7,

	$8,
	$9,
	$10,
	$11,
	$12,
	$13,
	$14,
	$15
) RETURNING id
`,
			entity.Id,

			entity.Partition,

			entity.ElementId,
			entity.ElementInstanceId,
			entity.JobId,
			entity.ProcessId,
			entity.ProcessInstanceId,

			entity.BpmnElementId,
			entity.BpmnElementType.String(),
			entity.CorrelationKey,
			entity.DueAt,
			entity.LockedAt,
			entity.LockedBy,
			entity.RetryCount,
			entity.Type.String(),
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := range entities {
		if _, err := batchResults.Exec(); err != nil {
			return nil, fmt.Errorf("failed to insert job into pool lock %+v: %v", entities[i], err)
		}
	}

	return entities, nil
}

func (r jobRepository) Unlock(cmd engine.UnlockJobsCmd) (int, error) {
	// delete from pool lock
	var sql bytes.Buffer
	if err := sqlJobUnlock.Execute(&sql, cmd); err != nil {
		return -1, fmt.Errorf("failed to execute job unlock template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return -1, fmt.Errorf("failed to unlock jobs: %v", err)
	}

	defer rows.Close()

	var entities []internal.JobPoolEntity
	for rows.Next() {
		var (
			entity internal.JobPoolEntity

			bpmnElementTypeValue string
			typeValue            string
		)

		if err := rows.Scan(
			&entity.Id,

			&entity.Partition,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.JobId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.CorrelationKey,
			&entity.DueAt,
			&entity.RetryCount,
			&typeValue,
		); err != nil {
			return -1, fmt.Errorf("failed to scan job unlock row: %v", err)
		}

		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.Type = engine.MapJobType(typeValue)

		entities = append(entities, entity)
	}

	if len(entities) == 0 {
		return 0, nil
	}

	// insert into pool
	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO job_pool (
	partition,

	element_id,
	element_instance_id,
	job_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	correlation_key,
	due_at,
	retry_count,
	type
) VALUES (
	$1,

	$2,
	$3,
	$4,
	$5,
	$6,

	$7,
	$8,
	$9,
	$10,
	$11,
	$12
) RETURNING id
`,
			entity.Partition,

			entity.ElementId,
			entity.ElementInstanceId,
			entity.JobId,
			entity.ProcessId,
			entity.ProcessInstanceId,

			entity.BpmnElementId,
			entity.BpmnElementType.String(),
			entity.CorrelationKey,
			entity.DueAt,
			entity.RetryCount,
			entity.Type.String(),
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := range entities {
		row := batchResults.QueryRow()

		if err := row.Scan(&entities[i].Id); err != nil {
			return -1, fmt.Errorf("failed to insert job into pool %+v: %v", entities[i], err)
		}
	}

	return len(entities), nil
}
