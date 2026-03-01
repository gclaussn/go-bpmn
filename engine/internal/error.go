package internal

import (
	"fmt"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

// triggerErrorEndEvent continues the execution by either finding an appropriate error boundary event and starting it or behaving like a non end event.
func (ec *executionContext) triggerErrorEndEvent(ctx Context) error {
	scope := ec.executions[0]
	execution := ec.executions[1]

	boundaryEvents, err := ctx.ElementInstances().SelectBoundaryEvents(scope)
	if err != nil {
		return err
	}

	errorCode := execution.Context.String

	target := findErrorBoundaryEvent(boundaryEvents, errorCode)
	if target != nil {
		targetScope, err := ctx.ElementInstances().Select(target.Partition, target.ParentId.Int32)
		if err != nil {
			return err
		}

		event := EventEntity{
			Partition: target.Partition,

			ElementInstanceId: target.Id,

			CreatedAt: ctx.Time(),
			CreatedBy: ec.engineOrWorkerId,
			ErrorCode: pgtype.Text{String: errorCode, Valid: true},
		}

		if err := ctx.Events().Insert(&event); err != nil {
			return err
		}

		// overwrite executions to fulfill startBoundaryEvent contract
		ec.executions = []*ElementInstanceEntity{targetScope, target}

		_, err = ec.startBoundaryEvent(ctx, true)
		if err != nil {
			return err
		}

		// overwrite execution state to indicate which element instance has triggered the error
		for _, e := range ec.executions {
			if e.Id == execution.Id {
				e.State = engine.InstanceCompleted
			}
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

func findErrorBoundaryEvent(boundaryEvents []*ElementInstanceEntity, errorCode string) *ElementInstanceEntity {
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
			return boundaryEvent
		}
	}
	return target
}
