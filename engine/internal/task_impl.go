package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type DequeueProcessInstanceTask struct {
	BpmnProcessId string
}

func (t DequeueProcessInstanceTask) Execute(ctx Context, task *TaskEntity) error {
	queue, err := ctx.ProcessInstanceQueues().Select(t.BpmnProcessId)
	if err != nil {
		return err
	}

	if !queue.MustDequeue() {
		return nil
	}

	head, err := ctx.ProcessInstanceQueues().SelectElement(queue.HeadPartition.Time, queue.HeadId.Int32)
	if err != nil {
		return err
	}

	if head.NextId.Valid {
		queue.HeadId = head.NextId
		queue.HeadPartition = head.NextPartition
	} else {
		queue.HeadId = pgtype.Int4{}
		queue.HeadPartition = pgtype.Date{}
		queue.TailId = pgtype.Int4{}
		queue.TailPartition = pgtype.Date{}
	}

	startProcessInstance := TaskEntity{
		Partition: head.Partition,

		ProcessId:         pgtype.Int4{Int32: head.ProcessId, Valid: true},
		ProcessInstanceId: pgtype.Int4{Int32: head.Id, Valid: true},

		CreatedAt: ctx.Time(),
		CreatedBy: ctx.Options().EngineId,
		DueAt:     ctx.Time(),
		Type:      engine.TaskStartProcessInstance,

		Instance: StartProcessInstanceTask{},
	}

	if err := ctx.Tasks().Insert(&startProcessInstance); err != nil {
		return err
	}

	queue.ActiveCount = queue.ActiveCount + 1
	queue.QueuedCount = queue.QueuedCount - 1

	if err := ctx.ProcessInstanceQueues().Update(queue); err != nil {
		return err
	}

	if !queue.MustDequeue() {
		return nil
	}

	dequeueProcessInstance := TaskEntity{
		Partition: ctx.Date(),

		ProcessId: task.ProcessId,

		CreatedAt: ctx.Time(),
		CreatedBy: ctx.Options().EngineId,
		DueAt:     ctx.Time(),
		Type:      engine.TaskDequeueProcessInstance,

		Instance: DequeueProcessInstanceTask{BpmnProcessId: t.BpmnProcessId},
	}

	return ctx.Tasks().Insert(&dequeueProcessInstance)
}

type JoinParallelGatewayTask struct {
}

func (t JoinParallelGatewayTask) Execute(ctx Context, task *TaskEntity) error {
	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to find process instance",
			Detail: fmt.Sprintf("process instance %s/%d could not be found", task.Partition.Format(time.DateOnly), task.ProcessInstanceId.Int32),
		}
	}
	if err != nil {
		return err
	}

	if processInstance.State != engine.InstanceStarted && processInstance.State != engine.InstanceSuspended {
		return nil
	}

	process, err := ctx.ProcessCache().GetOrCacheById(ctx, processInstance.ProcessId)
	if err != nil {
		return err
	}

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		process:          process,
		processInstance:  processInstance,
	}

	return ec.handleParallelGateway(ctx, task)
}

type StartProcessInstanceTask struct {
}

func (t StartProcessInstanceTask) Execute(ctx Context, task *TaskEntity) error {
	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to start process instance",
			Detail: fmt.Sprintf("process instance %s/%d could not be found", task.Partition.Format(time.DateOnly), task.ProcessInstanceId.Int32),
		}
	}
	if err != nil {
		return err
	}

	if processInstance.State != engine.InstanceQueued {
		return nil
	}

	process, err := ctx.ProcessCache().GetOrCacheById(ctx, processInstance.ProcessId)
	if err != nil {
		return err
	}

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		process:          process,
		processInstance:  processInstance,
	}

	executions, err := ctx.ElementInstances().SelectByProcessInstanceAndState(processInstance)
	if err != nil {
		return err
	}

	for _, execution := range executions {
		if execution.ExecutionCount == 0 {
			continue // skip non scopes
		}

		execution.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		execution.State = engine.InstanceStarted
		execution.StateChangedBy = ec.engineOrWorkerId
	}

	processInstance.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
	processInstance.State = engine.InstanceStarted
	processInstance.StateChangedBy = ec.engineOrWorkerId

	if err := ec.continueExecutions(ctx, executions); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", executions, err)
		}
	}

	return ctx.ProcessInstances().Update(processInstance)
}

// TriggerEventTask triggers start or catch events.
//
// In case of a start event, a new process instance is created.
// In case of a catch event, an execution is continued.
type TriggerEventTask struct {
	SignalId  int64         `json:",omitempty"`
	Timer     *engine.Timer `json:",omitempty"`
}

func (t TriggerEventTask) Execute(ctx Context, task *TaskEntity) error {
	switch {
	case t.SignalId != 0:
		if task.ElementInstanceId.Valid {
			return triggerSignalCatchEvent(ctx, task, t.SignalId)
		} else {
			return triggerSignalStartEvent(ctx, task, t.SignalId)
		}
	case t.Timer != nil:
		if task.ElementInstanceId.Valid {
			return triggerTimerCatchEvent(ctx, task, *t.Timer)
		} else {
			return triggerTimerStartEvent(ctx, task, *t.Timer)
		}
	default:
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to trigger event",
			Detail: "event is not supported",
		}
	}
}
