package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

func SetTime(ctx Context, cmd engine.SetTimeCmd) (time.Time, time.Time, error) {
	old := ctx.Time()

	new, err := evaluateTimer(engine.Timer{
		Time:         cmd.Time,
		TimeCycle:    cmd.TimeCycle,
		TimeDuration: cmd.TimeDuration,
	}, ctx.Time())
	if err != nil {
		return time.Time{}, time.Time{}, engine.Error{
			Type:   engine.ErrorValidation,
			Title:  "failed to set time",
			Detail: err.Error(),
		}
	}

	sub := new.Sub(old)
	if sub.Milliseconds() < 0 {
		return time.Time{}, time.Time{}, engine.Error{
			Type:  engine.ErrorConflict,
			Title: "failed to set time",
			Detail: fmt.Sprintf(
				"time %s is before engine time %s",
				new.Format(time.RFC3339),
				old.Format(time.RFC3339),
			),
		}
	}

	return new, old, nil
}

func (ec *executionContext) setTimer(ctx Context, job *JobEntity, jobCompletion *engine.JobCompletion) error {
	if jobCompletion == nil || jobCompletion.Timer == nil {
		job.Error = pgtype.Text{String: "expected a timer", Valid: true}
		return nil
	}

	dueAt, err := evaluateTimer(*jobCompletion.Timer, ctx.Time())
	if err != nil {
		job.Error = pgtype.Text{String: fmt.Sprintf("failed to evaluate timer: %v", err), Valid: true}
		return nil
	}

	execution := ec.executions[1]

	if execution.BpmnElementType == model.ElementTimerBoundaryEvent {
		attachedTo, err := ctx.ElementInstances().Select(execution.Partition, execution.PrevId.Int32)
		if err != nil {
			return fmt.Errorf("failed to select attached to element instance: %v", err)
		}

		attachedTo.ExecutionCount++
		ec.addExecution(attachedTo)
	}

	triggerEventTask := TaskEntity{
		Partition: execution.Partition,

		ElementId:         pgtype.Int4{Int32: execution.ElementId, Valid: true},
		ElementInstanceId: pgtype.Int4{Int32: execution.Id, Valid: true},
		ProcessId:         pgtype.Int4{Int32: execution.ProcessId, Valid: true},
		ProcessInstanceId: pgtype.Int4{Int32: execution.ProcessInstanceId, Valid: true},

		BpmnElementId: pgtype.Text{String: execution.BpmnElementId, Valid: true},
		CreatedAt:     ctx.Time(),
		CreatedBy:     ec.engineOrWorkerId,
		DueAt:         dueAt,
		State:         engine.WorkCreated,
		Type:          engine.TaskTriggerEvent,

		Instance: TriggerEventTask{Timer: jobCompletion.Timer},
	}

	if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
		return err
	}

	execution.Context = pgtype.Text{String: jobCompletion.Timer.String(), Valid: true}
	return nil
}

func (ec *executionContext) triggerTimerBoundaryEvent(ctx Context, timer engine.Timer, interrupting bool) error {
	execution := ec.executions[1]

	newExecution, err := ec.startBoundaryEvent(ctx, interrupting)
	if err != nil {
		return err
	}

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
		}
	}

	if newExecution != nil {
		dueAt, err := evaluateTimer(timer, ctx.Time())
		if err != nil {
			return fmt.Errorf("failed to evaluate timer: %v", err)
		}

		triggerEventTask := TaskEntity{
			Partition: newExecution.Partition,

			ElementId:         pgtype.Int4{Int32: newExecution.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: newExecution.Id, Valid: true},
			ProcessId:         pgtype.Int4{Int32: newExecution.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: newExecution.ProcessInstanceId, Valid: true},

			BpmnElementId: pgtype.Text{String: newExecution.BpmnElementId, Valid: true},
			CreatedAt:     ctx.Time(),
			CreatedBy:     ec.engineOrWorkerId,
			DueAt:         dueAt,
			State:         engine.WorkCreated,
			Type:          engine.TaskTriggerEvent,

			Instance: TriggerEventTask{Timer: &timer},
		}

		if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
			return err
		}
	}

	event := EventEntity{
		Partition: execution.Partition,

		ElementInstanceId: execution.Id,

		CreatedAt:    ctx.Time(),
		CreatedBy:    ctx.Options().EngineId,
		Time:         toPgTimestamp(timer.Time),
		TimeCycle:    pgtype.Text{String: timer.TimeCycle, Valid: timer.TimeCycle != ""},
		TimeDuration: pgtype.Text{String: timer.TimeDuration.String(), Valid: !timer.TimeDuration.IsZero()},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	return nil
}

func (ec *executionContext) triggerTimerCatchEvent(ctx Context, timer engine.Timer) error {
	execution := ec.executions[1]

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
		}
	}

	event := EventEntity{
		Partition: execution.Partition,

		ElementInstanceId: execution.Id,

		CreatedAt:    ctx.Time(),
		CreatedBy:    ctx.Options().EngineId,
		Time:         toPgTimestamp(timer.Time),
		TimeCycle:    pgtype.Text{String: timer.TimeCycle, Valid: timer.TimeCycle != ""},
		TimeDuration: pgtype.Text{String: timer.TimeDuration.String(), Valid: !timer.TimeDuration.IsZero()},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	return nil
}

func (ec *executionContext) triggerTimerStartEvent(ctx Context, task *TaskEntity, timer engine.Timer) error {
	eventDefinition, err := ctx.EventDefinitions().Select(task.ElementId.Int32)
	if err != nil {
		return err
	}

	if eventDefinition.IsSuspended {
		return nil
	}

	var processInstance *ProcessInstanceEntity
	if task.ProcessInstanceId.Valid {
		processInstance, err = ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
		if err != nil {
			return err
		}

		if processInstance.EndedAt.Valid {
			return nil
		}
	} else {
		processInstance = &ProcessInstanceEntity{
			Partition: task.Partition,

			ProcessId: ec.process.Id,

			BpmnProcessId: ec.process.BpmnProcessId,
			CreatedAt:     ctx.Time(),
			CreatedBy:     ctx.Options().EngineId,
			StartedAt:     pgtype.Timestamp{Time: ctx.Time(), Valid: true},
			State:         engine.InstanceStarted,
			Version:       ec.process.Version,
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

	scope := ec.process.graph.createProcessScope(processInstance)

	execution, err := ec.process.graph.createExecutionAt(&scope, eventDefinition.BpmnElementId)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create execution",
			Detail: err.Error(),
		}
	}

	ec.processInstance = processInstance
	ec.addExecution(&scope)
	ec.addExecution(&execution)

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
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

		BpmnElementId: task.BpmnElementId,
		CreatedAt:     ctx.Time(),
		CreatedBy:     ctx.Options().EngineId,
		DueAt:         dueAt,
		State:         engine.WorkCreated,
		Type:          engine.TaskTriggerEvent,

		Instance: TriggerEventTask{Timer: &timer},
	}

	if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
		return err
	}

	return nil
}
