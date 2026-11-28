package pg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type taskRepository struct {
	tx    pgx.Tx
	txCtx context.Context

	engineId string
}

func (r taskRepository) Insert(entity *internal.TaskEntity) error {
	b, err := json.Marshal(entity.Instance)
	if err != nil {
		return fmt.Errorf("failed to marshal task instance: %v", err)
	}

	if len(b) != 2 {
		entity.SerializedTask = pgtype.Text{String: string(b), Valid: true}
	}

	row := r.tx.QueryRow(r.txCtx, `
INSERT INTO task (
	partition,
	id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	created_at,
	created_by,
	due_at,
	retry_count,
	serialized_task,
	state,
	type
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
	$15
) RETURNING id
`,
		entity.Partition,
		partitionSequence("task", entity.Partition),

		entity.ElementId,
		entity.ElementInstanceId,
		entity.ProcessId,
		entity.ProcessInstanceId,

		entity.BpmnElementId,
		pgtype.Text{String: entity.BpmnElementType.String(), Valid: entity.BpmnElementType != 0},
		entity.CreatedAt,
		entity.CreatedBy,
		entity.DueAt,
		entity.RetryCount,
		entity.SerializedTask,
		entity.State.String(),
		entity.Type.String(),
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert task %+v: %v", entity, err)
	}

	// insert into queue
	if _, err := r.tx.Exec(r.txCtx, `
INSERT INTO task_queue (
	partition,
	task_id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	due_at,
	retry_count,
	serialized_task,
	type
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
	$12
)
`,
		entity.Partition,
		entity.Id,

		entity.ElementId,
		entity.ElementInstanceId,
		entity.ProcessId,
		entity.ProcessInstanceId,

		entity.BpmnElementId,
		pgtype.Text{String: entity.BpmnElementType.String(), Valid: entity.BpmnElementType != 0},
		entity.DueAt,
		entity.RetryCount,
		entity.SerializedTask,
		entity.Type.String(),
	); err != nil {
		return fmt.Errorf("failed to insert task %+v into queue: %v", entity, err)
	}

	return nil
}

func (r taskRepository) InsertBatch(entities []*internal.TaskEntity) error {
	if len(entities) == 0 {
		return nil
	}

	for _, entity := range entities {
		b, err := json.Marshal(entity.Instance)
		if err != nil {
			return fmt.Errorf("failed to marshal task instance: %v", err)
		}

		if len(b) != 2 {
			entity.SerializedTask = pgtype.Text{String: string(b), Valid: true}
		}
	}

	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO task (
	partition,
	id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	created_at,
	created_by,
	due_at,
	retry_count,
	serialized_task,
	state,
	type
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
	$15
) RETURNING id
`,
			entity.Partition,
			partitionSequence("task", entity.Partition),

			entity.ElementId,
			entity.ElementInstanceId,
			entity.ProcessId,
			entity.ProcessInstanceId,

			entity.BpmnElementId,
			pgtype.Text{String: entity.BpmnElementType.String(), Valid: entity.BpmnElementType != 0},
			entity.CreatedAt,
			entity.CreatedBy,
			entity.DueAt,
			entity.RetryCount,
			entity.SerializedTask,
			entity.State.String(),
			entity.Type.String(),
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := range entities {
		row := batchResults.QueryRow()

		if err := row.Scan(&entities[i].Id); err != nil {
			return fmt.Errorf("failed to insert task %+v: %v", entities[i], err)
		}
	}

	// insert into queue
	queueBatch := &pgx.Batch{}

	for _, entity := range entities {
		queueBatch.Queue(`
INSERT INTO task_queue (
	partition,
	task_id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_elemnt_type,
	due_at,
	retry_count,
	serialized_task,
	type
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
	$12
)
`,
			entity.Partition,
			entity.Id,

			entity.ElementId,
			entity.ElementInstanceId,
			entity.ProcessId,
			entity.ProcessInstanceId,

			entity.BpmnElementId,
			pgtype.Text{String: entity.BpmnElementType.String(), Valid: entity.BpmnElementType != 0},
			entity.DueAt,
			entity.RetryCount,
			entity.SerializedTask,
			entity.Type.String(),
		)
	}

	queueBatchResults := r.tx.SendBatch(r.txCtx, queueBatch)
	defer queueBatchResults.Close()

	for i := range entities {
		if _, err := queueBatchResults.Exec(); err != nil {
			return fmt.Errorf("failed to insert task %+v into queue: %v", entities[i], err)
		}
	}

	return nil
}

func (r taskRepository) Select(partition time.Time, id int32) (*internal.TaskEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	completed_at,
	created_at,
	created_by,
	due_at,
	error,
	locked_at,
	locked_by,
	retry_count,
	serialized_task,
	state,
	type
FROM
	task
WHERE
	partition = $1 AND
	id = $2
`, partition, id)

	var (
		entity internal.TaskEntity

		bpmnElementTypeValue string
		stateValue           string
		typeValue            string
	)
	if err := row.Scan(
		&entity.ElementId,
		&entity.ElementInstanceId,
		&entity.ProcessId,
		&entity.ProcessInstanceId,

		&entity.BpmnElementId,
		&bpmnElementTypeValue,
		&entity.CompletedAt,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.DueAt,
		&entity.Error,
		&entity.LockedAt,
		&entity.LockedBy,
		&entity.RetryCount,
		&entity.SerializedTask,
		&stateValue,
		&typeValue,
	); err != nil {
		return nil, fmt.Errorf("failed to select task %s/%d: %v", partition.Format(time.DateOnly), id, err)
	}

	entity.Partition = partition
	entity.Id = id
	entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
	entity.State = engine.MapWorkState(stateValue)
	entity.Type = engine.MapTaskType(typeValue)

	return &entity, nil
}

