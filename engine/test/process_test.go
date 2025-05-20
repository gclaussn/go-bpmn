package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestCreateProcess(t *testing.T) {
	assert := assert.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	bpmnXml := mustReadBpmnFile(t, "task/service.bpmn")

	// given
	cmd := engine.CreateProcessCmd{
		BpmnProcessId: "serviceTest",
		BpmnXml:       bpmnXml,
		Tags: map[string]string{
			"a": "b",
			"x": "y",
		},
		Version:  "1",
		WorkerId: testWorkerId,
	}

	for i, e := range engines {
		// when
		process, err := e.CreateProcess(cmd)

		t.Run(engineTypes[i]+"create and get BPMN XML", func(t *testing.T) {
			if err != nil {
				t.Fatalf("failed to create process: %v", err)
			}

			// then
			assert.Equal(engine.Process{
				Id: process.Id,

				BpmnProcessId: cmd.BpmnProcessId,
				CreatedAt:     process.CreatedAt,
				CreatedBy:     cmd.WorkerId,
				Parallelism:   0,
				Tags: map[string]string{
					"a": "b",
					"x": "y",
				},
				Version: cmd.Version,
			}, process)

			assert.NotEmpty(process.Id)
			assert.NotEmpty(process.CreatedAt)

			results, err := e.Query(engine.ProcessCriteria{Id: process.Id})
			if err != nil {
				t.Fatalf("failed to query process: %v", err)
			}

			assert.Lenf(results, 1, "expected on process")

			assert.Equal(engine.Process{
				Id: process.Id,

				BpmnProcessId: cmd.BpmnProcessId,
				CreatedAt:     process.CreatedAt,
				CreatedBy:     cmd.WorkerId,
				Parallelism:   0,
				Tags:          cmd.Tags,
				Version:       cmd.Version,
			}, results[0])

			// when
			bpmnXml, err := e.GetBpmnXml(engine.GetBpmnXmlCmd{ProcessId: process.Id})
			if err != nil {
				t.Fatalf("failed to query process: %v", err)
			}

			// then
			assert.Equal(cmd.BpmnXml, bpmnXml)
		})

		t.Run(engineTypes[i]+"returns existing process when created again", func(t *testing.T) {
			// when
			existingProcess, err := e.CreateProcess(cmd)
			if err != nil {
				t.Fatalf("failed to create process: %v", err)
			}

			// then
			assert.Equal(engine.Process{
				Id: process.Id,

				BpmnProcessId: cmd.BpmnProcessId,
				CreatedAt:     process.CreatedAt,
				CreatedBy:     cmd.WorkerId,
				Parallelism:   0,
				Tags:          cmd.Tags,
				Version:       cmd.Version,
			}, existingProcess)
		})

		t.Run(engineTypes[i]+"returns error when created again with a different BPMN XML", func(t *testing.T) {
			// when
			cmd.BpmnXml += " "
			_, err := e.CreateProcess(cmd)

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorConflict, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"returns error when BPMN XML cannot be parsed", func(t *testing.T) {
			// when
			_, err := e.CreateProcess(engine.CreateProcessCmd{
				BpmnProcessId: "",
				BpmnXml:       "",
				Version:       "1",
				WorkerId:      testWorkerId,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorProcessModel, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
			assert.Equal("XML is empty", engineErr.Detail)
		})

		t.Run(engineTypes[i]+"returns error when BPMN model has no process", func(t *testing.T) {
			// when
			_, err := e.CreateProcess(engine.CreateProcessCmd{
				BpmnProcessId: "notExisting",
				BpmnXml:       bpmnXml,
				Version:       "1",
				WorkerId:      testWorkerId,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorProcessModel, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
			assert.Contains(engineErr.Detail, "but [serviceTest]")
		})

		t.Run(engineTypes[i]+"returns error when BPMN process is invalid", func(t *testing.T) {
			// when
			_, err := e.CreateProcess(engine.CreateProcessCmd{
				BpmnProcessId: "processNotExecutableTest",
				BpmnXml:       mustReadBpmnFile(t, "invalid/process-not-executable.bpmn"),
				Version:       "1",
				WorkerId:      testWorkerId,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorProcessModel, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
			assert.Equal("BPMN process is not executable", engineErr.Detail)
		})
	}
}
