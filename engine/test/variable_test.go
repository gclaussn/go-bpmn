package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestVariables(t *testing.T) {
	assert := assert.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	for i, e := range engines {
		// given
		process := mustCreateProcess(t, e, "task/service.bpmn", "serviceTest")
		piAssert := mustCreateProcessInstance(t, e, process)
		processInstance := piAssert.ProcessInstance()

		t.Run(engineTypes[i]+"set process variables", func(t *testing.T) {
			// when
			if err := e.SetProcessVariables(context.Background(), engine.SetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,

				Variables: []engine.VariableData{
					{Name: "a", Data: &engine.Data{
						Encoding: "encoding-pa",
						Value:    "value-pa",
					}},
					{Name: "b", Data: &engine.Data{
						Encoding:    "encoding-pb",
						IsEncrypted: true,
						Value:       "value-pb",
					}},
					{Name: "c", Data: &engine.Data{
						Encoding: "encoding-pc",
						Value:    "value-pc",
					}},
					{Name: "d", Data: nil},
				},
				WorkerId: testWorkerId,
			}); err != nil {
				t.Fatalf("failed to set process variables: %v", err)
			}
		})

		// given
		piAssert.IsWaitingAt("serviceTask")
		elementInstance := piAssert.ElementInstance()

		t.Run(engineTypes[i]+"set element variables", func(t *testing.T) {
			// when
			if err := e.SetElementVariables(context.Background(), engine.SetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,

				Variables: []engine.VariableData{
					{Name: "a", Data: &engine.Data{
						Encoding: "encoding-ea",
						Value:    "value-ea",
					}},
					{Name: "b", Data: &engine.Data{
						Encoding:    "encoding-eb",
						IsEncrypted: true,
						Value:       "value-eb",
					}},
					{Name: "c", Data: &engine.Data{
						Encoding: "encoding-ec",
						Value:    "value-ec",
					}},
					{Name: "d", Data: nil},
				},
				WorkerId: "test",
			}); err != nil {
				t.Fatalf("failed to set element variables: %v", err)
			}
		})

		t.Run(engineTypes[i]+"set element variables returns error when element instance is ended", func(t *testing.T) {
			// given
			results, err := e.CreateQuery().QueryElementInstances(context.Background(), engine.ElementInstanceCriteria{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
				BpmnElementId:     "startEvent",
			})
			if err != nil {
				t.Fatalf("failed to query element instance: %v", err)
			}

			assert.Lenf(results, 1, "expected on element instance")

			// when
			err = e.SetElementVariables(context.Background(), engine.SetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: results[0].Id,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorConflict, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"set element variables returns error when element instance not exists", func(t *testing.T) {
			// when
			err := e.SetElementVariables(context.Background(), engine.SetElementVariablesCmd{
				Partition: elementInstance.Partition,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorNotFound, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"get process variables", func(t *testing.T) {
			// when
			variables, err := e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
			})
			if err != nil {
				t.Fatalf("failed to get process variables: %v", err)
			}

			// then
			assert.Len(variables, 3)

			assert.Equal(variables[0].Name, "a")
			assert.Equal(variables[0].Data.Encoding, "encoding-pa")
			assert.False(variables[0].Data.IsEncrypted)
			assert.Equal(variables[0].Data.Value, "value-pa")

			assert.Equal(variables[1].Name, "b")
			assert.Equal(variables[1].Data.Encoding, "encoding-pb")
			assert.True(variables[1].Data.IsEncrypted)
			assert.Equal(variables[1].Data.Value, "value-pb")

			assert.Equal(variables[2].Name, "c")
			assert.Equal(variables[2].Data.Encoding, "encoding-pc")
			assert.False(variables[2].Data.IsEncrypted)
			assert.Equal(variables[2].Data.Value, "value-pc")
		})

		t.Run(engineTypes[i]+"get process variables by names", func(t *testing.T) {
			// when
			variables, err := e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,

				Names: []string{"a", "c", "d"},
			})
			if err != nil {
				t.Fatalf("failed to get process variables: %v", err)
			}

			// then
			assert.Len(variables, 2)

			assert.Equal("a", variables[0].Name)
			assert.Equal("c", variables[1].Name)
		})

		t.Run(engineTypes[i]+"get process variables returns error when process instance not exists", func(t *testing.T) {
			// when
			_, err := e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
				Partition: processInstance.Partition,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorNotFound, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"get element variables", func(t *testing.T) {
			// when
			variables, err := e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,
			})
			if err != nil {
				t.Fatalf("failed to get element variables: %v", err)
			}

			// then
			assert.Len(variables, 3)

			assert.Equal(variables[0].Name, "a")
			assert.Equal(variables[0].Data.Encoding, "encoding-ea")
			assert.False(variables[0].Data.IsEncrypted)
			assert.Equal(variables[0].Data.Value, "value-ea")

			assert.Equal(variables[1].Name, "b")
			assert.Equal(variables[1].Data.Encoding, "encoding-eb")
			assert.True(variables[1].Data.IsEncrypted)
			assert.Equal(variables[1].Data.Value, "value-eb")

			assert.Equal(variables[2].Name, "c")
			assert.Equal(variables[2].Data.Encoding, "encoding-ec")
			assert.False(variables[2].Data.IsEncrypted)
			assert.Equal(variables[2].Data.Value, "value-ec")
		})

		t.Run(engineTypes[i]+"get element variables by names", func(t *testing.T) {
			// when
			variables, err := e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,

				Names: []string{"a", "c", "d"},
			})
			if err != nil {
				t.Fatalf("failed to get element variables: %v", err)
			}

			// then
			assert.Len(variables, 2)

			assert.Equal("a", variables[0].Name)
			assert.Equal("c", variables[1].Name)
		})

		t.Run(engineTypes[i]+"get element variables returns error when element instance not exists", func(t *testing.T) {
			// when
			_, err := e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
				Partition: elementInstance.Partition,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorNotFound, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"update and delete process variables", func(t *testing.T) {
			// when
			if err := e.SetProcessVariables(context.Background(), engine.SetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,

				Variables: []engine.VariableData{
					{Name: "a", Data: &engine.Data{
						Encoding:    "encoding-pa*",
						IsEncrypted: true,
						Value:       "value-pa*",
					}},
					{Name: "b", Data: nil},
				},
				WorkerId: "test",
			}); err != nil {
				t.Fatalf("failed to set process variables: %v", err)
			}

			// then
			variables, err := e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
			})
			if err != nil {
				t.Fatalf("failed to get process variables: %v", err)
			}

			assert.Len(variables, 2)

			assert.Equal(variables[0].Name, "a")
			assert.Equal(variables[0].Data.Encoding, "encoding-pa*")
			assert.True(variables[0].Data.IsEncrypted)
			assert.Equal(variables[0].Data.Value, "value-pa*")

			assert.Equal(variables[1].Name, "c")
		})

		t.Run(engineTypes[i]+"update and delete element variables", func(t *testing.T) {
			// when
			if err := e.SetElementVariables(context.Background(), engine.SetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,

				Variables: []engine.VariableData{
					{Name: "a", Data: &engine.Data{
						Encoding:    "encoding-ea*",
						IsEncrypted: true,
						Value:       "value-ea*",
					}},
					{Name: "c", Data: nil},
				},
				WorkerId: "test",
			}); err != nil {
				t.Fatalf("failed to set element variables: %v", err)
			}

			// then
			variables, err := e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,
			})
			if err != nil {
				t.Fatalf("failed to get element variables: %v", err)
			}

			assert.Len(variables, 2)

			assert.Equal(variables[0].Name, "a")
			assert.Equal(variables[0].Data.Encoding, "encoding-ea*")
			assert.True(variables[0].Data.IsEncrypted)
			assert.Equal(variables[0].Data.Value, "value-ea*")

			assert.Equal(variables[1].Name, "b")
		})

		// given
		piAssert.CompleteJob()
		piAssert.IsCompleted()

		t.Run(engineTypes[i]+"set process variables returns error when process instance is ended", func(t *testing.T) {
			// when
			err := e.SetProcessVariables(context.Background(), engine.SetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorConflict, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"set process variables returns error when process instance not exists", func(t *testing.T) {
			// when
			err := e.SetProcessVariables(context.Background(), engine.SetProcessVariablesCmd{
				Partition: processInstance.Partition,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorNotFound, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"set element variables returns error when process instance is ended", func(t *testing.T) {
			// when
			err := e.SetElementVariables(context.Background(), engine.SetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorConflict, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})
	}
}
