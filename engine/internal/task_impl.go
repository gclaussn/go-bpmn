package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
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

	executions, err := ctx.ElementInstances().SelectByProcessInstanceAndState(processInstance)
	if err != nil {
		return err
	}

	for _, execution := range executions {
		if execution.ExecutionCount <= 0 {
			continue // skip non scope
		}

		execution.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		execution.State = engine.InstanceStarted
	}

	processInstance.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
	processInstance.State = engine.InstanceStarted

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		executions:       executions,
		process:          process,
		processInstance:  processInstance,
	}

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
		}
	}

	return ctx.ProcessInstances().Update(processInstance)
}

// TriggerEventTask triggers start or catch events.
//
// In case of a start event, a new process instance is created.
// In case of a boundary or catch event, an execution is continued.
type TriggerEventTask struct {
	MessageId int64         `json:",omitempty"`
	SignalId  int64         `json:",omitempty"`
	Timer     *engine.Timer `json:",omitempty"`
}

func (t TriggerEventTask) Execute(ctx Context, task *TaskEntity) error {
	process, err := ctx.ProcessCache().GetOrCacheById(ctx, task.ProcessId.Int32)
	if err != nil {
		return err
	}

	bpmnElement := process.graph.elementByElementId(task.ElementId.Int32)
	if bpmnElement == nil {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to find BPMN element",
			Detail: fmt.Sprintf("BPMN element with ID %d could not be found", task.ElementId.Int32),
		}
	}

	var ec *executionContext
	switch bpmnElement.Type {
	case
		model.ElementMessageStartEvent,
		model.ElementSignalStartEvent,
		model.ElementTimerStartEvent:
		ec = &executionContext{
			engineOrWorkerId: ctx.Options().EngineId,
			process:          process,
		}
	default:
		processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
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

		ec = &executionContext{
			engineOrWorkerId: ctx.Options().EngineId,
			executions:       []*ElementInstanceEntity{execution},
			process:          process,
			processInstance:  processInstance,
		}
	}

	var interrupting bool
	switch bpmnElement.Type {
	case
		model.ElementSignalBoundaryEvent,
		model.ElementTimerBoundaryEvent:
		interrupting = bpmnElement.Model.(model.BoundaryEvent).CancelActivity
	}

	switch bpmnElement.Type {
	case model.ElementMessageCatchEvent:
		return ec.triggerMessageCatchEvent(ctx, t.MessageId)
	case model.ElementMessageStartEvent:
		expireMessage := t.Timer != nil
		return ec.triggerMessageStartEvent(ctx, task, bpmnElement, t.MessageId, expireMessage)
	case model.ElementSignalBoundaryEvent:
		return ec.triggerSignalBoundaryEvent(ctx, t.SignalId, interrupting)
	case model.ElementSignalCatchEvent:
		return ec.triggerSignalCatchEvent(ctx, t.SignalId)
	case model.ElementSignalStartEvent:
		return ec.triggerSignalStartEvent(ctx, task, bpmnElement, t.SignalId)
	case model.ElementTimerBoundaryEvent:
		return ec.triggerTimerBoundaryEvent(ctx, *t.Timer, interrupting)
	case model.ElementTimerCatchEvent:
		return ec.triggerTimerCatchEvent(ctx, *t.Timer)
	case model.ElementTimerStartEvent:
		return ec.triggerTimerStartEvent(ctx, task, *t.Timer)
	default:
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to trigger event",
			Detail: "event is not supported",
		}
	}
}
