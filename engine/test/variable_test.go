package test

import (
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
			if err := e.SetProcessVariables(engine.SetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,

				Variables: map[string]*engine.Data{
					"a": {
						Encoding: "encoding-pa",
						Value:    "value-pa",
					},
					"b": {
						Encoding:    "encoding-pb",
						IsEncrypted: true,
						Value:       "value-pb",
					},
					"c": {
						Encoding: "encoding-pc",
						Value:    "value-pc",
					},
					"d": nil,
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
			if err := e.SetElementVariables(engine.SetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,

				Variables: map[string]*engine.Data{
					"a": {
						Encoding: "encoding-ea",
						Value:    "value-ea",
					},
					"b": {
						Encoding:    "encoding-eb",
						IsEncrypted: true,
						Value:       "value-eb",
					},
					"c": {
						Encoding: "encoding-ec",
						Value:    "value-ec",
					},
					"d": nil,
				},
				WorkerId: "test",
			}); err != nil {
				t.Fatalf("failed to set element variables: %v", err)
			}
		})

		t.Run(engineTypes[i]+"set element variables returns error when element instance is ended", func(t *testing.T) {
			// given
			results, err := e.Query(engine.ElementInstanceCriteria{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
				BpmnElementId:     "startEvent",
			})
			if err != nil {
				t.Fatalf("failed to query element instance: %v", err)
			}

			assert.Lenf(results, 1, "expected on element instance")

			// when
			err = e.SetElementVariables(engine.SetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: results[0].(engine.ElementInstance).Id,
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
			err := e.SetElementVariables(engine.SetElementVariablesCmd{
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
			variables, err := e.GetProcessVariables(engine.GetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
			})
			if err != nil {
				t.Fatalf("failed to get process variables: %v", err)
			}

			// then
			assert.Len(variables, 3)

			assert.Contains(variables, "a")
			assert.Contains(variables, "b")
			assert.Contains(variables, "c")

			assert.Equal(variables["a"].Encoding, "encoding-pa")
			assert.False(variables["a"].IsEncrypted)
			assert.Equal(variables["a"].Value, "value-pa")

			assert.Equal(variables["b"].Encoding, "encoding-pb")
			assert.True(variables["b"].IsEncrypted)
			assert.Equal(variables["b"].Value, "value-pb")

			assert.Equal(variables["c"].Encoding, "encoding-pc")
			assert.False(variables["c"].IsEncrypted)
			assert.Equal(variables["c"].Value, "value-pc")
		})

		t.Run(engineTypes[i]+"get process variables by names", func(t *testing.T) {
			// when
			variables, err := e.GetProcessVariables(engine.GetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,

				Names: []string{"a", "c", "d"},
			})
			if err != nil {
				t.Fatalf("failed to get process variables: %v", err)
			}

			// then
			assert.Len(variables, 2)

			assert.Contains(variables, "a")
			assert.Contains(variables, "c")
		})

		t.Run(engineTypes[i]+"get process variables returns error when process instance not exists", func(t *testing.T) {
			// when
			_, err := e.GetProcessVariables(engine.GetProcessVariablesCmd{
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
			variables, err := e.GetElementVariables(engine.GetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,
			})
			if err != nil {
				t.Fatalf("failed to get element variables: %v", err)
			}

			// then
			assert.Len(variables, 3)

			assert.Contains(variables, "a")
			assert.Contains(variables, "b")
			assert.Contains(variables, "c")

			assert.Equal(variables["a"].Encoding, "encoding-ea")
			assert.False(variables["a"].IsEncrypted)
			assert.Equal(variables["a"].Value, "value-ea")

			assert.Equal(variables["b"].Encoding, "encoding-eb")
			assert.True(variables["b"].IsEncrypted)
			assert.Equal(variables["b"].Value, "value-eb")

			assert.Equal(variables["c"].Encoding, "encoding-ec")
			assert.False(variables["c"].IsEncrypted)
			assert.Equal(variables["c"].Value, "value-ec")
		})

		t.Run(engineTypes[i]+"get element variables by names", func(t *testing.T) {
			// when
			variables, err := e.GetElementVariables(engine.GetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,

				Names: []string{"a", "c", "d"},
			})
			if err != nil {
				t.Fatalf("failed to get element variables: %v", err)
			}

			// then
			assert.Len(variables, 2)

			assert.Contains(variables, "a")
			assert.Contains(variables, "c")
		})

		t.Run(engineTypes[i]+"get element variables returns error when element instance not exists", func(t *testing.T) {
			// when
			_, err := e.GetElementVariables(engine.GetElementVariablesCmd{
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
			if err := e.SetProcessVariables(engine.SetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,

				Variables: map[string]*engine.Data{
					"a": {
						Encoding:    "encoding-pa*",
						IsEncrypted: true,
						Value:       "value-pa*",
					},
					"b": nil,
				},
				WorkerId: "test",
			}); err != nil {
				t.Fatalf("failed to set process variables: %v", err)
			}

			// then
			variables, err := e.GetProcessVariables(engine.GetProcessVariablesCmd{
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
			})
			if err != nil {
				t.Fatalf("failed to get process variables: %v", err)
			}

			assert.Len(variables, 2)

			assert.Contains(variables, "a")
			assert.Contains(variables, "c")

			assert.Equal(variables["a"].Encoding, "encoding-pa*")
			assert.True(variables["a"].IsEncrypted)
			assert.Equal(variables["a"].Value, "value-pa*")
		})

		t.Run(engineTypes[i]+"update and delete element variables", func(t *testing.T) {
			// when
			if err := e.SetElementVariables(engine.SetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,

				Variables: map[string]*engine.Data{
					"a": {
						Encoding:    "encoding-ea*",
						IsEncrypted: true,
						Value:       "value-ea*",
					},
					"c": nil,
				},
				WorkerId: "test",
			}); err != nil {
				t.Fatalf("failed to set element variables: %v", err)
			}

			// then
			variables, err := e.GetElementVariables(engine.GetElementVariablesCmd{
				Partition:         elementInstance.Partition,
				ElementInstanceId: elementInstance.Id,
			})
			if err != nil {
				t.Fatalf("failed to get element variables: %v", err)
			}

			assert.Len(variables, 2)

			assert.Contains(variables, "a")
			assert.Contains(variables, "b")

			assert.Equal(variables["a"].Encoding, "encoding-ea*")
			assert.True(variables["a"].IsEncrypted)
			assert.Equal(variables["a"].Value, "value-ea*")
		})

		// given
		piAssert.CompleteJob()
		piAssert.IsEnded()

		t.Run(engineTypes[i]+"set process variables returns error when process instance is ended", func(t *testing.T) {
			// when
			err := e.SetProcessVariables(engine.SetProcessVariablesCmd{
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
			err := e.SetProcessVariables(engine.SetProcessVariablesCmd{
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
			err := e.SetElementVariables(engine.SetElementVariablesCmd{
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
