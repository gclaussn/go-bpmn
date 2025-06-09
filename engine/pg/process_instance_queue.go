package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type processInstanceQueueRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r processInstanceQueueRepository) InsertElement(entity *internal.ProcessInstanceQueueElementEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
INSERT INTO process_instance_queue_element (
	partition,
	id,

	process_id,

	next_partition,
	next_id,
	prev_partition,
	prev_id
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
		entity.Partition,
		entity.Id,

		entity.ProcessId,

		entity.NextPartition,
		entity.NextId,
		entity.PrevPartition,
		entity.PrevId,
	); err != nil {
		return fmt.Errorf("failed to insert process instance queue element %+v: %v", entity, err)
	}

	return nil
}

func (r processInstanceQueueRepository) Select(bpmnProcessId string) (*internal.ProcessInstanceQueueEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	parallelism,
	active_count,
	queued_count,
	head_partition,
	head_id,
	tail_partition,
	tail_id
FROM
	process_instance_queue
WHERE
	bpmn_process_id = $1
FOR UPDATE
`, bpmnProcessId)

	var entity internal.ProcessInstanceQueueEntity
	if err := row.Scan(
		&entity.Parallelism,
		&entity.ActiveCount,
		&entity.QueuedCount,
		&entity.HeadPartition,
		&entity.HeadId,
		&entity.TailPartition,
		&entity.TailId,
	); err != nil {
		return nil, fmt.Errorf("failed to select process instance queue %s: %v", bpmnProcessId, err)
	}

	entity.BpmnProcessId = bpmnProcessId

	return &entity, nil
}

func (r processInstanceQueueRepository) SelectElement(partition time.Time, id int32) (*internal.ProcessInstanceQueueElementEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	process_id,

	next_partition,
	next_id,
	prev_partition,
	prev_id
FROM
	process_instance_queue_element
WHERE
	partition = $1 AND
	id = $2
`, partition, id)

	var entity internal.ProcessInstanceQueueElementEntity
	if err := row.Scan(
		&entity.ProcessId,

		&entity.NextPartition,
		&entity.NextId,
		&entity.PrevPartition,
		&entity.PrevId,
	); err != nil {
		return nil, fmt.Errorf("failed to select process instance queue element %s/%d: %v", partition.Format(time.DateOnly), id, err)
	}

	entity.Partition = partition
	entity.Id = id

	return &entity, nil
}

func (r processInstanceQueueRepository) Update(entity *internal.ProcessInstanceQueueEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	process_instance_queue
SET
	active_count = $2,
	queued_count = $3,
	head_partition = $4,
	head_id = $5,
	tail_partition = $6,
	tail_id = $7
WHERE
	bpmn_process_id = $1
`,
		entity.BpmnProcessId,

		entity.ActiveCount,
		entity.QueuedCount,
		entity.HeadPartition,
		entity.HeadId,
		entity.TailPartition,
		entity.TailId,
	); err != nil {
		return fmt.Errorf("failed to update process instance queue %+v: %v", entity, err)
	}

	return nil
}

func (r processInstanceQueueRepository) UpdateElement(entity *internal.ProcessInstanceQueueElementEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	process_instance_queue_element
SET
	next_partition = $3,
	next_id = $4,
	prev_partition = $5,
	prev_id = $6
WHERE
	partition = $1 AND
	id = $2
`,
		entity.Partition,
		entity.Id,

		entity.NextPartition,
		entity.NextId,
		entity.PrevPartition,
		entity.PrevId,
	); err != nil {
		return fmt.Errorf("failed to update process instance queue element %+v: %v", entity, err)
	}

	return nil
}

func (r processInstanceQueueRepository) Upsert(entity *internal.ProcessInstanceQueueEntity) error {
	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO process_instance_queue (
	bpmn_process_id,

	parallelism,
	active_count,
	queued_count,
	head_partition,
	head_id,
	tail_partition,
	tail_id
) VALUES (
	$1,

	$2,
	$3,
	$4,
	$5,
	$6,
	$7,
	$8
) ON CONFLICT (bpmn_process_id) DO UPDATE
SET
	parallelism = EXCLUDED.parallelism
RETURNING
	active_count,
	queued_count
`,
		entity.BpmnProcessId,

		entity.Parallelism,
		entity.ActiveCount,
		entity.QueuedCount,
		entity.HeadPartition,
		entity.HeadId,
		entity.TailPartition,
		entity.TailId,
	)

	if err := row.Scan(
		&entity.ActiveCount,
		&entity.QueuedCount,
	); err != nil {
		return fmt.Errorf("failed to upsert process instance queue %+v: %v", entity, err)
	}

	return nil
}
