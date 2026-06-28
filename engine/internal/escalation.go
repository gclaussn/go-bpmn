package internal

import (
	"fmt"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

func (ec *executionContext) escalateJob(ctx Context, job *JobEntity, escalationCode string, retryTimer engine.ISO8601Duration) (bool, error) {
	execution := ec.executions[1]

	boundaryEvent, interrupting, err := findEscalationBoundaryEvent(ctx, ec.process.graph, execution, execution.BpmnElementType, escalationCode)
	if err != nil {
		return false, err
	}
	if boundaryEvent == nil {
		return false, engine.Error{
			Type:   engine.ErrorExecution,
			Title:  "failed to escalate job",
			Detail: fmt.Sprintf("failed to find boundary event for escalation code %s", escalationCode),
		}
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

			Instance: TriggerEventTask{EscalationCode: escalationCode},
		}

		if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
			return false, err
		}

		if interrupting {
			return false, terminateProcessInstance(ctx, ec.processInstance)
		}

		// retry job
		retry := JobEntity{
			Partition: job.Partition,

			ElementId:         job.ElementId,
			ElementInstanceId: job.ElementInstanceId,
			ProcessId:         job.ProcessId,
			ProcessInstanceId: job.ProcessInstanceId,

			BpmnElementId:  job.BpmnElementId,
			CorrelationKey: job.CorrelationKey,
			CreatedAt:      ctx.Time(),
			CreatedBy:      ec.engineOrWorkerId,
			DueAt:          retryTimer.Calculate(ctx.Time()),
			RetryCount:     job.RetryCount,
			State:          engine.WorkCreated,
			Type:           job.Type,
		}

		if err := ctx.Jobs().Insert(&retry); err != nil {
			return false, err
		}

		return false, nil
	}

	scope := ec.executions[0]

	// overwrite executions to fulfill startBoundaryEvent contract
	ec.executions = []*ElementInstanceEntity{scope, boundaryEvent}

	newBoundaryEvent, err := ec.startBoundaryEvent(ctx, interrupting)
	if err != nil {
		return false, err
	}

	if newBoundaryEvent != nil {
		// retry job
		retry := JobEntity{
			Partition: job.Partition,

			ElementId:         job.ElementId,
			ElementInstanceId: job.ElementInstanceId,
			ProcessId:         job.ProcessId,
			ProcessInstanceId: job.ProcessInstanceId,

			BpmnElementId:  job.BpmnElementId,
			CorrelationKey: job.CorrelationKey,
			CreatedAt:      ctx.Time(),
			CreatedBy:      ec.engineOrWorkerId,
			DueAt:          retryTimer.Calculate(ctx.Time()),
			RetryCount:     job.RetryCount,
			State:          engine.WorkCreated,
			Type:           job.Type,
		}

		if err := ctx.Jobs().Insert(&retry); err != nil {
			return false, err
		}

		// overwrite executions to avoid completion of execution (e.g. service task)
		ec.executions = []*ElementInstanceEntity{scope, newBoundaryEvent, boundaryEvent}
	}

	event := EventEntity{
		Partition: boundaryEvent.Partition,

		ElementInstanceId: boundaryEvent.Id,

		CreatedAt:      ctx.Time(),
		CreatedBy:      ec.engineOrWorkerId,
		EscalationCode: pgtype.Text{String: escalationCode, Valid: true},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return false, err
	}

	return true, nil
}

