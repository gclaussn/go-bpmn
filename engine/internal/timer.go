package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type TimerEventEntity struct {
	ElementId int32

	ProcessId int32

	BpmnElementId string
	BpmnProcessId string
	IsSuspended   bool
	Time          pgtype.Timestamp
	TimeCycle     pgtype.Text
	TimeDuration  pgtype.Text
	Version       string
}

type TimerEventRepository interface {
	InsertBatch([]*TimerEventEntity) error
	Select(elementId int32) (*TimerEventEntity, error)
	SelectByBpmnProcessId(bpmnProcessId string) ([]*TimerEventEntity, error)
	UpdateBatch([]*TimerEventEntity) error
}

func suspendTimerEvents(ctx Context, bpmnProcessId string) error {
	events, err := ctx.TimerEvents().SelectByBpmnProcessId(bpmnProcessId)
	if err != nil {
		return err
	}

	var suspendedEvents []*TimerEventEntity
	for _, event := range events {
		if event.IsSuspended {
			continue
		}

		event.IsSuspended = true
		suspendedEvents = append(suspendedEvents, event)
	}

	return ctx.TimerEvents().UpdateBatch(suspendedEvents)
}

func triggerTimerCatchEvent(ctx Context, task *TaskEntity, process *ProcessEntity) error {
	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to trigger timer event",
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

func triggerTimerStartEvent(ctx Context, task *TaskEntity, process *ProcessEntity) error {
	timerEvent, err := ctx.TimerEvents().Select(task.ElementId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to trigger timer event",
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
		StartedAt:      pgtype.Timestamp{Time: ctx.Time(), Valid: true},
		State:          engine.InstanceStarted,
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
		return engine.Error{
			Type:   engine.ErrorBug,
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
		Type:      engine.TaskTriggerEvent,

		Instance: TriggerEventTask{},
	}

	if err := ctx.Tasks().Insert(&triggerTimerEvent); err != nil {
		return err
	}

	return nil
}
