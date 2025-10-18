package internal

import (
	"fmt"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5/pgtype"
)

func triggerTimerCatchEvent(ctx Context, task *TaskEntity, timer engine.Timer) error {
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

	process, err := ctx.ProcessCache().GetOrCacheById(ctx, task.ProcessId.Int32)
	if err != nil {
		return err
	}

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		process:          process,
		processInstance:  processInstance,
	}

	if err := ec.continueExecutions(ctx, []*ElementInstanceEntity{execution}); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue execution %+v: %v", execution, err)
		}
	}

	event := EventEntity{
		Partition: execution.Partition,

		ElementInstanceId: execution.Id,

		CreatedAt:    ctx.Time(),
		CreatedBy:    ctx.Options().EngineId,
		Time:         pgtype.Timestamp{Time: timer.Time, Valid: !timer.Time.IsZero()},
		TimeCycle:    pgtype.Text{String: timer.TimeCycle, Valid: timer.TimeCycle != ""},
		TimeDuration: pgtype.Text{String: timer.TimeDuration.String(), Valid: !timer.TimeDuration.IsZero()},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	return nil
}

func triggerTimerStartEvent(ctx Context, task *TaskEntity, timer engine.Timer) error {
	eventDefinition, err := ctx.EventDefinitions().Select(task.ElementId.Int32)
	if err != nil {
		return err
	}

	if eventDefinition.IsSuspended {
		return nil
	}

	process, err := ctx.ProcessCache().GetOrCacheById(ctx, task.ProcessId.Int32)
	if err != nil {
		return err
	}

	var processInstance *ProcessInstanceEntity
	if task.ProcessInstanceId.Valid {
		selectedProcessInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
		if err != nil {
			return err
		}

		if selectedProcessInstance.EndedAt.Valid {
			return nil
		}

		processInstance = selectedProcessInstance
	} else {
		processInstance = &ProcessInstanceEntity{
			Partition: task.Partition,

			ProcessId: process.Id,

			BpmnProcessId: process.BpmnProcessId,
			CreatedAt:     ctx.Time(),
			CreatedBy:     ctx.Options().EngineId,
			StartedAt:     pgtype.Timestamp{Time: ctx.Time(), Valid: true},
			State:         engine.InstanceStarted,
			Version:       process.Version,
		}

		if err := ctx.ProcessInstances().Insert(processInstance); err != nil {
			return err
		}

		// set ID of inserted process instance for a possible retry
		task.ProcessInstanceId = pgtype.Int4{Int32: processInstance.Id}

		if err := enqueueProcessInstance(ctx, processInstance); err != nil {
			return fmt.Errorf("failed to enqueue process instance: %v", err)
		}
	}

	scope := process.graph.createProcessScope(processInstance)

	execution, err := process.graph.createExecutionAt(&scope, eventDefinition.BpmnElementId)
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
		processInstance:  processInstance,
	}

	executions := []*ElementInstanceEntity{&scope, &execution}
	if err := ec.continueExecutions(ctx, executions); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", executions, err)
		}
	}

	event := EventEntity{
		Partition: execution.Partition,

		ElementInstanceId: execution.Id,

		CreatedAt:    ctx.Time(),
		CreatedBy:    ctx.Options().EngineId,
		Time:         eventDefinition.Time,
		TimeCycle:    eventDefinition.TimeCycle,
		TimeDuration: eventDefinition.TimeDuration,
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	if timer.TimeCycle == "" {
		return nil // one-time event
	}

	// insert task for next tick
	dueAt, err := evaluateTimer(timer, task.DueAt)
	if err != nil {
		// next tick cannot be determined
		return nil
	}

	triggerEventTask := TaskEntity{
		Partition: ctx.Date(),

		ElementId: task.ElementId,
		ProcessId: task.ProcessId,

		CreatedAt: ctx.Time(),
		CreatedBy: ctx.Options().EngineId,
		DueAt:     dueAt,
		Type:      engine.TaskTriggerEvent,

		Instance: TriggerEventTask{Timer: &timer},
	}

	if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
		return err
	}

	return nil
}
