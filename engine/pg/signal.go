package pg

import (
	"context"
	"fmt"

	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

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
	element_instance_id,
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
