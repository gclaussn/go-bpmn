package internal

import (
	"context"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5/pgtype"
)

func NewTaskExecutor(e engine.Engine, interval time.Duration, taskLimit int) *TaskExecutor {
	tickerCtx, tickerCancel := context.WithCancel(context.Background())

	return &TaskExecutor{
		engine:    e,
		taskLimit: taskLimit,

		tickerCtx:    tickerCtx,
		tickerCancel: tickerCancel,
		ticker:       time.NewTicker(interval),
	}
}

type Task interface {
	// Execute executes the task's logic, using the context of an entity.
	//
	// If [engine.Error] is returned, the task completes, but a retry task or an incident is created.
	// If a different error is returned, the task fails.
	Execute(Context, *TaskEntity) error
}

type TaskEntity struct {
	Partition time.Time
	Id        int32

	ElementId         pgtype.Int4
	ElementInstanceId pgtype.Int4
	EventId           pgtype.Int4
	ProcessId         pgtype.Int4
	ProcessInstanceId pgtype.Int4

	CompletedAt    pgtype.Timestamp
	CreatedAt      time.Time
	CreatedBy      string
	DueAt          time.Time
	Error          pgtype.Text
	LockedAt       pgtype.Timestamp
	LockedBy       pgtype.Text
	RetryCount     int
	RetryTimer     pgtype.Text
	SerializedTask pgtype.Text
	Type           engine.TaskType

	Instance Task // task instance that should be executed
}

func (e TaskEntity) Task() engine.Task {
	return engine.Task{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		ElementId:         e.ElementId.Int32,
		ElementInstanceId: e.ElementInstanceId.Int32,
		EventId:           e.EventId.Int32,
		ProcessId:         e.ProcessId.Int32,
		ProcessInstanceId: e.ProcessInstanceId.Int32,

		CompletedAt:    timeOrNil(e.CompletedAt),
		CreatedAt:      e.CreatedAt,
		CreatedBy:      e.CreatedBy,
		DueAt:          e.DueAt,
		Error:          e.Error.String,
		LockedAt:       timeOrNil(e.LockedAt),
		LockedBy:       e.LockedBy.String,
		RetryCount:     e.RetryCount,
		RetryTimer:     engine.ISO8601Duration(e.RetryTimer.String),
		SerializedTask: e.SerializedTask.String,
		Type:           e.Type,
	}
}

type TaskExecutor struct {
	engine    engine.Engine
	taskLimit int

	tickerCtx    context.Context
	tickerCancel context.CancelFunc
	ticker       *time.Ticker
}

func (e *TaskExecutor) Execute() {
	go func() {
		for {
			select {
			case <-e.ticker.C:
				_, _, _ = e.engine.ExecuteTasks(engine.ExecuteTasksCmd{Limit: e.taskLimit})
			case <-e.tickerCtx.Done():
				return
			}
		}
	}()
}

func (e *TaskExecutor) Stop() {
	e.ticker.Stop()
	e.tickerCancel()
}

type TaskRepository interface {
	Insert(*TaskEntity) error
	InsertBatch([]*TaskEntity) error
	Select(partition time.Time, id int32) (*TaskEntity, error)
	Update(*TaskEntity) error

	Query(engine.TaskCriteria, engine.QueryOptions) ([]any, error)

	Lock(cmd engine.ExecuteTasksCmd, lockedAt time.Time) ([]*TaskEntity, error)
	Unlock(engine.UnlockTasksCmd) (int, error)
}

// ExecuteTask executes the instance of a task.
//
// If the execution fails, a retry task is created. When all retries are depleted an incident is created.
func ExecuteTask(ctx Context, task *TaskEntity) error {
	retryCount := task.RetryCount

	if task.Instance != nil {
		if err := task.Instance.Execute(ctx, task); err != nil {
			if _, ok := err.(engine.Error); ok {
				task.Error = pgtype.Text{String: err.Error(), Valid: true}
			} else {
				return err
			}
		}
	} else {
		// in case of an undefined task type or incomplete mapping - see engine/pg/task.go:taskRepository#Lock
		err := engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to map task type",
			Detail: fmt.Sprintf("type %s is not supported", task.Type),
		}

		task.Error = pgtype.Text{String: err.Error(), Valid: true}
		retryCount = 0
	}

	task.CompletedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}

	if err := ctx.Tasks().Update(task); err != nil {
		return err
	}

	if !task.Error.Valid {
		return nil
	}

	if retryCount > 0 {
		retryTimer := engine.ISO8601Duration(task.RetryTimer.String)

		retry := TaskEntity{
			Partition: task.Partition,

			ElementId:         task.ElementId,
			ElementInstanceId: task.ElementInstanceId,
			EventId:           task.EventId,
			ProcessId:         task.ProcessId,
			ProcessInstanceId: task.ProcessInstanceId,

			CreatedAt:      ctx.Time(),
			CreatedBy:      ctx.Options().EngineId,
			DueAt:          retryTimer.Calculate(ctx.Time()),
			RetryCount:     retryCount - 1,
			RetryTimer:     task.RetryTimer,
			SerializedTask: task.SerializedTask,
			Type:           task.Type,

			Instance: task.Instance,
		}

		return ctx.Tasks().Insert(&retry)
	} else {
		incident := IncidentEntity{
			Partition: task.Partition,

			ElementId:         task.ElementId,
			ElementInstanceId: task.ElementInstanceId,
			ProcessId:         task.ProcessId,
			ProcessInstanceId: task.ProcessInstanceId,
			TaskId:            pgtype.Int4{Int32: task.Id, Valid: true},

			CreatedAt: ctx.Time(),
			CreatedBy: ctx.Options().EngineId,
		}

		return ctx.Incidents().Insert(&incident)
	}
}

func UnlockTasks(ctx Context, cmd engine.UnlockTasksCmd) (int, error) {
	return ctx.Tasks().Unlock(cmd)
}
