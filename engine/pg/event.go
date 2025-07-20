package pg

import (
	"context"
	"fmt"
	"time"

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
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO event (
	partition,
	id,

	created_at,
	created_by,
	signal_name,
	signal_subscriber_count,
	time,
	time_cycle,
	time_duration
) VALUES (
	$1,
	nextval($2),

	$3,
	$4,
	$5,
	$6,
	$7,
	$8,
	$9
) RETURNING id
`,
		entity.Partition,
		partitionSequence("event", entity.Partition),

		entity.CreatedAt,
		entity.CreatedBy,
		entity.SignalName,
		entity.SignalSubscriberCount,
		entity.Time,
		entity.TimeCycle,
		entity.TimeDuration,
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert event %+v: %v", entity, err)
	}

	return nil
}

type eventDefinitionRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r eventDefinitionRepository) InsertBatch(entities []*internal.EventDefinitionEntity) error {
	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO event_definition (
	element_id,

	process_id,

	bpmn_element_id,
	bpmn_element_type,
	bpmn_process_id,
	is_suspended,
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
	$11
)
`,
			entity.ElementId,

			entity.ProcessId,

			entity.BpmnElementId,
			entity.BpmnElementType.String(),
			entity.BpmnProcessId,
			entity.IsSuspended,
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
	is_suspended,
	signal_name,
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
		&entity.IsSuspended,
		&entity.SignalName,
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

func (r eventDefinitionRepository) SelectBySignalName(signalName string) ([]*internal.EventDefinitionEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	element_id,

	process_id,

	bpmn_element_id,
	bpmn_element_type,
	bpmn_process_id,
	is_suspended,
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
			&entity.IsSuspended,
			&entity.SignalName,
			&entity.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event definition row: %v", err)
		}

		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.IsSuspended = false
		entity.SignalName = pgtype.Text{String: signalName, Valid: true}

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r eventDefinitionRepository) UpdateBatch(entities []*internal.EventDefinitionEntity) error {
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

type eventVariableRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r eventVariableRepository) InsertBatch(entities []*internal.EventVariableEntity) error {
	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO event_variable (
	partition,
	id,

	event_id,

	encoding,
	is_encrypted,
	name,
	value
) VALUES (
	$1,
	nextval($2),

	$3,

	$4,
	$5,
	$6,
	$7
) RETURNING id
`,
			entity.Partition,
			partitionSequence("event_variable", entity.Partition),

			entity.EventId,

			entity.Encoding,
			entity.IsEncrypted,
			entity.Name,
			entity.Value,
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := range entities {
		row := batchResults.QueryRow()

		if err := row.Scan(&entities[i].Id); err != nil {
			return fmt.Errorf("failed to insert event variable %+v: %v", entities[i], err)
		}
	}

	return nil
}

func (r eventVariableRepository) SelectByEvent(partition time.Time, eventId int32) ([]*internal.EventVariableEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	id,

	encoding,
	is_encrypted,
	name,
	value
FROM
	event_variable
WHERE
	partition = $1 AND
	event_id = $2
`, partition, eventId)
	if err != nil {
		return nil, fmt.Errorf("failed to select event variables by partition %s and event ID %d: %v", partition.Format(time.DateOnly), eventId, err)
	}

	defer rows.Close()

	var entities []*internal.EventVariableEntity
	for rows.Next() {
		var entity internal.EventVariableEntity

		if err := rows.Scan(
			&entity.Id,

			&entity.Encoding,
			&entity.IsEncrypted,
			&entity.Name,
			&entity.Value,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event variable row: %v", err)
		}

		entity.Partition = partition
		entity.EventId = eventId

		entities = append(entities, &entity)
	}

	return entities, nil
}
