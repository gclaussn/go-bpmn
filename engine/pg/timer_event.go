package pg

import (
	"context"
	"fmt"

	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type timerEventRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r timerEventRepository) Insert(entities []*internal.TimerEventEntity) error {
	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO timer_event (
	element_id,

	process_id,

	bpmn_process_id,
	is_suspended,
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
	$8
)
`,
			entity.ElementId,

			entity.ProcessId,

			entity.BpmnProcessId,
			entity.IsSuspended,
			entity.Time,
			entity.TimeCycle,
			entity.TimeDuration,
			entity.Version,
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := 0; i < len(entities); i++ {
		if _, err := batchResults.Exec(); err != nil {
			return fmt.Errorf("failed to insert timer event %+v: %v", entities[i], err)
		}
	}

	return nil
}

func (r timerEventRepository) SelectByBpmnProcessId(bpmnProcessId string) ([]*internal.TimerEventEntity, error) {
	rows, err := r.tx.Query(r.txCtx, `
SELECT
	element_id,

	process_id,

	is_suspended,
	time,
	time_cycle,
	time_duration,
	version
FROM
	timer_event
WHERE
	bpmn_process_id = $1
`, bpmnProcessId)
	if err != nil {
		return nil, fmt.Errorf("failed to select timer events by BPMN process ID %s: %v", bpmnProcessId, err)
	}

	defer rows.Close()

	var entities []*internal.TimerEventEntity
	for rows.Next() {
		var entity internal.TimerEventEntity

		if err := rows.Scan(
			&entity.ElementId,

			&entity.ProcessId,

			&entity.IsSuspended,
			&entity.Time,
			&entity.TimeCycle,
			&entity.TimeDuration,
			&entity.Version,
		); err != nil {
			return nil, fmt.Errorf("failed to scan timer event row: %v", err)
		}

		entity.BpmnProcessId = bpmnProcessId

		entities = append(entities, &entity)
	}

	return entities, nil
}

func (r timerEventRepository) Update(entities []*internal.TimerEventEntity) error {
	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
UPDATE
	timer_event
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

	for i := 0; i < len(entities); i++ {
		if _, err := batchResults.Exec(); err != nil {
			return fmt.Errorf("failed to update timer event %+v: %v", entities[i], err)
		}
	}

	return nil
}
