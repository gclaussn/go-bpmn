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
			Title:  "failed to find process instance",
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

// TriggerTimerEventTask is executed when a timer is due.
//
// In case of a timer
//
//   - catch event: continues the execution, if the process instance and element instance are not ended
type TriggerTimerEventTask struct {
}

func (t TriggerTimerEventTask) Execute(ctx Context, task *TaskEntity) error {
	process, err := ctx.ProcessCache().GetOrCacheById(ctx, task.ProcessId.Int32)
	if err != nil {
		return err
	}

	if task.ProcessInstanceId.Valid {
		processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
		if err == pgx.ErrNoRows { // indicates a bug
			return engine.Error{
				Title:  "failed to find process instance",
				Detail: fmt.Sprintf("process instance %s/%d could not be found", task.Partition.Format(time.DateOnly), task.ProcessInstanceId.Int32),
			}
		}
		if err != nil {
			return err
		}

		if processInstance.EndedAt.Valid {
			return nil
		}

		execution, err := ctx.ElementInstances().Select(task.Partition, task.ElementInstanceId.Int32)
		if err != nil {
			return err
		}

		if execution.EndedAt.Valid {
			return nil
		}

		ec := executionContext{
			engineOrWorkerId: ctx.Options().EngineId,
			process:          process,
			processInstance:  processInstance,
		}

		if err := ec.continueExecutions(ctx, []*ElementInstanceEntity{execution}); err != nil {
			if _, ok := err.(engine.Error); ok {
				task.Error = pgtype.Text{String: err.Error(), Valid: true}
			} else {
				return fmt.Errorf("failed to continue execution %+v: %v", execution, err)
			}
		}

		return nil
	}

	timerEvent, err := ctx.TimerEvents().Select(task.ElementId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{ // indicates a bug
			Title:  "failed to find timer event",
			Detail: fmt.Sprintf("timer event %d could not be found", task.ElementId.Int32),
		}
	}
	if err != nil {
		return err
	}

	if timerEvent.IsSuspended {
		return nil
	}

	processInstance := ProcessInstanceEntity{
		Partition: ctx.Date(),

		ProcessId: process.Id,

		BpmnProcessId:  process.BpmnProcessId,
		CreatedAt:      ctx.Time(),
		CreatedBy:      ctx.Options().EngineId,
		State:          engine.InstanceCreated,
		StateChangedBy: ctx.Options().EngineId,
		Version:        process.Version,
	}

	if err := ctx.ProcessInstances().Insert(&processInstance); err != nil {
		return err
	}

	if err := enqueueProcessInstance(ctx, &processInstance); err != nil {
		return fmt.Errorf("failed to enqueue process instance: %v", err)
	}

	scope := process.graph.createProcessScope(&processInstance)

	execution, err := process.graph.createExecutionAt(&scope, timerEvent.BpmnElementId)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create execution",
			Detail: err.Error(),
		}
	}

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		process:          process,
		processInstance:  &processInstance,
	}

	executions := []*ElementInstanceEntity{&scope, &execution}
	if err := ec.continueExecutions(ctx, executions); err != nil {
		return err
	}

	if !timerEvent.TimeCycle.Valid {
		return nil // one-time event
	}

	// insert task for next time cycle
	timer := engine.Timer{TimeCycle: timerEvent.TimeCycle.String}

	dueAt, err := evaluateTimer(timer, task.DueAt)
	if err != nil {
		return engine.Error{ // indicates a bug
			Title:  "failed to evaluate timer",
			Detail: err.Error(),
		}
	}

	triggerTimerEvent := TaskEntity{
		Partition: ctx.Date(),

		ElementId: task.ElementId,
		ProcessId: task.ProcessId,

		CreatedAt: ctx.Time(),
		CreatedBy: ctx.Options().EngineId,
		DueAt:     dueAt,
		Type:      engine.TaskTriggerTimerEvent,

		Instance: TriggerTimerEventTask{},
	}

	if err := ctx.Tasks().Insert(&triggerTimerEvent); err != nil {
		return err
	}

	return nil
}