func (ec *executionContext) escalateUserTask(ctx Context, userTask *UserTaskEntity, escalationCode string) (bool, error) {
	execution := ec.executions[1]

	boundaryEvent, interrupting, err := findEscalationBoundaryEvent(ctx, ec.process.graph, execution, execution.BpmnElementType, escalationCode)
	if err != nil {
		return false, err
	}
	if boundaryEvent == nil {
		return false, engine.Error{
			Type:   engine.ErrorExecution,
			Title:  "failed to escalate user task",
			Detail: fmt.Sprintf("failed to find boundary event for escalation code %s", escalationCode),
		}
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

			Instance: TriggerEventTask{EscalationCode: escalationCode},
		}

		if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
			return false, err
		}

		if interrupting {
			userTask.State = engine.UserTaskTerminated
			return false, terminateProcessInstance(ctx, ec.processInstance)
		}

		return false, nil
	}

	scope := ec.executions[0]

	// overwrite executions to fulfill startBoundaryEvent contract
	ec.executions = []*ElementInstanceEntity{scope, boundaryEvent}

	newBoundaryEvent, err := ec.startBoundaryEvent(ctx, interrupting)
	if err != nil {
		return false, err
	}

	if newBoundaryEvent != nil {
		// overwrite executions to avoid completion of execution
		ec.executions = []*ElementInstanceEntity{scope, newBoundaryEvent, boundaryEvent}
	} else {
		userTask.State = engine.UserTaskTerminated
	}

	event := EventEntity{
		Partition: boundaryEvent.Partition,

		ElementInstanceId: boundaryEvent.Id,

		CreatedAt:      ctx.Time(),
		CreatedBy:      ec.engineOrWorkerId,
		EscalationCode: pgtype.Text{String: escalationCode, Valid: true},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return false, err
	}

	return true, nil
}

