package pg

import (
	"context"
	"fmt"

	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type signalRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r signalRepository) Insert(entity *internal.SignalEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO signal (
	partition,
	id,

	name,
	sent_at,
	sent_by,
	subscriber_count
) VALUES (
	$1,
	nextval($2),

	$3,
	$4,
	$5,
	$6
) RETURNING id
`,
		entity.Partition,
		partitionSequence("signal", entity.Partition),

		entity.Name,
		entity.SentAt,
		entity.SentBy,
		entity.SubscriberCount,
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert signal %+v: %v", entity, err)
	}

	return nil
}

type signalEventRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r signalEventRepository) InsertBatch(entities []*internal.SignalEventEntity) error {
	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO signal_event (
	element_id,

	process_id,

	bpmn_element_id,
	bpmn_process_id,
	is_suspended,
	name,
	version
) VALUES (
	$1,

	$2,

	$3,
	$4,
	$5,
	$6,
	$7
)
`,
			entity.ElementId,

			entity.ProcessId,

			entity.BpmnElementId,
			entity.BpmnProcessId,
			entity.IsSuspended,
			entity.Name,
			entity.Version,
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := range entities {
		if _, err := batchResults.Exec(); err != nil {
			return fmt.Errorf("failed to insert signal event %+v: %v", entities[i], err)
		}
	}

	return nil
}

func (r signalEventRepository) SelectByBpmnProcessId(bpmnProcessId string) ([]*internal.SignalEventEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	element_id,

	process_id,

	bpmn_element_id,
	is_suspended,
	name,
	version
FROM
	signal_event
WHERE
	bpmn_process_id = $1
`, bpmnProcessId)
	if err != nil {
		return nil, fmt.Errorf("failed to select signal events by BPMN process ID %s: %v", bpmnProcessId, err)
	}

	defer rows.Close()

	var entities []*internal.SignalEventEntity
	for rows.Next() {
		var entity internal.SignalEventEntity

		if err := rows.Scan(
			&entity.ElementId,

			&entity.ProcessId,

			&entity.BpmnElementId,
			&entity.IsSuspended,
			&entity.Name,
			&entity.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan signal event row: %v", err)
		}

		entity.BpmnProcessId = bpmnProcessId

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r signalEventRepository) SelectByNameAndNotSuspended(name string) ([]*internal.SignalEventEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	element_id,

	process_id,

	bpmn_element_id,
	bpmn_process_id,
	version
FROM
	signal_event
WHERE
	name = $1 AND
	is_suspended IS FALSE
`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to not suspended select signal events by name %s: %v", name, err)
	}

	defer rows.Close()

	var entities []*internal.SignalEventEntity
	for rows.Next() {
		var entity internal.SignalEventEntity

		if err := rows.Scan(
			&entity.ElementId,

			&entity.ProcessId,

			&entity.BpmnElementId,
			&entity.IsSuspended,
			&entity.Name,
			&entity.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan signal event row: %v", err)
		}

		entity.IsSuspended = false
		entity.Name = name

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r signalEventRepository) UpdateBatch(entities []*internal.SignalEventEntity) error {
	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
UPDATE
	signal_event
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
			return fmt.Errorf("failed to update signal event %+v: %v", entities[i], err)
		}
	}

	return nil
}

type signalSubscriptionRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r signalSubscriptionRepository) DeleteByName(name string) ([]*internal.SignalSubscriptionEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
DELETE FROM
	signal_subscription
WHERE
	name = $1
RETURNING
	id,

	element_id,
	element_instance_id,
	partition,
	process_id,
	process_instance_id,

	created_at,
	created_by
`, name)
	if err != nil {
		return nil, fmt.Errorf("failed to delete signal subscriptions by name %s: %v", name, err)
	}

	defer rows.Close()

	var entities []*internal.SignalSubscriptionEntity
	for rows.Next() {
		var entity internal.SignalSubscriptionEntity

		if err := rows.Scan(
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.Partition,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.CreatedAt,
			&entity.CreatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan signal subscription row: %v", err)
		}

		entity.Name = name

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r signalSubscriptionRepository) Insert(entity *internal.SignalSubscriptionEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO signal_subscription (
	element_id,
	element_instance_id
	partition,
	process_id,
	process_instance_id,

	created_at,
	created_by,
	name
) VALUES (
	$1,
	$2,
	$3,
	$4,
	$5,

	$6,
	$7,
	$8
) RETURNING id
`,
		entity.ElementId,
		entity.ElementInstanceId,
		entity.Partition,
		entity.ProcessId,
		entity.ProcessInstanceId,

		entity.CreatedAt,
		entity.CreatedBy,
		entity.Name,
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert signal subscription %+v: %v", entity, err)
	}

	return nil
}
