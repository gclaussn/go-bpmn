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

type userTaskRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r userTaskRepository) Insert(entity *internal.UserTaskEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO user_task (
	partition,
	id,

	revision,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	correlation_key,
	created_at,
	created_by,
	state,
	tags,
	updated_at,
	updated_by
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
	$14,
	$15
) RETURNING id
`,
		entity.Partition,
		partitionSequence("user_task", entity.Partition),

		entity.Revision,

		entity.ElementId,
		entity.ElementInstanceId,
		entity.ProcessId,
		entity.ProcessInstanceId,

		entity.BpmnElementId,
		entity.CorrelationKey,
		entity.CreatedAt,
		entity.CreatedBy,
		entity.State.String(),
		entity.Tags,
		entity.UpdatedAt,
		entity.UpdatedBy,
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert user task %+v: %v", entity, err)
	}

	return nil
}

func (r userTaskRepository) Select(partition time.Time, id int32) (*internal.UserTaskEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	revision,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	correlation_key,
	created_at,
	created_by,
	state,
	tags,
	updated_at,
	updated_by
FROM
	user_task
WHERE
	partition = $1 AND
	id = $2
`, partition, id)

	var stateValue string

	var entity internal.UserTaskEntity
	if err := row.Scan(
		&entity.Revision,

		&entity.ElementId,
		&entity.ElementInstanceId,
		&entity.ProcessId,
		&entity.ProcessInstanceId,

		&entity.BpmnElementId,
		&entity.CorrelationKey,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&stateValue,
		&entity.Tags,
		&entity.UpdatedAt,
		&entity.UpdatedBy,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to select user task %s/%d: %v", partition.Format(time.DateOnly), id, err)
		}
	}

	entity.Partition = partition
	entity.Id = id
	entity.State = engine.MapUserTaskState(stateValue)

	return &entity, nil
}

func (r userTaskRepository) Update(entity *internal.UserTaskEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	user_task
SET
	revision = $3,

	state = $4,
	tags = $5,
	updated_at = $6,
	updated_by = $7
WHERE
	partition = $1 AND
	id = $2
`,
		entity.Partition,
		entity.Id,

		entity.Revision,

		entity.State.String(),
		entity.Tags,
		entity.UpdatedAt,
		entity.UpdatedBy,
	); err != nil {
		return fmt.Errorf("failed to update user task %+v: %v", entity, err)
	}

	return nil
}

func (r userTaskRepository) Query(criteria engine.UserTaskCriteria, options engine.QueryOptions) ([]engine.UserTask, error) {
	var sql bytes.Buffer
	if err := sqlUserTaskQuery.Execute(&sql, map[string]any{
		"c": criteria,
		"o": options,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute user task query template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return nil, fmt.Errorf("failed to execute user task query: %v", err)
	}

	defer rows.Close()

	results := make([]engine.UserTask, 0)
	for rows.Next() {
		var stateValue string

		var entity internal.UserTaskEntity
		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.Revision,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.BpmnElementId,
			&entity.CorrelationKey,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&stateValue,
			&entity.Tags,
			&entity.UpdatedAt,
			&entity.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user task row: %v", err)
		}

		entity.State = engine.MapUserTaskState(stateValue)

		results = append(results, entity.UserTask())
	}

	return results, nil
}
