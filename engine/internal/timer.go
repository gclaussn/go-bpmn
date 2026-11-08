package internal

import (
	"errors"
	"fmt"
	"time"

	"github.com/adhocore/gronx"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

func SetTime(ctx Context, cmd engine.SetTimeCmd) (time.Time, time.Time, error) {
	old := ctx.Time()

	//lint:ignore S1016 cmd is not converted to engine.Timer
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
	if jobCompletion == nil || jobCompletion.Timer.IsZero() {
		job.Error = pgtype.Text{String: "expected a timer", Valid: true}
		return nil
	}

	dueAt, err := evaluateTimer(jobCompletion.Timer, ctx.Time())
	if err != nil {
		job.Error = pgtype.Text{String: fmt.Sprintf("failed to evaluate timer: %v", err), Valid: true}
		return nil
	}

	execution := ec.executions[1]

	if execution.PrevId.Valid { // boundary event or catch event next to event-based gateway
		prev, err := ctx.ElementInstances().Select(execution.Partition, execution.PrevId.Int32)
		if err != nil {
			return fmt.Errorf("failed to previous element instance: %v", err)
		}

		prev.ExecutionCount++
		ec.addExecution(prev)
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

			Instance: TriggerEventTask{Timer: timer},
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
	execution := ec.executions[1]

	if execution.PrevId.Valid {
		ec.completeEventBasedGateay(ctx)
	}

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
		CreatedBy:    ec.engineOrWorkerId,
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

		Instance: TriggerEventTask{Timer: timer},
	}

	if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
		return err
	}

	return nil
}

func mapTimerEventDefinitions(
	eventDefinitions []EventDefinitionEntity,
	causes []engine.ErrorCause,
	timerDefinitions []engine.TimerDefinition,
	timerEvents []*model.Element,
	processElement *model.Element,
) ([]EventDefinitionEntity, []engine.ErrorCause) {
	timers := make(map[string]engine.Timer, len(timerDefinitions))
	for _, timerDefinition := range timerDefinitions {
		var exists bool
		for _, timerEvent := range timerEvents {
			if timerEvent.Id == timerDefinition.BpmnElementId {
				exists = true
				break
			}
		}

		if exists {
			timers[timerDefinition.BpmnElementId] = timerDefinition.Timer
		} else {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(processElement),
				Type:    "timer_event",
				Detail:  fmt.Sprintf("process %s has no timer event %s", processElement.Id, timerDefinition.BpmnElementId),
			})
		}
	}

	for _, timerEvent := range timerEvents {
		timer, ok := timers[timerEvent.Id]
		if ok && timer.IsZero() {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(timerEvent),
				Type:    "timer_event",
				Detail:  "timer must specify a time, time cycle or time duration",
			})
		}

		if ok {
			continue
		}

		source := getEventDefinition(timerEvent).Timer

		if timerEvent.Type == model.ElementTimerStartEvent && source == nil {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(timerEvent),
				Type:    "timer_event",
				Detail:  fmt.Sprintf("no timer defined for element %s", timerEvent.Id),
			})
			continue
		}

		if source == nil {
			continue
		}

		// parse timer
		var parserErr error
		switch {
		case source.Time != "":
			t, err := time.Parse(time.RFC3339Nano, source.Time)
			if err != nil {
				parserErr = fmt.Errorf("failed to parse timeDate %s: %v", source.Time, err)
			} else {
				timer.Time = t
			}
		case source.TimeCycle != "":
			if !gronx.IsValid(source.TimeCycle) {
				parserErr = fmt.Errorf("failed to parse timeCycle %s: CRON is invalid", source.TimeCycle)
			} else {
				timer.TimeCycle = source.TimeCycle
			}
		case source.TimeDuration != "":
			d, err := engine.NewISO8601Duration(source.TimeDuration)
			if err != nil {
				parserErr = fmt.Errorf("failed to parse timeDuration %s: %v", source.TimeDuration, err)
			} else {
				timer.TimeDuration = d
			}
		default:
			parserErr = errors.New("timer event definition must specify a timeDate, timeCycle or timeDuration")
		}

		if parserErr != nil {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(timerEvent),
				Type:    "timer_event",
				Detail:  parserErr.Error(),
			})
			continue
		}

		timers[timerEvent.Id] = timer
	}

	for bpmnElementId, timer := range timers {
		eventDefinitions = append(eventDefinitions, EventDefinitionEntity{
			BpmnElementId: bpmnElementId,
			Time:          pgtype.Timestamp{Time: timer.Time, Valid: !timer.Time.IsZero()},
			TimeCycle:     pgtype.Text{String: timer.TimeCycle, Valid: timer.TimeCycle != ""},
			TimeDuration:  pgtype.Text{String: timer.TimeDuration.String(), Valid: !timer.TimeDuration.IsZero()},
		})
	}

	return eventDefinitions, causes
}
