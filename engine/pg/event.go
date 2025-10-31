package pg

import (
	"context"
	"fmt"

	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type eventRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r eventRepository) Insert(entity *internal.EventEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
INSERT INTO event (
	partition,

	element_instance_id,

	created_at,
	created_by,
	error_code,
	message_correlation_key,
	message_name,
	signal_name,
	time,
	time_cycle,
	time_duration
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
	$11
)
`,
		entity.Partition,

		entity.ElementInstanceId,

		entity.CreatedAt,
		entity.CreatedBy,
		entity.ErrorCode,
		entity.MessageCorrelationKey,
		entity.MessageName,
		entity.SignalName,
		entity.Time,
		entity.TimeCycle,
		entity.TimeDuration,
	); err != nil {
		return fmt.Errorf("failed to insert event %+v: %v", entity, err)
	}

	return nil
}

type eventDefinitionRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r eventDefinitionRepository) InsertBatch(entities []*internal.EventDefinitionEntity) error {
	if len(entities) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO event_definition (
	element_id,

	process_id,

	bpmn_element_id,
	bpmn_element_type,
	bpmn_process_id,
	error_code,
	is_suspended,
	message_name,
	signal_name,
	time,
	time_cycle,
	time_duration,
	version
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
	$13
)
`,
			entity.ElementId,

			entity.ProcessId,

			entity.BpmnElementId,
			entity.BpmnElementType.String(),
			entity.BpmnProcessId,
			entity.ErrorCode,
			entity.IsSuspended,
			entity.MessageName,
			entity.SignalName,
			entity.Time,
			entity.TimeCycle,
			entity.TimeDuration,
			entity.Version,
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := range entities {
		if _, err := batchResults.Exec(); err != nil {
			return fmt.Errorf("failed to insert event definition %+v: %v", entities[i], err)
		}
	}

	return nil
}

func (r eventDefinitionRepository) Select(elementId int32) (*internal.EventDefinitionEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	process_id,

	bpmn_element_id,
	bpmn_element_type,
	bpmn_process_id,
	error_code,
	is_suspended,
	time,
	time_cycle,
	time_duration,
	version
FROM
	event_definition
WHERE
	element_id = $1
`, elementId)

	var (
		entity               internal.EventDefinitionEntity
		bpmnElementTypeValue string
	)

	if err := row.Scan(
		&entity.ProcessId,

		&entity.BpmnElementId,
		&bpmnElementTypeValue,
		&entity.BpmnProcessId,
		&entity.ErrorCode,
		&entity.IsSuspended,
		&entity.Time,
		&entity.TimeCycle,
		&entity.TimeDuration,
		&entity.Version,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to select event definition %d: %v", elementId, err)
		}
	}

	entity.ElementId = elementId
	entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)

	return &entity, nil
}

func (r eventDefinitionRepository) SelectByBpmnProcessId(bpmnProcessId string) ([]*internal.EventDefinitionEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	element_id,

	process_id,

	bpmn_element_id,
	bpmn_element_type,
	is_suspended,
	message_name,
	signal_name,
	time,
	time_cycle,
	time_duration,
	version
FROM
	event_definition
WHERE
	bpmn_process_id = $1
`, bpmnProcessId)
	if err != nil {
		return nil, fmt.Errorf("failed to select event definitions by BPMN process ID %s: %v", bpmnProcessId, err)
	}

	defer rows.Close()

	var entities []*internal.EventDefinitionEntity
	for rows.Next() {
		var (
			entity               internal.EventDefinitionEntity
			bpmnElementTypeValue string
		)

		if err := rows.Scan(
			&entity.ElementId,

			&entity.ProcessId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.IsSuspended,
			&entity.MessageName,
			&entity.SignalName,
			&entity.Time,
			&entity.TimeCycle,
			&entity.TimeDuration,
			&entity.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event definition row: %v", err)
		}

		entity.BpmnProcessId = bpmnProcessId
		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r eventDefinitionRepository) SelectByProcessId(processId int32) ([]*internal.EventDefinitionEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	element_id,

	bpmn_element_id,
	bpmn_element_type,
	bpmn_process_id,
	is_suspended,
	message_name,
	signal_name,
	time,
	time_cycle,
	time_duration,
	version
FROM
	event_definition
WHERE
	process_id = $1
`, processId)
	if err != nil {
		return nil, fmt.Errorf("failed to select event definitions by process ID %d: %v", processId, err)
	}

	defer rows.Close()

	var entities []*internal.EventDefinitionEntity
	for rows.Next() {
		var (
			entity               internal.EventDefinitionEntity
			bpmnElementTypeValue string
		)

		if err := rows.Scan(
			&entity.ElementId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.BpmnProcessId,
			&entity.IsSuspended,
			&entity.MessageName,
			&entity.SignalName,
			&entity.Time,
			&entity.TimeCycle,
			&entity.TimeDuration,
			&entity.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event definition row: %v", err)
		}

		entity.ProcessId = processId
		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r eventDefinitionRepository) SelectByMessageName(messageName string) (*internal.EventDefinitionEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	element_id,

	process_id,

	bpmn_element_id,
	bpmn_element_type,
	bpmn_process_id,
	message_name,
	version
FROM
	event_definition
WHERE
	message_name = $1 AND
	is_suspended IS FALSE
ORDER BY
	element_id
LIMIT
	1
`, messageName)

	var (
		entity               internal.EventDefinitionEntity
		bpmnElementTypeValue string
	)

	if err := row.Scan(
		&entity.ElementId,

		&entity.ProcessId,

		&entity.BpmnElementId,
		&bpmnElementTypeValue,
		&entity.BpmnProcessId,
		&entity.MessageName,
		&entity.Version,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to select not suspended event definition by message name %s: %v", messageName, err)
		}
	}

	entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
	entity.MessageName = pgtype.Text{String: messageName, Valid: true}

	return &entity, nil
}

func (r eventDefinitionRepository) SelectBySignalName(signalName string) ([]*internal.EventDefinitionEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	element_id,

	process_id,

	bpmn_element_id,
	bpmn_element_type,
	bpmn_process_id,
	signal_name,
	version
FROM
	event_definition
WHERE
	signal_name = $1 AND
	is_suspended IS FALSE
`, signalName)
	if err != nil {
		return nil, fmt.Errorf("failed to select not suspended event definitions by signal name %s: %v", signalName, err)
	}

	defer rows.Close()

	var entities []*internal.EventDefinitionEntity
	for rows.Next() {
		var (
			entity               internal.EventDefinitionEntity
			bpmnElementTypeValue string
		)

		if err := rows.Scan(
			&entity.ElementId,

			&entity.ProcessId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.BpmnProcessId,
			&entity.SignalName,
			&entity.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event definition row: %v", err)
		}

		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.SignalName = pgtype.Text{String: signalName, Valid: true}

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r eventDefinitionRepository) UpdateBatch(entities []*internal.EventDefinitionEntity) error {
	if len(entities) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
UPDATE
	event_definition
SET
	is_suspended = $2
WHERE
	element_id = $1
`,
			entity.ElementId,

			entity.IsSuspended,
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := range entities {
		if _, err := batchResults.Exec(); err != nil {
			return fmt.Errorf("failed to update event definition %+v: %v", entities[i], err)
		}
	}

	return nil
}
