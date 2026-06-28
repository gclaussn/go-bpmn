package internal

import (
	"fmt"
	"slices"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

type EventEntity struct {
	Partition time.Time

	ElementInstanceId int32

	CreatedAt             time.Time
	CreatedBy             string
	ErrorCode             pgtype.Text
	EscalationCode        pgtype.Text
	MessageCorrelationKey pgtype.Text
	MessageName           pgtype.Text
	SignalName            pgtype.Text
	Time                  pgtype.Timestamp
	TimeCycle             pgtype.Text
	TimeDuration          pgtype.Text
}

type EventRepository interface {
	Insert(*EventEntity) error
}

type EventDefinitionEntity struct {
	ElementId int32

	ProcessId int32

	BpmnElementId   string
	BpmnElementType model.ElementType
	BpmnProcessId   string
	ErrorCode       pgtype.Text
	EscalationCode  pgtype.Text
	IsSuspended     bool
	MessageName     pgtype.Text
	SignalName      pgtype.Text
	Time            pgtype.Timestamp
	TimeCycle       pgtype.Text
	TimeDuration    pgtype.Text
	Version         string
}

type EventDefinitionRepository interface {
	InsertBatch([]*EventDefinitionEntity) error
	Select(elementId int32) (*EventDefinitionEntity, error)

	SelectByProcessId(processId int32) ([]*EventDefinitionEntity, error)

	SelectByBpmnProcessId(bpmnProcessId string) ([]*EventDefinitionEntity, error)

	// SelectByMessageName selects a not suspended event definition for the message name.
	//
	// If no such event definition exists, nil is returned.
	SelectByMessageName(messageName string) (*EventDefinitionEntity, error)

	// SelectBySignalName selects all not suspended event definitions for the signal name.
	SelectBySignalName(signalName string) ([]*EventDefinitionEntity, error)

	UpdateBatch([]*EventDefinitionEntity) error
}

// TriggerEventTask triggers start or catch events.
//
// In case of a start event, a new process instance is created.
// In case of a boundary or catch event, an execution is continued.
type TriggerEventTask struct {
	ErrorCode      string        `json:",omitempty"`
	EscalationCode string        `json:",omitempty"`
	MessageId      int64         `json:",omitempty"`
	SignalId       int64         `json:",omitempty"`
	Timer          *engine.Timer `json:",omitempty"`
}

func (t TriggerEventTask) Execute(ctx Context, task *TaskEntity) error {
	process, err := ctx.ProcessCache().GetOrCacheById(ctx, task.ProcessId.Int32)
	if err != nil {
		return err
	}

	node, err := process.graph.node(task.BpmnElementId.String)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to find BPMN element",
			Detail: err.Error(),
		}
	}

	bpmnElement := node.bpmnElement

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
			task.State = engine.WorkCanceled
			break
		}

		execution, err := ctx.ElementInstances().Select(task.Partition, task.ElementInstanceId.Int32)
		if err != nil {
			return err
		}
		if execution.EndedAt.Valid {
			task.State = engine.WorkCanceled
			break
		}

		scope, err := ctx.ElementInstances().Select(execution.Partition, execution.ParentId.Int32)
		if err != nil {
			return err
		}

		execution.parent = scope

		ec = &executionContext{
			engineOrWorkerId: ctx.Options().EngineId,
			executions:       []*ElementInstanceEntity{scope, execution},
			process:          process,
			processInstance:  processInstance,
		}
	}

	switch {
	case task.State != engine.WorkCanceled:
		break // trigger event, if work not canceled
	case t.MessageId != 0:
		// expire message, if work canceled
		message, err := ctx.Messages().Select(t.MessageId)
		if err != nil {
			return err
		}

		message.ExpiresAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		return ctx.Messages().Update(message)
	case t.SignalId != 0:
		// reduce active subscriber count, if work canceled
		signal, err := ctx.Signals().Select(t.SignalId)
		if err != nil {
			return err
		}

		signal.ActiveSubscriberCount--
		return ctx.Signals().Update(signal)
	default:
		return nil
	}

	var interrupting bool
	switch bpmnElement.Type {
	case
		model.ElementEscalationBoundaryEvent,
		model.ElementMessageBoundaryEvent,
		model.ElementSignalBoundaryEvent,
		model.ElementTimerBoundaryEvent:
		interrupting = bpmnElement.Model.(model.BoundaryEvent).CancelActivity
	}

	switch bpmnElement.Type {
	case model.ElementErrorBoundaryEvent:
		return ec.triggerErrorBoundaryEvent(ctx, t.ErrorCode)
	case model.ElementErrorEndEvent:
		return ec.triggerErrorEndEvent(ctx)
	case model.ElementEscalationBoundaryEvent:
		return ec.triggerEscalationBoundaryEvent(ctx, t.EscalationCode, interrupting)
	case model.ElementEscalationEndEvent, model.ElementEscalationThrowEvent:
		return ec.triggerEscalationThrowEvent(ctx)
	case model.ElementMessageBoundaryEvent:
		return ec.triggerMessageBoundaryEvent(ctx, t.MessageId, interrupting)
	case model.ElementMessageCatchEvent:
		return ec.triggerMessageCatchEvent(ctx, t.MessageId)
	case model.ElementMessageStartEvent:
		expireMessage := t.Timer != nil
		return ec.triggerMessageStartEvent(ctx, task, bpmnElement, t.MessageId, expireMessage)
	case model.ElementSignalBoundaryEvent:
		return ec.triggerSignalBoundaryEvent(ctx, t.SignalId, interrupting)
	case model.ElementSignalCatchEvent:
		return ec.triggerSignalCatchEvent(ctx, t.SignalId)
	case model.ElementSignalEndEvent, model.ElementSignalThrowEvent:
		return ec.triggerSignalThrowEvent(ctx)
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
			Detail: fmt.Sprintf("BPMN element type %s is not supported", bpmnElement.Type),
		}
	}
}