func (ec *executionContext) setEscalationCode(ctx Context, job *JobEntity, jobCompletion *engine.JobCompletion) error {
	execution := ec.executions[1]

	var context string
	if jobCompletion != nil {
		context = jobCompletion.EscalationCode
	}

	switch execution.BpmnElementType {
	case model.ElementEscalationBoundaryEvent:
		attachedTo, err := ctx.ElementInstances().Select(execution.Partition, execution.PrevId.Int32)
		if err != nil {
			return fmt.Errorf("failed to select attached to element instance: %v", err)
		}

		attachedTo.ExecutionCount++
		ec.addExecution(attachedTo)
	case model.ElementEscalationEndEvent, model.ElementEscalationThrowEvent:
		if context == "" {
			job.Error = pgtype.Text{String: "expected an escalation code", Valid: true}
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

// triggerEscalationBoundaryEvent triggers the execution of an escalation boundary event -
// only applied for escalation boundary events that are attached to a call activity.
func (ec *executionContext) triggerEscalationBoundaryEvent(ctx Context, escalationCode string, interrupting bool) error {
	scope, boundaryEvent := ec.executions[0], ec.executions[1]

	newBoundaryEvent, err := ec.startBoundaryEvent(ctx, interrupting)
	if err != nil {
		return err
	}

	if newBoundaryEvent != nil {
		// overwrite executions to avoid completion of call activity
		ec.executions = []*ElementInstanceEntity{scope, newBoundaryEvent, boundaryEvent}
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

		CreatedAt:      ctx.Time(),
		CreatedBy:      ec.engineOrWorkerId,
		EscalationCode: pgtype.Text{String: escalationCode, Valid: true},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	return nil
}

// triggerEscalationThrowEvent triggers the execution of an escalation throw or end event.
//
// If no appropriate escalation boundary event is found, the escalation event behaves like a none throw event or like a none end event.
//
// If an appropriate escalation boundary event is found, the execution continues at the escalation boundary event.
// When the boundary event is interrupting the scope is terminated.
// Moreover when the scope is a call activity within the parent process instance, the child process instance is terminated.
//
// Please note: an escalation throw event can only trigger non-interrupting boundary events.
// Whereas an escalation end event can only trigger interrupting boundary events that will terminate the scope.
func (ec *executionContext) triggerEscalationThrowEvent(ctx Context) error {
	scope, execution := ec.executions[0], ec.executions[1]

	escalationCode := execution.Context.String

	boundaryEvent, interrupting, err := findEscalationBoundaryEvent(ctx, ec.process.graph, scope, execution.BpmnElementType, escalationCode)
	if err != nil {
		return err
	}

	switch {
	case boundaryEvent != nil && boundaryEvent.ProcessInstanceId != execution.ProcessInstanceId:
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

			Instance: TriggerEventTask{EscalationCode: escalationCode},
		}

		if err := ctx.Tasks().Insert(&triggerEventTask); err != nil {
			return err
		}

		if execution.BpmnElementType == model.ElementEscalationEndEvent {
			// mark escalation end event as completed
			execution.State = engine.InstanceCompleted

			if err := ctx.ElementInstances().Update(execution); err != nil {
				return err
			}

			return terminateProcessInstance(ctx, ec.processInstance)
		}
	case boundaryEvent != nil:
		boundaryEventScope, err := ctx.ElementInstances().Select(boundaryEvent.Partition, boundaryEvent.ParentId.Int32)
		if err != nil {
			return err
		}

		// overwrite executions to fulfill startBoundaryEvent contract
		ec.executions = []*ElementInstanceEntity{boundaryEventScope, boundaryEvent}

		if _, err := ec.startBoundaryEvent(ctx, interrupting); err != nil {
			return err
		}

		if interrupting {
			// mark escalation end event as completed
			for _, e := range ec.executions {
				if e.Id == execution.Id {
					e.State = engine.InstanceCompleted
				}
			}
		} else {
			ec.addExecution(scope)
			ec.addExecution(execution)
		}

		event := EventEntity{
			Partition: boundaryEvent.Partition,

			ElementInstanceId: boundaryEvent.Id,

			CreatedAt:      ctx.Time(),
			CreatedBy:      ec.engineOrWorkerId,
			EscalationCode: pgtype.Text{String: escalationCode, Valid: true},
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

// findEscalationBoundaryEvent tries to find an escalation boundary event at the given scope.
//
// If no appropriate boundary event is found, the parent scopes are recursively tested.
// When a call activity scope is reached, the recursion stops.
//
// sourceType is the BPMN element type of the execution that triggered the escalation. Possible types are:
//   - any element type that can have a job of type [engine.JobExecute] - e.g. service task or message end event
//   - escalation throw event
//   - escalation end event
//
// Depending on the source type, interrupting, non-interrupting or boundary events of both types can be found.
func findEscalationBoundaryEvent(
	ctx Context,
	graph *graph,
	scope *ElementInstanceEntity,
	sourceType model.ElementType,
	escalationCode string,
) (*ElementInstanceEntity, bool, error) {
	switch scope.BpmnElementType {
	case model.ElementProcess:
		if !scope.ParentId.Valid {
			return nil, false, nil
		}
	default:
		boundaryEvents, err := ctx.ElementInstances().SelectByPrevId(scope.Partition, scope.Id)
		if err != nil {
			return nil, false, err
		}

		var (
			target             *ElementInstanceEntity
			targetInterrupting bool
		)
		for _, boundaryEvent := range boundaryEvents {
			if boundaryEvent.BpmnElementType != model.ElementEscalationBoundaryEvent {
				continue
			}

			boundaryEventNode, err := graph.node(boundaryEvent.BpmnElementId)
			if err != nil {
				return nil, false, err
			}

			interrupting := boundaryEventNode.bpmnElement.Model.(model.BoundaryEvent).CancelActivity

			switch sourceType {
			case model.ElementEscalationEndEvent:
				if !interrupting {
					continue // cannot trigger a non-interrupting boundary event
				}
			case model.ElementEscalationThrowEvent:
				if interrupting {
					continue // cannot trigger an interrupting boundary event
				}
			}

			if boundaryEvent.Context.String == "" && target == nil {
				target = boundaryEvent
				targetInterrupting = interrupting
				continue
			}
			if boundaryEvent.Context.String == escalationCode {
				target = boundaryEvent
				targetInterrupting = interrupting
				break
			}
		}

		if target != nil {
			return target, targetInterrupting, nil
		}
	}

	if scope.BpmnElementType == model.ElementCallActivity {
		return nil, false, nil // stop recursion
	}

	parentScope, err := ctx.ElementInstances().Select(scope.Partition, scope.ParentId.Int32)
	if err != nil {
		return nil, false, err
	}

	parentProcess, err := ctx.ProcessCache().GetOrCacheById(ctx, parentScope.ProcessId)
	if err != nil {
		return nil, false, err
	}

	// !recursion
	return findEscalationBoundaryEvent(ctx, parentProcess.graph, parentScope, sourceType, escalationCode)
}