func (r taskRepository) Update(entity *internal.TaskEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	task
SET
	completed_at = $3,
	error = $4,
	locked_at = $5,
	locked_by = $6,
	state = $7
WHERE
	partition = $1 AND
	id = $2
`,
		entity.Partition,
		entity.Id,

		entity.CompletedAt,
		entity.Error,
		entity.LockedAt,
		entity.LockedBy,
		entity.State.String(),
	); err != nil {
		return fmt.Errorf("failed to update task %+v: %v", entity, err)
	}

	if !entity.CompletedAt.Valid {
		return nil
	}

	if _, err := r.tx.Exec(r.txCtx, `
DELETE FROM
	task_queue_lock
WHERE
	partition = $1 AND
	task_id = $2
`,
		entity.Partition,
		entity.Id,
	); err != nil {
		return fmt.Errorf("failed to delete locked task %s/%d from queue lock: %v", entity.Partition.Format(time.DateOnly), entity.Id, err)
	}

	return nil
}

func (r taskRepository) Query(criteria engine.TaskCriteria, options engine.QueryOptions, now time.Time) ([]engine.Task, error) {
	var sql bytes.Buffer
	if err := sqlTaskQuery.Execute(&sql, map[string]any{
		"c": criteria,
		"o": options,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute task query template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return nil, fmt.Errorf("failed to execute task query: %v", err)
	}

	defer rows.Close()

	results := make([]engine.Task, 0)
	for rows.Next() {
		var (
			entity internal.TaskEntity

			bpmnElementTypeValue string
			stateValue           string
			typeValue            string
		)

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.CompletedAt,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.DueAt,
			&entity.Error,
			&entity.LockedAt,
			&entity.LockedBy,
			&entity.RetryCount,
			&entity.SerializedTask,
			&stateValue,
			&typeValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task row: %v", err)
		}

		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.State = engine.MapWorkState(stateValue)
		entity.Type = engine.MapTaskType(typeValue)

		if entity.State == engine.WorkCreated && !now.Before(entity.DueAt) {
			entity.State = engine.WorkDue
		}

		results = append(results, entity.Task())
	}

	return results, nil
}

func (r taskRepository) Lock(cmd engine.ExecuteTasksCmd, lockedAt time.Time) ([]*internal.TaskEntity, error) {
	if cmd.Limit <= 0 {
		cmd.Limit = 1
	}

	var sql bytes.Buffer
	if err := sqlTaskLock.Execute(&sql, cmd); err != nil {
		return nil, fmt.Errorf("failed to execute task lock template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String(), lockedAt, r.engineId)
	if err != nil {
		return nil, fmt.Errorf("failed to lock tasks: %v", err)
	}

	defer rows.Close()

	var (
		queueIds []int64
		entities []*internal.TaskEntity
	)
	for rows.Next() {
		var (
			queueId int64
			entity  internal.TaskEntity

			bpmnElementTypeValue string
			stateValue           string
			typeValue            string
		)

		if err := rows.Scan(
			&queueId,

			&entity.Partition,
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.DueAt,
			&entity.RetryCount,
			&entity.SerializedTask,
			&typeValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task lock row: %v", err)
		}

		queueIds = append(queueIds, queueId)

		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.State = engine.MapWorkState(stateValue)
		entity.Type = engine.MapTaskType(typeValue)

		var err error

		switch entity.Type {
		case engine.TaskContinueExecution:
			entity.Instance = internal.ContinueExecutionTask{}
		case engine.TaskDequeueProcessInstance:
			task := internal.DequeueProcessInstanceTask{}
			err = json.Unmarshal([]byte(entity.SerializedTask.String), &task)
			entity.Instance = task
		case engine.TaskJoinParallelGateway:
			entity.Instance = internal.JoinParallelGatewayTask{}
		case engine.TaskStartProcessInstance:
			entity.Instance = internal.StartProcessInstanceTask{}
		case engine.TaskTerminateProcessInstance:
			entity.Instance = internal.TerminateProcessInstanceTask{}
		case engine.TaskTriggerEvent:
			task := internal.TriggerEventTask{}
			if entity.SerializedTask.Valid {
				err = json.Unmarshal([]byte(entity.SerializedTask.String), &task)
			}
			entity.Instance = task
		// management
		case engine.TaskCreatePartition:
			task := createPartitionTask{}
			err = json.Unmarshal([]byte(entity.SerializedTask.String), &task)
			entity.Instance = task
		case engine.TaskDetachPartition:
			task := detachPartitionTask{}
			err = json.Unmarshal([]byte(entity.SerializedTask.String), &task)
			entity.Instance = task
		case engine.TaskDropPartition:
			task := dropPartitionTask{}
			err = json.Unmarshal([]byte(entity.SerializedTask.String), &task)
			entity.Instance = task
		case engine.TaskPurgeMessages:
			entity.Instance = purgeMessagesTask{}
		case engine.TaskPurgeSignals:
			entity.Instance = purgeSignalsTask{}
		default:
			entity.Instance = nil // see internal/task.go:ExecuteTask
		}

		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal task: %v", err)
		}

		entities = append(entities, &entity)
	}

	if len(entities) == 0 {
		return entities, nil
	}

	// delete from queue
	queueIdsToDelete := make([]string, len(queueIds))
	for i, queueId := range queueIds {
		queueIdsToDelete[i] = strconv.FormatInt(queueId, 10)
	}

	if _, err := r.tx.Exec(r.txCtx, fmt.Sprintf("DELETE FROM task_queue WHERE id IN (%s)", strings.Join(queueIdsToDelete, ","))); err != nil {
		return nil, fmt.Errorf("failed to delete locked tasks from pool: %v", err)
	}

	// insert into queue lock
	batch := &pgx.Batch{}

	for i, entity := range entities {
		batch.Queue(`
INSERT INTO task_queue_lock (
	id,

	partition,
	task_id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	due_at,
	locked_at,
	locked_by,
	retry_count,
	serialized_task,
	type
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
	$13,
	$14,
	$15
)
`,
			queueIds[i],

			entity.Partition,
			entity.Id,

			entity.ElementId,
			entity.ElementInstanceId,
			entity.ProcessId,
			entity.ProcessInstanceId,

			entity.BpmnElementId,
			entity.BpmnElementType.String(),
			entity.DueAt,
			entity.LockedAt,
			entity.LockedBy,
			entity.RetryCount,
			entity.SerializedTask,
			entity.Type.String(),
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := range entities {
		if _, err := batchResults.Exec(); err != nil {
			return nil, fmt.Errorf("failed to insert task %+v into queue lock: %v", entities[i], err)
		}
	}

	return entities, nil
}

func (r taskRepository) Unlock(cmd engine.UnlockTasksCmd) (int, error) {
	// delete from queue lock
	var sql bytes.Buffer
	if err := sqlTaskUnlock.Execute(&sql, cmd); err != nil {
		return -1, fmt.Errorf("failed to execute task unlock template: %v", err)
	}

	rows, err := r.tx.Query(r.txCtx, sql.String())
	if err != nil {
		return -1, fmt.Errorf("failed to unlock tasks: %v", err)
	}

	defer rows.Close()

	var entities []internal.TaskEntity
	for rows.Next() {
		var (
			entity internal.TaskEntity

			bpmnElementTypeValue string
			typeValue            string
		)

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.BpmnElementId,
			&bpmnElementTypeValue,
			&entity.DueAt,
			&entity.RetryCount,
			&entity.SerializedTask,
			&typeValue,
		); err != nil {
			return -1, fmt.Errorf("failed to scan task unlock row: %v", err)
		}

		entity.BpmnElementType = model.MapElementType(bpmnElementTypeValue)
		entity.Type = engine.MapTaskType(typeValue)

		entities = append(entities, entity)
	}

	if len(entities) == 0 {
		return 0, nil
	}

	// insert into queue
	batch := &pgx.Batch{}

	for _, entity := range entities {
		batch.Queue(`
INSERT INTO task_queue (
	partition,
	task_id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_elemnt_type,
	due_at,
	retry_count,
	serialized_task,
	type
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
	$12
)
`,
			entity.Partition,
			entity.Id,

			entity.ElementId,
			entity.ElementInstanceId,
			entity.ProcessId,
			entity.ProcessInstanceId,

			entity.BpmnElementId,
			pgtype.Text{String: entity.BpmnElementType.String(), Valid: entity.BpmnElementType != 0},
			entity.DueAt,
			entity.RetryCount,
			entity.SerializedTask,
			entity.Type.String(),
		)
	}

	batchResults := r.tx.SendBatch(r.txCtx, batch)
	defer batchResults.Close()

	for i := range entities {
		if _, err := batchResults.Exec(); err != nil {
			return -1, fmt.Errorf("failed to insert task %+v into queue: %v", entities[i], err)
		}
	}

	return len(entities), nil
}
