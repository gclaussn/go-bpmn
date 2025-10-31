package pg

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
)

type elementInstanceRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r elementInstanceRepository) Insert(entity *internal.ElementInstanceEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO element_instance (
	partition,
	id,

	parent_id,
	prev_element_id,
	prev_id,

	element_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	created_at,
	created_by,
	ended_at,
	execution_count,
	is_multi_instance,
	started_at,
	state
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
	$15,
	$16,
	$17
) RETURNING id
`,
		entity.Partition,
		partitionSequence("element_instance", entity.Partition),

		entity.ParentId,
		entity.PrevElementId,
		entity.PrevId,

		entity.ElementId,
		entity.ProcessId,
		entity.ProcessInstanceId,

		entity.BpmnElementId,
		entity.BpmnElementType.String(),
		entity.CreatedAt,
		entity.CreatedBy,
		entity.EndedAt,
		entity.ExecutionCount,
		entity.IsMultiInstance,
		entity.StartedAt,
		entity.State.String(),
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert element instance %+v: %v", entity, err)
	}

	return nil
}

func (r elementInstanceRepository) Select(partition time.Time, id int32) (*internal.ElementInstanceEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	parent_id,
	prev_element_id,
	prev_id,

	element_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	created_at,
	created_by,
	ended_at,
	execution_count,
	is_multi_instance,
	started_at,
	state
FROM
	element_instance
WHERE
	partition = $1 AND
	id = $2
`, partition, id)

	var bpmnElementTypeValue string
	var stateValue string

	var entity internal.ElementInstanceEntity
	if err := row.Scan(
		&entity.ParentId,
		&entity.PrevElementId,
		&entity.PrevId,

		&entity.ElementId,
		&entity.ProcessId,
		&entity.ProcessInstanceId,

		&entity.BpmnElementId,
		&bpmnElementTypeValue,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.EndedAt,
		&entity.ExecutionCount,
		&entity.IsMultiInstance,
		&entity.StartedAt,
		&stateValue,
	); err != nil {
		return nil, fmt.Errorf("failed to select element instance %s/%d: %v", partition.Format(time.DateOnly), id, err)
	}

	entity.Partition = partition
	entity.Id = id
	entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
	entity.State = engine.MapInstanceState(stateValue)

	return &entity, nil
}

func (r elementInstanceRepository) SelectByProcessInstanceAndState(processInstance *internal.ProcessInstanceEntity) ([]*internal.ElementInstanceEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	id,

	parent_id,
	prev_element_id,
	prev_id,

	element_id,
	process_id,

	bpmn_element_id,
	bpmn_element_type,
	created_at,
	created_by,
	ended_at,
	execution_count,
	is_multi_instance,
	started_at
FROM
	element_instance
WHERE
	partition = $1 AND
	process_instance_id = $2 AND
	state = $3
`,
		processInstance.Partition,
		processInstance.Id,
		processInstance.State.String(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to select element instances by process instance %s/%d and state %s: %v",
			processInstance.Partition.Format(time.DateOnly),
			processInstance.Id,
			processInstance.State,
			err,
		)
	}

	defer rows.Close()

	var entities []*internal.ElementInstanceEntity
	for rows.Next() {
		var entity internal.ElementInstanceEntity

		var bpmnElementTypeValue string

		if err := rows.Scan(
			&entity.Id,

			&entity.ParentId,
			&entity.PrevElementId,
			&entity.PrevId,

			&entity.ElementId,
			&entity.ProcessId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.EndedAt,
			&entity.ExecutionCount,
			&entity.IsMultiInstance,
			&entity.StartedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan element instance row: %v", err)
		}

		entity.Partition = processInstance.Partition
		entity.ProcessInstanceId = processInstance.Id
		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.State = processInstance.State

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r elementInstanceRepository) SelectParallelGateways(execution *internal.ElementInstanceEntity) ([]*internal.ElementInstanceEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	id,

	prev_element_id,
	prev_id,

	process_id,

	bpmn_element_id,
	bpmn_element_type,
	created_at,
	created_by,
	ended_at,
	execution_count,
	is_multi_instance,
	started_at
FROM
	element_instance
WHERE
	partition = $1 AND
	parent_id = $2 AND
	element_id = $3 AND
	state = $4
`,
		execution.Partition,
		execution.ParentId.Int32,
		execution.ElementId,
		execution.State.String(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to select parallel gateway element instances: %v", err)
	}

	defer rows.Close()

	var entities []*internal.ElementInstanceEntity
	for rows.Next() {
		var entity internal.ElementInstanceEntity

		var bpmnElementTypeValue string

		if err := rows.Scan(
			&entity.Id,

			&entity.PrevElementId,
			&entity.PrevId,

			&entity.ProcessId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.EndedAt,
			&entity.ExecutionCount,
			&entity.IsMultiInstance,
			&entity.StartedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan element instance row: %v", err)
		}

		entity.Partition = execution.Partition
		entity.ParentId = execution.ParentId
		entity.ElementId = execution.ElementId
		entity.ProcessInstanceId = execution.ProcessInstanceId
		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.State = execution.State

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r elementInstanceRepository) Update(entity *internal.ElementInstanceEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	element_instance
SET
	ended_at = $3,
	execution_count = $4,
	started_at = $5,
	state = $6,
WHERE
	partition = $1 AND
	id = $2
`,
		entity.Partition,
		entity.Id,

		entity.EndedAt,
		entity.ExecutionCount,
		entity.StartedAt,
		entity.State.String(),
	); err != nil {
		return fmt.Errorf("failed to update element instance %+v: %v", entity, err)
	}

	return nil
}

func (r elementInstanceRepository) Query(criteria engine.ElementInstanceCriteria, options engine.QueryOptions) ([]engine.ElementInstance, error) {
	var sql bytes.Buffer
	if err := sqlElementInstanceQuery.Execute(&sql, map[string]any{
		"c": criteria,
		"o": options,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute element instance query template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return nil, fmt.Errorf("failed to execute element instance query: %v", err)
	}

	defer rows.Close()

	results := make([]engine.ElementInstance, 0)
	for rows.Next() {
		var entity internal.ElementInstanceEntity

		var bpmnElementTypeValue string
		var stateValue string

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ParentId,

			&entity.ElementId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.EndedAt,
			&entity.IsMultiInstance,
			&entity.StartedAt,
			&stateValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan element instance row: %v", err)
		}

		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.State = engine.MapInstanceState(stateValue)

		results = append(results, entity.ElementInstance())
	}

	return results, nil
}
