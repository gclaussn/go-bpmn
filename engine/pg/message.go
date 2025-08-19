package pg

import (
	"context"
	"fmt"

	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type messageRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

type messageSubscriptionRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r messageSubscriptionRepository) DeleteByNameAndCorrelationKey(name string, correlationKey string) (*internal.MessageSubscriptionEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
DELETE FROM
	message_subscription
WHERE
	name = $1 AND
	correlation_key = $2
RETURNING
	id,

	partition,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	created_at,
	created_by
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
			return nil, fmt.Errorf("failed to delete message subscription by name %s and correlation key %s: %v", name, correlationKey, err)
		}
	}

	entity.Name = name
	entity.CorrelationKey = correlationKey

	return &entity, nil
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

type messageVariableRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}
