package pg

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// !keep in sync with engine/mem/variable_test.go
func TestParentElementVariables(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	now := time.Now().UTC().Truncate(time.Millisecond)

	// given
	p1 := &internal.ProcessInstanceEntity{
		Partition: now,

		ProcessId: 1,

		State: engine.InstanceStarted,
	}

	p2 := &internal.ProcessInstanceEntity{
		Partition: now,

		ProcessId: 2,

		State: engine.InstanceStarted,
	}

	mustInsertEntities(t, e, []any{p1, p2})

	e0 := &internal.ElementInstanceEntity{
		Partition: now,

		ElementId:         1,
		ProcessId:         p1.ProcessId,
		ProcessInstanceId: p1.Id,

		BpmnElementId:   "x",
		BpmnElementType: model.ElementProcess,
		State:           engine.InstanceStarted,
	}

	mustInsertEntities(t, e, []any{e0})

	e1 := &internal.ElementInstanceEntity{
		Partition: now,

		ParentId: pgtype.Int4{Int32: e0.Id, Valid: true},

		ElementId:         1,
		ProcessId:         p1.ProcessId,
		ProcessInstanceId: p1.Id,

		BpmnElementId:   "a",
		BpmnElementType: model.ElementCallActivity,
		State:           engine.InstanceStarted,
	}

	mustInsertEntities(t, e, []any{e1})

	e2 := &internal.ElementInstanceEntity{
		Partition: now,

		ParentId: pgtype.Int4{Int32: e1.Id, Valid: true},

		ElementId:         2,
		ProcessId:         p2.ProcessId,
		ProcessInstanceId: p2.Id,

		BpmnElementId:   "b",
		BpmnElementType: model.ElementProcess,
		State:           engine.InstanceStarted,
	}

	mustInsertEntities(t, e, []any{e2})

	e3 := &internal.ElementInstanceEntity{
		Partition: now,

		ParentId: pgtype.Int4{Int32: e2.Id, Valid: true},

		ElementId:         3,
		ProcessId:         p2.ProcessId,
		ProcessInstanceId: p2.Id,

		BpmnElementId:   "c",
		BpmnElementType: model.ElementSubProcess,
		State:           engine.InstanceStarted,
	}

	mustInsertEntities(t, e, []any{e3})

	e4 := &internal.ElementInstanceEntity{
		Partition: now,

		ParentId: pgtype.Int4{Int32: e3.Id, Valid: true},

		ElementId:         4,
		ProcessId:         p2.ProcessId,
		ProcessInstanceId: p2.Id,

		BpmnElementId:   "d",
		BpmnElementType: model.ElementServiceTask,
		State:           engine.InstanceStarted,
	}

	mustInsertEntities(t, e, []any{e4})

	// set variables at call activity of parent process
	if err := e.SetElementVariables(context.Background(), engine.SetElementVariablesCmd{
		Partition:         engine.Partition(now),
		ElementInstanceId: e1.Id,

		Variables: []engine.ElementVariable{
			{
				Name: "a",
				Data: &engine.Data{
					Encoding: "encoding-a",
					Value:    "value-a",
				},
			},
			{
				BpmnElementId: "a",
				Name:          "b",
				Data: &engine.Data{
					Encoding: "encoding-b",
					Value:    "value-b",
				},
			},
		},
		WorkerId: testWorkerId,
	}); err != nil {
		t.Fatalf("failed to set variables: %v", err)
	}

	// set variables at service task
	if err := e.SetElementVariables(context.Background(), engine.SetElementVariablesCmd{
		Partition:         engine.Partition(now),
		ElementInstanceId: e4.Id,

		Variables: []engine.ElementVariable{
			{
				Name: "a",
				Data: &engine.Data{
					Encoding: "encoding-a",
					Value:    "value-a",
				},
			},
			{ // duplicate
				BpmnElementId: "d",
				Name:          "a",
				Data: &engine.Data{
					Encoding: "encoding-a*",
					Value:    "value-a*",
				},
			},
			{
				BpmnElementId: "c",
				Name:          "a",
				Data: &engine.Data{
					Encoding: "encoding-a**",
					Value:    "value-a**",
				},
			},
			{
				BpmnElementId: "b",
				Name:          "b",
				Data: &engine.Data{
					Encoding: "encoding-b",
					Value:    "value-b",
				},
			},
			{ // duplicate
				BpmnElementId: "b",
				Name:          "b",
				Data: &engine.Data{
					Encoding: "encoding-b*",
					Value:    "value-b*",
				},
			},
			{
				BpmnElementId: "d",
				Name:          "c",
				Data: &engine.Data{
					Encoding: "encoding-c",
					Value:    "value-c",
				},
			},
		},
		WorkerId: testWorkerId,
	}); err != nil {
		t.Fatalf("failed to set variables: %v", err)
	}

	t.Run("get element variables", func(t *testing.T) {
		variables, err := e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
			Partition:         engine.Partition(e4.Partition),
			ElementInstanceId: e4.Id,
		})
		if err != nil {
			t.Fatalf("failed to get variables: %v", err)
		}

		require.Len(variables, 4)

		assert.Equal("c", variables[0].BpmnElementId)
		assert.Equal("a", variables[0].Name)
		assert.Equal("encoding-a**", variables[0].Data.Encoding)
		assert.Equal("value-a**", variables[0].Data.Value)

		assert.Equal("d", variables[1].BpmnElementId)
		assert.Equal("a", variables[1].Name)
		assert.Equal("encoding-a", variables[1].Data.Encoding)
		assert.Equal("value-a", variables[1].Data.Value)

		assert.Equal("b", variables[2].BpmnElementId)
		assert.Equal("b", variables[2].Name)
		assert.Equal("encoding-b", variables[2].Data.Encoding)
		assert.Equal("value-b", variables[2].Data.Value)

		assert.Equal("d", variables[3].BpmnElementId)
		assert.Equal("c", variables[3].Name)
		assert.Equal("encoding-c", variables[3].Data.Encoding)
		assert.Equal("value-c", variables[3].Data.Value)
	})

	t.Run("get element variables, but exclude parent variables", func(t *testing.T) {
		variables, err := e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
			Partition:         engine.Partition(e4.Partition),
			ElementInstanceId: e4.Id,

			ExcludeParentVariables: true,
		})
		if err != nil {
			t.Fatalf("failed to get variables: %v", err)
		}

		require.Len(variables, 2)

		assert.Equal("d", variables[0].BpmnElementId)
		assert.Equal("a", variables[0].Name)
		assert.Equal("encoding-a", variables[0].Data.Encoding)
		assert.Equal("value-a", variables[0].Data.Value)

		assert.Equal("d", variables[1].BpmnElementId)
		assert.Equal("c", variables[1].Name)
		assert.Equal("encoding-c", variables[1].Data.Encoding)
		assert.Equal("value-c", variables[1].Data.Value)
	})

	t.Run("delete and update parent element variables", func(t *testing.T) {
		if err := e.SetElementVariables(context.Background(), engine.SetElementVariablesCmd{
			Partition:         engine.Partition(now),
			ElementInstanceId: e4.Id,

			Variables: []engine.ElementVariable{
				{
					BpmnElementId: "c",
					Name:          "a",
					Data: &engine.Data{
						Encoding: "encoding-a***",
						Value:    "value-a***",
					},
				},
				{
					BpmnElementId: "b",
					Name:          "b",
				},
			},
			WorkerId: testWorkerId,
		}); err != nil {
			t.Fatalf("failed to set variables: %v", err)
		}

		variables, err := e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
			Partition:         engine.Partition(e4.Partition),
			ElementInstanceId: e4.Id,
		})
		if err != nil {
			t.Fatalf("failed to get variables: %v", err)
		}

		require.Len(variables, 3)

		assert.Equal("c", variables[0].BpmnElementId)
		assert.Equal("a", variables[0].Name)
		assert.Equal("encoding-a***", variables[0].Data.Encoding)
		assert.Equal("value-a***", variables[0].Data.Value)
	})

	t.Run("returns error when scope does not exist", func(t *testing.T) {
		err := e.SetElementVariables(context.Background(), engine.SetElementVariablesCmd{
			Partition:         engine.Partition(now),
			ElementInstanceId: e4.Id,

			Variables: []engine.ElementVariable{
				{
					BpmnElementId: "z",
					Name:          "a",
				},
				{
					BpmnElementId: "x",
					Name:          "b",
				},
				{
					BpmnElementId: "b",
					Name:          "b",
				},
			},
			WorkerId: testWorkerId,
		})

		require.IsTypef(engine.Error{}, err, "expected engine error")

		engineErr := err.(engine.Error)
		assert.Equal(engine.ErrorValidation, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.Contains(engineErr.Detail, "has no such scopes [x, z], but [b, c, d]")
	})
}
