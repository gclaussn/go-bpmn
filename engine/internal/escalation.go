package internal

import (
	"fmt"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

// TODO
func (ec *executionContext) triggerEscalationEndEvent(ctx Context) error {
	scope := ec.executions[0]
	execution := ec.executions[1]

	boundaryEvents, err := ctx.ElementInstances().SelectBoundaryEvents(scope)
	if err != nil {
		return err
	}

	escalationCode := execution.Context.String

	// find escalation boundary event
	var target *ElementInstanceEntity
	for _, boundaryEvent := range boundaryEvents {
		if boundaryEvent.BpmnElementType != model.ElementEscalationBoundaryEvent {
			continue
		}

		node, err := ec.process.graph.node(boundaryEvent.BpmnElementId)
		if err != nil {
			return err
		}

		interrupting := node.bpmnElement.Model.(model.BoundaryEvent).CancelActivity

		switch execution.BpmnElementType {
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
			continue
		}
		if boundaryEvent.Context.String == escalationCode {
			target = boundaryEvent
			break
		}
	}

	if target != nil {
		targetScope, err := ctx.ElementInstances().Select(target.Partition, target.ParentId.Int32)
		if err != nil {
			return err
		}

		event := EventEntity{
			Partition: target.Partition,

			ElementInstanceId: target.Id,

			CreatedAt:      ctx.Time(),
			CreatedBy:      ec.engineOrWorkerId,
			EscalationCode: pgtype.Text{String: escalationCode, Valid: true},
		}

		if err := ctx.Events().Insert(&event); err != nil {
			return err
		}

		// overwrite executions to fulfill startBoundaryEvent contract
		ec.executions = []*ElementInstanceEntity{targetScope, target}

		node, _ := ec.process.graph.node(target.BpmnElementId)
		interrupting := node.bpmnElement.Model.(model.BoundaryEvent).CancelActivity

		if _, err := ec.startBoundaryEvent(ctx, interrupting); err != nil {
			return err
		}

		if interrupting {
			// overwrite execution state to indicate which element instance has triggered the escalation
			for _, e := range ec.executions {
				if e.Id == execution.Id {
					e.State = engine.InstanceCompleted
				}
			}
		} else {
			ec.addExecution(scope)
			ec.addExecution(execution)
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

func findEscalationBoundaryEvent(boundaryEvents []*ElementInstanceEntity, escalationCode string) *ElementInstanceEntity {
	var target *ElementInstanceEntity
	for _, boundaryEvent := range boundaryEvents {
		if boundaryEvent.BpmnElementType != model.ElementEscalationBoundaryEvent {
			continue
		}

		if boundaryEvent.Context.String == "" && target == nil {
			target = boundaryEvent
			continue
		}
		if boundaryEvent.Context.String == escalationCode {
			return boundaryEvent
		}
	}
	return target
}
