package internal

import (
	"fmt"
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
			Detail: fmt.Sprintf("element type %s is not supported", bpmnElement.Type),
		}
	}
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
