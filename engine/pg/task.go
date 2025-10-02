package pg

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
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

	created_at,
	created_by,
	due_at,
	retry_count,
	retry_timer,
	serialized_task,
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
	$13
) RETURNING id
`,
		entity.Partition,
		partitionSequence("task", entity.Partition),

		entity.ElementId,
		entity.ElementInstanceId,
		entity.ProcessId,
		entity.ProcessInstanceId,

		entity.CreatedAt,
		entity.CreatedBy,
		entity.DueAt,
		entity.RetryCount,
		entity.RetryTimer,
		entity.SerializedTask,
		entity.Type.String(),
	)

	if err := row.Scan(&entity.Id); err != nil {
		return fmt.Errorf("failed to insert task %+v: %v", entity, err)
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

	created_at,
	created_by,
	due_at,
	retry_count,
	retry_timer,
	serialized_task,
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
	$13
) RETURNING id
`,
			entity.Partition,
			partitionSequence("task", entity.Partition),

			entity.ElementId,
			entity.ElementInstanceId,
			entity.ProcessId,
			entity.ProcessInstanceId,

			entity.CreatedAt,
			entity.CreatedBy,
			entity.DueAt,
			entity.RetryCount,
			entity.RetryTimer,
			entity.SerializedTask,
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

	return nil
}

func (r taskRepository) Select(partition time.Time, id int32) (*internal.TaskEntity, error) {
	row := r.tx.QueryRow(r.txCtx, `
SELECT
	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	completed_at,
	created_at,
	created_by,
	due_at,
	error,
	locked_at,
	locked_by,
	retry_count,
	retry_timer,
	serialized_task,
	type
FROM
	task
WHERE
	partition = $1 AND
	id = $2
`, partition, id)

	var (
		entity    internal.TaskEntity
		typeValue string
	)
	if err := row.Scan(
		&entity.ElementId,
		&entity.ElementInstanceId,
		&entity.ProcessId,
		&entity.ProcessInstanceId,

		&entity.CompletedAt,
		&entity.CreatedAt,
		&entity.CreatedBy,
		&entity.DueAt,
		&entity.Error,
		&entity.LockedAt,
		&entity.LockedBy,
		&entity.RetryCount,
		&entity.RetryTimer,
		&entity.SerializedTask,
		&typeValue,
	); err != nil {
		return nil, fmt.Errorf("failed to select task %s/%d: %v", partition.Format(time.DateOnly), id, err)
	}

	entity.Partition = partition
	entity.Id = id
	entity.Type = engine.MapTaskType(typeValue)

	return &entity, nil
}

func (r taskRepository) Update(entity *internal.TaskEntity) error {
	if _, err := r.tx.Exec(r.txCtx, `
UPDATE
	task
SET
	completed_at = $3,
	error = $4
WHERE
	partition = $1 AND
	id = $2
`,
		entity.Partition,
		entity.Id,

		entity.CompletedAt,
		entity.Error,
	); err != nil {
		return fmt.Errorf("failed to update task %+v: %v", entity, err)
	}

	return nil
}

func (r taskRepository) Query(criteria engine.TaskCriteria, options engine.QueryOptions) ([]engine.Task, error) {
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
			entity    internal.TaskEntity
			typeValue string
		)

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.CompletedAt,
			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.DueAt,
			&entity.Error,
			&entity.LockedAt,
			&entity.LockedBy,
			&entity.RetryCount,
			&entity.RetryTimer,
			&entity.SerializedTask,
			&typeValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task row: %v", err)
		}

		entity.Type = engine.MapTaskType(typeValue)

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

	var entities []*internal.TaskEntity
	for rows.Next() {
		var (
			entity    internal.TaskEntity
			typeValue string
		)

		if err := rows.Scan(
			&entity.Partition,
			&entity.Id,

			&entity.ElementId,
			&entity.ElementInstanceId,
			&entity.ProcessId,
			&entity.ProcessInstanceId,

			&entity.CreatedAt,
			&entity.CreatedBy,
			&entity.DueAt,
			&entity.LockedAt,
			&entity.LockedBy,
			&entity.RetryCount,
			&entity.RetryTimer,
			&entity.SerializedTask,
			&typeValue,
		); err != nil {
			return nil, fmt.Errorf("failed to scan task lock row: %v", err)
		}

		entity.Type = engine.MapTaskType(typeValue)

		var err error

		switch entity.Type {
		case engine.TaskDequeueProcessInstance:
			task := internal.DequeueProcessInstanceTask{}
			err = json.Unmarshal([]byte(entity.SerializedTask.String), &task)
			entity.Instance = task
		case engine.TaskJoinParallelGateway:
			entity.Instance = internal.JoinParallelGatewayTask{}
		case engine.TaskStartProcessInstance:
			entity.Instance = internal.StartProcessInstanceTask{}
		case engine.TaskTriggerEvent:
			task := internal.TriggerEventTask{}
			err = json.Unmarshal([]byte(entity.SerializedTask.String), &task)
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

	return entities, nil
}

func (r taskRepository) Unlock(cmd engine.UnlockTasksCmd) (int, error) {
	var sql bytes.Buffer
	if err := sqlTaskUnlock.Execute(&sql, cmd); err != nil {
		return -1, fmt.Errorf("failed to execute task unlock template: %v", err)
	}

	commandTag, err := r.tx.Exec(r.txCtx, sql.String())
	if err != nil {
		return -1, fmt.Errorf("failed to unlock tasks: %v", err)
	}

	return int(commandTag.RowsAffected()), nil
}
