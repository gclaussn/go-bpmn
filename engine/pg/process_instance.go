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

type processInstanceRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r processInstanceRepository) Insert(entity *internal.ProcessInstanceEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO process_instance (
	partition,
	id,

	parent_id,
	root_id,

	process_id,

	bpmn_process_id,
	correlation_key,
	created_at,
	created_by,
	started_at,
	state,
	state_changed_by,
	tags,
	version
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
		partitionSequence("process_instance", entity.Partition),

		entity.ParentId,
		entity.RootId,

		entity.ProcessId,

		entity.BpmnProcessId,
		entity.CorrelationKey,
		entity.CreatedAt,
		entity.CreatedBy,
		entity.StartedAt,
		entity.State.String(),
		entity.StateChangedBy,
		entity.Tags,
		entity.Version,
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert process instance %+v: %v", entity, err)
	}

	return nil
}

func (r processInstanceRepository) Select(partition time.Time, id int32) (*internal.ProcessInstanceEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	parent_id,
	root_id,

	process_id,

	bpmn_process_id,
	correlation_key,
	created_at,
	created_by,
	ended_at,
	started_at,
	state,
	state_changed_by,
	tags,
	version
FROM
	process_instance
WHERE
	partition = $1 AND
	id = $2
FOR UPDATE
`, partition, id)

	var stateValue string

	var entity internal.ProcessInstanceEntity
	if err := row.Scan(
		&entity.ParentId,
		&entity.RootId,

		&entity.ProcessId,

		&entity.BpmnProcessId,
		&entity.CorrelationKey,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.EndedAt,
		&entity.StartedAt,
		&stateValue,
		&entity.StateChangedBy,
		&entity.Tags,
		&entity.Version,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to select process instance %s/%d: %v", partition.Format(time.DateOnly), id, err)
		}
	}

	entity.Partition = partition
	entity.Id = id
	entity.State = engine.MapInstanceState(stateValue)

	return &entity, nil
}

func (r processInstanceRepository) SelectByElementInstance(partition time.Time, elementInstanceId int32) (*internal.ProcessInstanceEntity, error) {
	row := r.tx.QueryRow(r.txCtx, "SELECT process_instance_id FROM element_instance WHERE partition = $1 AND id = $2", partition, elementInstanceId)

	var id int32
	if err := row.Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf(
				"failed to select process instance ID by element instance %s/%d: %v",
				partition.Format(time.DateOnly),
				elementInstanceId,
				err,
			)
		}
	}

	processInstance, err := r.Select(partition, id)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to select process instance %s/%d", partition.Format(time.DateOnly), id)
	}

	return processInstance, err
}

func (r processInstanceRepository) SelectByJob(partition time.Time, jobId int32) (*internal.ProcessInstanceEntity, error) {
	row := r.tx.QueryRow(r.txCtx, "SELECT process_instance_id FROM job WHERE partition = $1 AND id = $2", partition, jobId)

	var id int32
	if err := row.Scan(&id); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf(
				"failed to select process instance ID by job %s/%d: %v",
				partition.Format(time.DateOnly),
				jobId,
				err,
			)
		}
	}

	processInstance, err := r.Select(partition, id)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to select process instance %s/%d", partition.Format(time.DateOnly), id)
	}

	return processInstance, err
}

func (r processInstanceRepository) Update(entity *internal.ProcessInstanceEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	process_instance
SET
	ended_at = $3,
	started_at = $4,
	state = $5,
	state_changed_by = $6
WHERE
	partition = $1 AND
	id = $2
`,
		entity.Partition,
		entity.Id,

		entity.EndedAt,
		entity.StartedAt,
		entity.State.String(),
		entity.StateChangedBy,
	); err != nil {
		return fmt.Errorf("failed to update process instance %+v: %v", entity, err)
	}

	return nil
}

func (r processInstanceRepository) Query(criteria engine.ProcessInstanceCriteria, options engine.QueryOptions) ([]any, error) {
	var sql bytes.Buffer
	if err := sqlProcessInstanceQuery.Execute(&sql, map[string]any{
		"c": criteria,
		"o": options,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute process instance query template: %v", err)
	}

	sqls := sql.String()

	rows, err := r.tx.Query(r.txCtx, sqls)
	if err != nil {
		return nil, fmt.Errorf("failed to execute process instance query: %v", err)
	}

	defer rows.Close()

	var results []any
	for rows.Next() {
		var entity internal.ProcessInstanceEntity

		var stateValue string

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ParentId,
			&entity.RootId,

			&entity.ProcessId,

			&entity.BpmnProcessId,
			&entity.CorrelationKey,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.EndedAt,
			&entity.StartedAt,
			&stateValue,
			&entity.StateChangedBy,
			&entity.Tags,
			&entity.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan process instance row: %v", err)
		}

		entity.State = engine.MapInstanceState(stateValue)

		results = append(results, entity.ProcessInstance())
	}

	return results, nil
}
