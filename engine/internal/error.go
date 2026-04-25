package internal

import (
	"fmt"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

func (ec *executionContext) executeError(ctx Context, job *JobEntity, errorCode string) (bool, error) {
	execution := ec.executions[1]

	boundaryEvent, err := findErrorBoundaryEvent(ctx, execution, errorCode)
	if err != nil {
		return false, err
	}

	if boundaryEvent == nil {
		job.Error = pgtype.Text{String: fmt.Sprintf("failed to find boundary event for error code %s", errorCode), Valid: true}
		return false, nil
	}

	if boundaryEvent.ProcessInstanceId != execution.ProcessInstanceId {
		triggerEventTask := TaskEntity{
			Partition: boundaryEvent.Partition,

			ElementId:         pgtype.Int4{Int32: boundaryEvent.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: boundaryEvent.Id, Valid: true},
			ProcessId:         pgtype.Int4{Int32: boundaryEvent.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: boundaryEvent.ProcessInstanceId, Valid: true},

			BpmnElementId: pgtype.Text{String: boundaryEvent.BpmnElementId, Valid: true},
			CreatedAt:     ctx.Time(),
			CreatedBy:     ec.engineOrWorkerId,
			DueAt:         ctx.Time(),
			State:         engine.WorkCreated,
			Type:          engine.TaskTriggerEvent,

			Instance: TriggerEventTask{ErrorCode: errorCode},
		}

		if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
			return false, err
		}

		return false, terminateProcessInstance(ctx, ec.processInstance)
	}

	scope := ec.executions[0]

	// overwrite executions to fulfill startBoundaryEvent contract
	ec.executions = []*ElementInstanceEntity{scope, boundaryEvent}

	_, err = ec.startBoundaryEvent(ctx, true) // always interrupting
	if err != nil {
		return false, err
	}

	event := EventEntity{
		Partition: boundaryEvent.Partition,

		ElementInstanceId: boundaryEvent.Id,

		CreatedAt: ctx.Time(),
		CreatedBy: ec.engineOrWorkerId,
		ErrorCode: pgtype.Text{String: errorCode, Valid: true},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return false, err
	}

	return true, nil
}

func (ec *executionContext) setErrorCode(ctx Context, job *JobEntity, jobCompletion *engine.JobCompletion) error {
	execution := ec.executions[1]

	var context string
	if jobCompletion != nil {
		context = jobCompletion.ErrorCode
	}

	switch execution.BpmnElementType {
	case model.ElementErrorBoundaryEvent:
		attachedTo, err := ctx.ElementInstances().Select(execution.Partition, execution.PrevId.Int32)
		if err != nil {
			return fmt.Errorf("failed to select attached to element instance: %v", err)
		}

		attachedTo.ExecutionCount++
		ec.addExecution(attachedTo)
	case model.ElementErrorEndEvent:
		if context == "" {
			job.Error = pgtype.Text{String: "expected an error code", Valid: true}
			return nil
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
			DueAt:         ctx.Time(),
			State:         engine.WorkCreated,
			Type:          engine.TaskTriggerEvent,

			Instance: TriggerEventTask{},
		}

		if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
			return err
		}
	}

	execution.Context = pgtype.Text{String: context, Valid: true}
	return nil
}

