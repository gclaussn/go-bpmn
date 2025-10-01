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
	active_subscriber_count,
	created_at,
	created_by,
	name,
	subscriber_count
) VALUES (
	$1,
	$2,
	$3,
	$4,
	$5
) RETURNING id
`,
		entity.ActiveSubscriberCount,
		entity.CreatedAt,
		entity.CreatedBy,
		entity.Name,
		entity.SubscriberCount,
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert signal %+v: %v", entity, err)
	}

	return nil
}

func (r signalRepository) Select(id int64) (*internal.SignalEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	active_subscriber_count,
	created_at,
	created_by,
	name,
	subscriber_count
FROM
	signal
WHERE
	id = $1
FOR UPDATE
`, id)

	var entity internal.SignalEntity
	if err := row.Scan(
		&entity.ActiveSubscriberCount,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.Name,
		&entity.SubscriberCount,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to select signal %d: %v", id, err)
		}
	}

	entity.Id = id

	return &entity, nil
}

func (r signalRepository) Update(entity *internal.SignalEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	signal
SET
	active_subscriber_count = $2
WHERE
	id = $1
`,
		entity.Id,

		entity.ActiveSubscriberCount,
	); err != nil {
		return fmt.Errorf("failed to update signal %+v: %v", entity, err)
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

	partition,

	element_id,
	element_instance_id,
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

			&entity.Partition,

			&entity.ElementId,
			&entity.ElementInstanceId,
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
	partition,

	element_id,
	element_instance_id,
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
		entity.Partition,

		entity.ElementId,
		entity.ElementInstanceId,
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

type signalVariableRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r signalVariableRepository) InsertBatch(entities []*internal.SignalVariableEntity) error {
	if len(entities) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO signal_variable (
	signal_id,

	encoding,
	is_encrypted,
	name,
	value
) VALUES (
	$1,

	$2,
	$3,
	$4,
	$5
) RETURNING id
`,
			entity.SignalId,

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
			return fmt.Errorf("failed to insert signal variable %+v: %v", entities[i], err)
		}
	}

	return nil
}

func (r signalVariableRepository) SelectBySignalId(signalId int64) ([]*internal.SignalVariableEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	id,

	encoding,
	is_encrypted,
	name,
	value
FROM
	signal_variable
WHERE
	signal_id = $1
`, signalId)
	if err != nil {
		return nil, fmt.Errorf("failed to select signal variables by signal ID %d: %v", signalId, err)
	}

	defer rows.Close()

	var entities []*internal.SignalVariableEntity
	for rows.Next() {
		var entity internal.SignalVariableEntity

		if err := rows.Scan(
			&entity.Id,

			&entity.Encoding,
			&entity.IsEncrypted,
			&entity.Name,
			&entity.Value,
		); err != nil {
			return nil, fmt.Errorf("failed to scan signal variable row: %v", err)
		}

		entity.SignalId = signalId

		entities = append(entities, &entity)
	}

	return entities, nil
}
