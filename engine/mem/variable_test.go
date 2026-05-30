package mem

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

// !keep in sync with engine/pg/variable_test.go
func TestParentElementVariables(t *testing.T) {
	//assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	now := time.Now().UTC().Truncate(time.Millisecond)

	// given
	e1 := &internal.ElementInstanceEntity{
		Partition: now,

		ElementId:         1,
		ProcessId:         1,
		ProcessInstanceId: 1,

		BpmnElementId:   "a",
		BpmnElementType: model.ElementCallActivity,
		State:           engine.InstanceStarted,
	}

	mustInsertEntities(t, e, []any{e1})

	e2 := &internal.ElementInstanceEntity{
		Partition: now,

		ParentId: pgtype.Int4{Int32: e1.Id, Valid: true},

		ElementId:         2,
		ProcessId:         2,
		ProcessInstanceId: 2,

		BpmnElementId:   "b",
		BpmnElementType: model.ElementProcess,
		State:           engine.InstanceStarted,
	}

	mustInsertEntities(t, e, []any{e2})

	e3 := &internal.ElementInstanceEntity{
		Partition: now,

		ParentId: pgtype.Int4{Int32: e2.Id, Valid: true},

		ElementId:         3,
		ProcessId:         2,
		ProcessInstanceId: 2,

		BpmnElementId:   "c",
		BpmnElementType: model.ElementSubProcess,
		State:           engine.InstanceStarted,
	}

	mustInsertEntities(t, e, []any{e3})

	e4 := &internal.ElementInstanceEntity{
		Partition: now,

		ParentId: pgtype.Int4{Int32: e3.Id, Valid: true},

		ElementId:         4,
		ProcessId:         2,
		ProcessInstanceId: 2,

		BpmnElementId:   "d",
		BpmnElementType: model.ElementServiceTask,
		State:           engine.InstanceStarted,
	}

	mustInsertEntities(t, e, []any{e4})

	t.Run("set and get element variables", func(t *testing.T) {
		e.SetElementVariables(context.Background(), engine.SetElementVariablesCmd{
			Partition:         engine.Partition(now),
			ElementInstanceId: e4.Id,

			Variables: []engine.ElementVariable{
				{
					Name: "a",
					Data: &engine.Data{
						Encoding: "encoding-ea",
						Value:    "value-ea",
					},
				},
				{ // duplicate
					BpmnElementId: "d",
					Name:          "a",
					Data: &engine.Data{
						Encoding: "encoding-ea*",
						Value:    "value-ea*",
					},
				},
				{
					BpmnElementId: "c",
					Name:          "a",
					Data: &engine.Data{
						Encoding: "encoding-ea**",
						Value:    "value-ea**",
					},
				},
				{
					BpmnElementId: "b",
					Name:          "b",
					Data: &engine.Data{
						Encoding: "encoding-eb",
						Value:    "value-eb",
					},
				},
			},
			WorkerId: testWorkerId,
		})
	})
}
