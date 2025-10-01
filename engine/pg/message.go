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

type messageRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r messageRepository) Insert(entity *internal.MessageEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO message (
	correlation_key,
	created_at,
	created_by,
	expires_at,
	is_conflict,
	is_correlated,
	name,
	unique_key
) VALUES (
	$1,
	$2,
	$3,
	$4,
	$5,
	$6,
	$7,
	$8
) ON CONFLICT (name,correlation_key,unique_key) WHERE unique_key IS NOT NULL AND (is_correlated IS FALSE OR expires_at IS NULL) DO UPDATE
SET
	is_conflict = TRUE
RETURNING
	id,

	created_at,
	created_by,
	expires_at,
	is_conflict,
	is_correlated
`,
		entity.CorrelationKey,
		entity.CreatedAt,
		entity.CreatedBy,
		entity.ExpiresAt,
		entity.IsConflict,
		entity.IsCorrelated,
		entity.Name,
		entity.UniqueKey,
	)

	if err := row.Scan(
		&entity.Id,

		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.ExpiresAt,
		&entity.IsConflict,
		&entity.IsCorrelated,
	); err != nil {
		return fmt.Errorf("failed to insert message %+v: %v", entity, err)
	}

	return nil
}

func (r messageRepository) Select(id int64) (*internal.MessageEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	correlation_key,
	created_at,
	created_by,
	expires_at,
	is_correlated,
	name,
	unique_key
FROM
	message
WHERE
	id = $1
FOR UPDATE
`, id)

	var entity internal.MessageEntity
	if err := row.Scan(
		&entity.CorrelationKey,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.ExpiresAt,
		&entity.IsCorrelated,
		&entity.Name,
		&entity.UniqueKey,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to select message %d: %v", id, err)
		}
	}

	entity.Id = id

	return &entity, nil
}

func (r messageRepository) SelectBuffered(name string, correlationKey string, now time.Time) (*internal.MessageEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	id,

	created_at,
	created_by,
	expires_at,
	unique_key
FROM
	message
WHERE
	name = $1 AND
	correlation_key = $2 AND
	expires_at > $3 AND
	is_correlated IS FALSE
ORDER BY
	created_at
LIMIT
	1
FOR UPDATE SKIP LOCKED
`, name, correlationKey, now)

	var entity internal.MessageEntity
	if err := row.Scan(
		&entity.Id,

		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.ExpiresAt,
		&entity.UniqueKey,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to select buffered message: %v", err)
		}
	}

	entity.CorrelationKey = correlationKey
	entity.Name = name

	return &entity, nil
}

func (r messageRepository) Update(entity *internal.MessageEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	message
SET
	expires_at = $2,
	is_correlated = $3
WHERE
	id = $1
`,
		entity.Id,

		entity.ExpiresAt,
		entity.IsCorrelated,
	); err != nil {
		return fmt.Errorf("failed to update message %+v: %v", entity, err)
	}

	return nil
}

func (r messageRepository) Query(criteria engine.MessageCriteria, options engine.QueryOptions, now time.Time) ([]engine.Message, error) {
	var sql bytes.Buffer
	if err := sqlMessageQuery.Execute(&sql, map[string]any{
		"c": criteria,
		"o": options,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute message query template: %v", err)
	}

	var args []any
	if criteria.ExcludeExpired {
		args = append(args, now)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute message query: %v", err)
	}

	defer rows.Close()

	var results []engine.Message
	for rows.Next() {
		var entity internal.MessageEntity

		if err := rows.Scan(
			&entity.Id,

			&entity.CorrelationKey,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.ExpiresAt,
			&entity.IsCorrelated,
			&entity.Name,
			&entity.UniqueKey,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message row: %v", err)
		}

		results = append(results, entity.Message())
	}

	return results, nil
}

type messageSubscriptionRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r messageSubscriptionRepository) Delete(entity *internal.MessageSubscriptionEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `DELETE FROM message_subscription WHERE id = $1`, entity.Id); err != nil {
		return fmt.Errorf("failed to delete message subscription %+v: %v", entity, err)
	}
	return nil
}

func (r messageSubscriptionRepository) Insert(entity *internal.MessageSubscriptionEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO message_subscription (
	partition,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	correlation_key,
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
	$8,
	$9
) RETURNING id
`,
		entity.Partition,

		entity.ElementId,
		entity.ElementInstanceId,
		entity.ProcessId,
		entity.ProcessInstanceId,

		entity.CorrelationKey,
		entity.CreatedAt,
		entity.CreatedBy,
		entity.Name,
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert message subscription %+v: %v", entity, err)
	}

	return nil
}

func (r messageSubscriptionRepository) SelectByNameAndCorrelationKey(name string, correlationKey string) (*internal.MessageSubscriptionEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	id,

	partition,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	created_at,
	created_by
FROM
	message_subscription
WHERE
	name = $1 AND
	correlation_key = $2
ORDER BY
	created_at
LIMIT
	1
FOR UPDATE SKIP LOCKED
`, name, correlationKey)

	var entity internal.MessageSubscriptionEntity
	if err := row.Scan(
		&entity.Id,

		&entity.Partition,

		&entity.ElementId,
		&entity.ElementInstanceId,
		&entity.ProcessId,
		&entity.ProcessInstanceId,

		&entity.CreatedAt,
		&entity.CreatedBy,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to select message subscription by name %s and correlation key %s: %v", name, correlationKey, err)
		}
	}

	entity.Name = name
	entity.CorrelationKey = correlationKey

	return &entity, nil
}

type messageVariableRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r messageVariableRepository) InsertBatch(entities []*internal.MessageVariableEntity) error {
	if len(entities) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO message_variable (
	message_id,

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
			entity.MessageId,

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
			return fmt.Errorf("failed to insert message variable %+v: %v", entities[i], err)
		}
	}

	return nil
}

func (r messageVariableRepository) SelectByMessageId(messageId int64) ([]*internal.MessageVariableEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	id,

	encoding,
	is_encrypted,
	name,
	value
FROM
	message_variable
WHERE
	message_id = $1
`, messageId)
	if err != nil {
		return nil, fmt.Errorf("failed to select message variables by message ID %d: %v", messageId, err)
	}

	defer rows.Close()

	var entities []*internal.MessageVariableEntity
	for rows.Next() {
		var entity internal.MessageVariableEntity

		if err := rows.Scan(
			&entity.Id,

			&entity.Encoding,
			&entity.IsEncrypted,
			&entity.Name,
			&entity.Value,
		); err != nil {
			return nil, fmt.Errorf("failed to scan message variable row: %v", err)
		}

		entity.MessageId = messageId

		entities = append(entities, &entity)
	}

	return entities, nil
}
