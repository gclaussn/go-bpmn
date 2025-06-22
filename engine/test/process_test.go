package test

import (
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.Contains(engineErr.Detail, "XML is empty")
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
			assert.Equal("BPMN process is invalid", engineErr.Detail)

			assert.Len(engineErr.Causes, 1)

			assert.Equal("/processNotExecutableTest", engineErr.Causes[0].Pointer)
			assert.NotEmpty(engineErr.Causes[0].Type)
			assert.Contains(engineErr.Causes[0].Detail, "not executable")
		})
	}
}

func TestCreateProcessWithTimer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	// given
	bpmnXml1 := mustReadBpmnFile(t, "event/timer-start.bpmn")
	bpmnXml2 := mustReadBpmnFile(t, "event/timer-start.v2.bpmn")

	t.Run("returns error when timer is missing", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd := engine.CreateProcessCmd{
					BpmnProcessId: "timerStartTest",
					BpmnXml:       bpmnXml1,
					Version:       "1",
					WorkerId:      testWorkerId,
				}

				// when
				_, err := e.CreateProcess(cmd)
				require.IsType(engine.Error{}, err)

				// then
				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorValidation, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
			})
		}
	})

	t.Run("returns error when BPMN element is not a timer start event", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd := engine.CreateProcessCmd{
					BpmnProcessId: "timerStartTest",
					BpmnXml:       bpmnXml1,
					Timers: map[string]*engine.Timer{
						"timerStartEvent": {},
						"endEvent":        {},
					},
					Version:  "2",
					WorkerId: testWorkerId,
				}

				// when
				_, err := e.CreateProcess(cmd)
				require.IsType(engine.Error{}, err)

				// then
				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorValidation, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
			})
		}
	})

	t.Run("returns error when BPMN element not exists", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd := engine.CreateProcessCmd{
					BpmnProcessId: "timerStartTest",
					BpmnXml:       bpmnXml1,
					Timers: map[string]*engine.Timer{
						"timerStartEvent": {},
						"not-existing":    {},
					},
					Version:  "3",
					WorkerId: testWorkerId,
				}

				// when
				_, err := e.CreateProcess(cmd)
				require.IsType(engine.Error{}, err)

				// then
				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorValidation, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
			})
		}
	})

	t.Run("create", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd1 := engine.CreateProcessCmd{
					BpmnProcessId: "timerStartTest",
					BpmnXml:       bpmnXml1,
					Timers: map[string]*engine.Timer{
						"timerStartEvent": {TimeCycle: "0 * * * *"},
					},
					Version:  "4",
					WorkerId: testWorkerId,
				}

				// when
				process1, err := e.CreateProcess(cmd1)
				require.NoError(err, "failed to create process")

				// then
				results, err := e.Query(engine.TaskCriteria{ProcessId: process1.Id, Type: engine.TaskTriggerTimerEvent})
				require.NoError(err, "failed to query tasks")
				require.Len(results, 1)

				task := results[0].(engine.Task)

				assert.Equal(engine.Task{
					Partition: task.Partition,
					Id:        task.Id,

					ElementId:         task.ElementId,
					ElementInstanceId: int32(0),
					ProcessId:         process1.Id,
					ProcessInstanceId: int32(0),

					CreatedAt: task.CreatedAt,
					CreatedBy: testWorkerId,
					DueAt:     task.DueAt,
					Type:      engine.TaskTriggerTimerEvent,
				}, task)

				// given
				cmd2 := engine.CreateProcessCmd{
					BpmnProcessId: "timerStartTest",
					BpmnXml:       bpmnXml2,
					Timers: map[string]*engine.Timer{
						"timerStartEvent1": {TimeCycle: "0 * * * *"},
						"timerStartEvent2": {TimeCycle: "0 * * * *"},
					},
					Version:  "5",
					WorkerId: testWorkerId,
				}

				// when
				process2, err := e.CreateProcess(cmd2)
				require.NoError(err, "failed to create process")

				err = e.SetTime(engine.SetTimeCmd{
					Time: time.Now().Add(time.Hour),
				})
				require.NoError(err, "failed to set time")

				// then
				results, err = e.Query(engine.TaskCriteria{ProcessId: process2.Id, Type: engine.TaskTriggerTimerEvent})
				require.NoError(err, "failed to query tasks")
				require.Len(results, 2)

				// when
				completedTasks, failedTasks, err := e.ExecuteTasks(engine.ExecuteTasksCmd{Type: engine.TaskTriggerTimerEvent, Limit: 3})
				require.NoError(err, "failed to execute tasks")

				// then
				assert.Len(completedTasks, 3)
				assert.Len(failedTasks, 0)

				results, err = e.Query(engine.ProcessInstanceCriteria{})
				require.NoError(err, "failed to query process instances")
				require.Len(results, 2)

				assert.Equal(results[0].(engine.ProcessInstance).ProcessId, process2.Id)
				assert.Equal(results[1].(engine.ProcessInstance).ProcessId, process2.Id)

				results, err = e.Query(engine.TaskCriteria{ProcessId: process2.Id, Type: engine.TaskTriggerTimerEvent})
				require.NoError(err, "failed to query tasks")
				require.Len(results, 4, "expected two new tasks for the next time cycle")
			})
		}
	})
}