// startBoundaryEvent starts a boundary event.
// The method expects the scope of the boundary event as first and the boundary event as second execution.
//
// If the boundary event is non-interrupting, a new boundary event execution is attached and returned.
//
// If the boundary event is interrupting, all other boundary events as well as the execution, the boundary is attached to, are terminated.
func (ec *executionContext) startBoundaryEvent(ctx Context, interrupting bool) (*ElementInstanceEntity, error) {
	boundaryEventScope, boundaryEvent := ec.executions[0], ec.executions[1]

	// start boundary event
	boundaryEvent.State = engine.InstanceStarted

	if !interrupting {
		// attach new boundary event execution
		newBoundaryEvent, err := ec.process.graph.createExecutionAt(boundaryEventScope, boundaryEvent.BpmnElementId)
		if err != nil {
			return nil, err
		}

		newBoundaryEvent.ParentId = boundaryEvent.ParentId
		newBoundaryEvent.PrevElementId = boundaryEvent.PrevElementId
		newBoundaryEvent.PrevId = boundaryEvent.PrevId

		newBoundaryEvent.Context = boundaryEvent.Context
		newBoundaryEvent.State = engine.InstanceCreated

		ec.addExecution(&newBoundaryEvent)

		return &newBoundaryEvent, nil
	}

	attachedTo, err := ctx.ElementInstances().Select(boundaryEvent.Partition, boundaryEvent.PrevId.Int32)
	if err != nil {
		return nil, err
	}

	// terminate all other attached boundary events
	boundaryEvents, err := ctx.ElementInstances().SelectByPrevId(attachedTo.Partition, attachedTo.Id)
	if err != nil {
		return nil, err
	}

	for _, other := range boundaryEvents {
		if other.Id == boundaryEvent.Id {
			continue
		}

		other.State = engine.InstanceTerminated
		ec.addExecution(other)
	}

	// terminate attachedTo
	attachedTo.State = engine.InstanceTerminated
	attachedTo.parent = boundaryEventScope
	ec.addExecution(attachedTo)

	i := len(ec.executions) - 1
	for i < len(ec.executions) {
		execution := ec.executions[i]

		i++

		// if execution is a call activity, terminate descending process instances recursively
		if execution.BpmnElementType == model.ElementCallActivity {
			children, err := ctx.ElementInstances().SelectActiveChildren(execution)
			if err != nil {
				return nil, err
			}

			if len(children) == 0 {
				continue // process scope already ended
			}

			processScope := children[0]

			terminateProcessInstanceTask := TaskEntity{
				Partition: processScope.Partition,

				ProcessId:         pgtype.Int4{Int32: processScope.ProcessId, Valid: true},
				ProcessInstanceId: pgtype.Int4{Int32: processScope.ProcessInstanceId, Valid: true},

				CreatedAt: ctx.Time(),
				CreatedBy: ec.engineOrWorkerId,
				DueAt:     ctx.Time(),
				State:     engine.WorkCreated,
				Type:      engine.TaskTerminateProcessInstance,

				Instance: TerminateProcessInstanceTask{},
			}

			if err := ctx.Tasks().Insert(&terminateProcessInstanceTask); err != nil {
				return nil, err
			}
		}

		if execution.ExecutionCount <= 0 {
			continue
		}

		// if attachedTo is a scope (e.g. sub-process), terminate descending executions recursively
		children, err := ctx.ElementInstances().SelectActiveChildren(execution)
		if err != nil {
			return nil, err
		}

		for _, child := range children {
			child.State = engine.InstanceTerminated
			child.parent = execution
			ec.addExecution(child)
		}
	}

	// cancel message and signal subscriptions
	var (
		messageSubscriptionIds []int32
		signalSubscriptionIds  []int32
	)
	for _, execution := range ec.executions {
		if execution.State != engine.InstanceTerminated {
			continue
		}

		switch execution.BpmnElementType {
		case model.ElementMessageBoundaryEvent, model.ElementMessageCatchEvent:
			messageSubscriptionIds = append(messageSubscriptionIds, execution.Id)
		case model.ElementSignalBoundaryEvent, model.ElementSignalCatchEvent:
			signalSubscriptionIds = append(signalSubscriptionIds, execution.Id)
		}
	}

	if len(messageSubscriptionIds) != 0 {
		messageSubscriptions, err := ctx.MessageSubscriptions().SelectByProcessInstance(ec.processInstance)
		if err != nil {
			return nil, err
		}

		for _, messageSubscription := range messageSubscriptions {
			if !slices.Contains(messageSubscriptionIds, messageSubscription.ElementInstanceId) {
				continue
			}

			if err := ctx.MessageSubscriptions().Delete(messageSubscription); err != nil {
				return nil, err
			}
		}
	}

	if len(signalSubscriptionIds) != 0 {
		signalSubscriptions, err := ctx.SignalSubscriptions().SelectByProcessInstance(ec.processInstance)
		if err != nil {
			return nil, err
		}

		for _, signalSubscription := range signalSubscriptions {
			if !slices.Contains(signalSubscriptionIds, signalSubscription.ElementInstanceId) {
				continue
			}

			if err := ctx.SignalSubscriptions().Delete(signalSubscription); err != nil {
				return nil, err
			}
		}
	}

	// reverse execution order so that scopes are executed after their children
	// this guarantees that the continuation will decrement the execution count of a terminated scope to 0
	// otherwise a terminated scope might be skipped, leaving it's parent with a wrong execution count
	slices.SortFunc(ec.executions, func(a *ElementInstanceEntity, b *ElementInstanceEntity) int {
		return int(b.Id - a.Id)
	})

	return nil, nil
}

func suspendEventDefinitions(ctx Context, bpmnProcessId string) error {
	eventDefinitions, err := ctx.EventDefinitions().SelectByBpmnProcessId(bpmnProcessId)
	if err != nil {
		return err
	}

	var suspended []*EventDefinitionEntity
	for _, eventDefinition := range eventDefinitions {
		if eventDefinition.IsSuspended {
			continue
		}

		switch eventDefinition.BpmnElementType {
		case model.ElementMessageStartEvent, model.ElementSignalStartEvent, model.ElementTimerStartEvent:
			eventDefinition.IsSuspended = true
		default:
			continue
		}

		suspended = append(suspended, eventDefinition)
	}

	return ctx.EventDefinitions().UpdateBatch(suspended)
}