// triggerErrorBoundaryEvent triggers the execution of an error boundary event -
// only applied for error boundary events that are attached to a call activity.
func (ec *executionContext) triggerErrorBoundaryEvent(ctx Context, errorCode string) error {
	boundaryEvent := ec.executions[1]

	_, err := ec.startBoundaryEvent(ctx, true) // always interrupting
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

	event := EventEntity{
		Partition: boundaryEvent.Partition,

		ElementInstanceId: boundaryEvent.Id,

		CreatedAt: ctx.Time(),
		CreatedBy: ec.engineOrWorkerId,
		ErrorCode: pgtype.Text{String: errorCode, Valid: true},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	return nil
}

// triggerErrorEndEvent triggers the execution of an error end event.
//
// If no appropriate error boundary event is found, the error end event behaves like a none end event.
//
// If an appropriate error boundary event is found, the execution continues at the error boundary event and the scope is terminated.
// When the scope is a call activity within the parent process instance, the child process instance is terminated.
func (ec *executionContext) triggerErrorEndEvent(ctx Context) error {
	scope, execution := ec.executions[0], ec.executions[1]

	errorCode := execution.Context.String

	boundaryEvent, err := findErrorBoundaryEvent(ctx, scope, errorCode)
	if err != nil {
		return err
	}

	if boundaryEvent != nil && boundaryEvent.ProcessInstanceId != execution.ProcessInstanceId {
		triggerEventTask := TaskEntity{
			Partition: boundaryEvent.Partition,

			ElementId:         pgtype.Int4{Int32: boundaryEvent.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: boundaryEvent.Id, Valid: true},
			ProcessId:         pgtype.Int4{Int32: boundaryEvent.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: boundaryEvent.ProcessInstanceId, Valid: true},

			BpmnElementId: pgtype.Text{String: boundaryEvent.BpmnElementId, Valid: true},
			CreatedAt:     ctx.Time(),
			CreatedBy:     ec.engineOrWorkerId,
			DueAt:         ctx.Time(),
			State:         engine.WorkCreated,
			Type:          engine.TaskTriggerEvent,

			Instance: TriggerEventTask{ErrorCode: errorCode},
		}

		if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
			return err
		}

		// mark error end event as completed
		execution.State = engine.InstanceCompleted

		if err := ctx.ElementInstances().Update(execution); err != nil {
			return err
		}

		return terminateProcessInstance(ctx, ec.processInstance)
	}

	if boundaryEvent != nil {
		boundaryEventScope, err := ctx.ElementInstances().Select(boundaryEvent.Partition, boundaryEvent.ParentId.Int32)
		if err != nil {
			return err
		}

		// overwrite executions to fulfill startBoundaryEvent contract
		ec.executions = []*ElementInstanceEntity{boundaryEventScope, boundaryEvent}

		_, err = ec.startBoundaryEvent(ctx, true) // always interrupting
		if err != nil {
			return err
		}

		// mark error end event as completed
		for _, e := range ec.executions {
			if e.Id == execution.Id {
				e.State = engine.InstanceCompleted
			}
		}

		event := EventEntity{
			Partition: boundaryEvent.Partition,

			ElementInstanceId: boundaryEvent.Id,

			CreatedAt: ctx.Time(),
			CreatedBy: ec.engineOrWorkerId,
			ErrorCode: pgtype.Text{String: errorCode, Valid: true},
		}

		if err := ctx.Events().Insert(&event); err != nil {
			return err
		}
	}

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
		}
	}

	return nil
}

// findErrorBoundaryEvent tries to find an error boundary event at the given scope.
//
// If no appropriate boundary event is found, the parent scopes are recursively tested.
// When a call activity scope is reached, the recursion stops.
func findErrorBoundaryEvent(ctx Context, scope *ElementInstanceEntity, errorCode string) (*ElementInstanceEntity, error) {
	switch scope.BpmnElementType {
	case model.ElementProcess:
		if !scope.ParentId.Valid {
			return nil, nil
		}
	default:
		boundaryEvents, err := ctx.ElementInstances().SelectBoundaryEvents(scope.Partition, scope.Id)
		if err != nil {
			return nil, err
		}

		var target *ElementInstanceEntity
		for _, boundaryEvent := range boundaryEvents {
			if boundaryEvent.BpmnElementType != model.ElementErrorBoundaryEvent {
				continue
			}
			if boundaryEvent.Context.String == "" && target == nil {
				target = boundaryEvent
				continue
			}
			if boundaryEvent.Context.String == errorCode {
				target = boundaryEvent
				break
			}
		}

		if target != nil {
			return target, nil
		}
	}

	if scope.BpmnElementType == model.ElementCallActivity {
		return nil, nil // stop recursion
	}

	parentScope, err := ctx.ElementInstances().Select(scope.Partition, scope.ParentId.Int32)
	if err != nil {
		return nil, err
	}

	// !recursion
	return findErrorBoundaryEvent(ctx, parentScope, errorCode)
}
