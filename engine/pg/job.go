package pg

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type jobRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r jobRepository) Insert(entity *internal.JobEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO job (
	partition,
	id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	correlation_key,
	created_at,
	created_by,
	due_at,
	retry_count,
	retry_timer,
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
		entity.CorrelationKey,
		entity.CreatedAt,
		entity.CreatedBy,
		entity.DueAt,
		entity.RetryCount,
		entity.RetryTimer,
		entity.Type.String(),
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert job %+v: %v", entity, err)
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
	bpmn_error_code,
	bpmn_escalation_code,
	completed_at,
	correlation_key,
	created_at,
	created_by,
	due_at,
	error,
	locked_at,
	locked_by,
	retry_count,
	retry_timer,
	type
FROM
	job
WHERE
	partition = $1 AND
	id = $2
`, partition, id)

	var (
		entity    internal.JobEntity
		typeValue string
	)
	if err := row.Scan(
		&entity.ElementId,
		&entity.ElementInstanceId,
		&entity.ProcessId,
		&entity.ProcessInstanceId,

		&entity.BpmnElementId,
		&entity.BpmnErrorCode,
		&entity.BpmnEscalationCode,
		&entity.CompletedAt,
		&entity.CorrelationKey,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.DueAt,
		&entity.Error,
		&entity.LockedAt,
		&entity.LockedBy,
		&entity.RetryCount,
		&entity.RetryTimer,
		&typeValue,
	); err != nil {
		return nil, fmt.Errorf("failed to select job %s/%d: %v", partition.Format(time.DateOnly), id, err)
	}

	entity.Partition = partition
	entity.Id = id
	entity.Type = engine.MapJobType(typeValue)

	return &entity, nil
}

func (r jobRepository) Update(entity *internal.JobEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	job
SET
	bpmn_error_code = $3,
	bpmn_escalation_code = $4,
	completed_at = $5,
	error = $6
WHERE
	partition = $1 AND
	id = $2
`,
		entity.Partition,
		entity.Id,

		entity.BpmnErrorCode,
		entity.BpmnEscalationCode,
		entity.CompletedAt,
		entity.Error,
	); err != nil {
		return fmt.Errorf("failed to update job %+v: %v", entity, err)
	}

	return nil
}

func (r jobRepository) Query(criteria engine.JobCriteria, options engine.QueryOptions) ([]any, error) {
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

	var results []any
	for rows.Next() {
		var (
			entity    internal.JobEntity
			typeValue string
		)

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.BpmnElementId,
			&entity.BpmnErrorCode,
			&entity.BpmnEscalationCode,
			&entity.CompletedAt,
			&entity.CorrelationKey,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.DueAt,
			&entity.Error,
			&entity.LockedAt,
			&entity.LockedBy,
			&entity.RetryCount,
			&entity.RetryTimer,
			&typeValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job row: %v", err)
		}

		entity.Type = engine.MapJobType(typeValue)

		results = append(results, entity.Job())
	}

	return results, nil
}

func (r jobRepository) Lock(cmd engine.LockJobsCmd, lockedAt time.Time) ([]*internal.JobEntity, error) {
	var sql bytes.Buffer
	if err := sqlJobLock.Execute(&sql, cmd); err != nil {
		return nil, fmt.Errorf("failed to execute job lock template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String(), lockedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to lock jobs: %v", err)
	}

	defer rows.Close()

	var entities []*internal.JobEntity
	for rows.Next() {
		var (
			entity    internal.JobEntity
			typeValue string
		)

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.BpmnElementId,
			&entity.CorrelationKey,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.DueAt,
			&entity.LockedAt,
			&entity.LockedBy,
			&entity.RetryCount,
			&entity.RetryTimer,
			&typeValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan job lock row: %v", err)
		}

		entity.Type = engine.MapJobType(typeValue)

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r jobRepository) Unlock(cmd engine.UnlockJobsCmd) (int, error) {
	var sql bytes.Buffer
	if err := sqlJobUnlock.Execute(&sql, cmd); err != nil {
		return -1, fmt.Errorf("failed to execute job unlock template: %v", err)
	}

	commandTag, err := r.tx.Exec(r.txCtx, sql.String())
	if err != nil {
		return -1, fmt.Errorf("failed to unlock jobs: %v", err)
	}

	return int(commandTag.RowsAffected()), nil
}
