package internal

import (
	"fmt"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5/pgtype"
)

func (ec *executionContext) triggerTimerBoundaryEvent(ctx Context, timer engine.Timer, interrupting bool) error {
	execution := ec.executions[0]

	scope, err := ctx.ElementInstances().Select(execution.Partition, execution.ParentId.Int32)
	if err != nil {
		return err
	}

	ec.addExecution(scope)

	var newExecution *ElementInstanceEntity

	// start boundary event
	execution.State = engine.InstanceStarted

	if interrupting {
		// terminate attached to and all other boundary events
		attachedTo, err := ctx.ElementInstances().Select(execution.Partition, execution.PrevId.Int32)
		if err != nil {
			return err
		}

		attachedTo.State = engine.InstanceTerminated
		ec.addExecution(attachedTo)

		boundaryEvents, err := ctx.ElementInstances().SelectBoundaryEvents(attachedTo)
		if err != nil {
			return err
		}

		for _, boundaryEvent := range boundaryEvents {
			if boundaryEvent.Id == execution.Id {
				continue
			}

			boundaryEvent.State = engine.InstanceTerminated
			ec.addExecution(boundaryEvent)
		}
	} else if timer.Time.IsZero() {
		// attach new boundary event, if timer is cycle or duration
		attached, err := ec.process.graph.createExecutionAt(scope, execution.BpmnElementId)
		if err != nil {
			return err
		}

		attached.ParentId = execution.ParentId
		attached.PrevElementId = execution.PrevElementId
		attached.PrevId = execution.PrevId

		attached.Context = execution.Context
		attached.State = engine.InstanceCreated

		newExecution = &attached
		ec.addExecution(newExecution)
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
		Time:         pgtype.Timestamp{Time: timer.Time, Valid: !timer.Time.IsZero()},
		TimeCycle:    pgtype.Text{String: timer.TimeCycle, Valid: timer.TimeCycle != ""},
		TimeDuration: pgtype.Text{String: timer.TimeDuration.String(), Valid: !timer.TimeDuration.IsZero()},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	return nil
}

func (ec *executionContext) triggerTimerCatchEvent(ctx Context, timer engine.Timer) error {
	execution := ec.executions[0]

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
		Time:         pgtype.Timestamp{Time: timer.Time, Valid: !timer.Time.IsZero()},
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
		Type:          engine.TaskTriggerEvent,

		Instance: TriggerEventTask{Timer: &timer},
	}

	if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
		return err
	}

	return nil
}
