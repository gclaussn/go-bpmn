package test

import (
	"context"
	"fmt"
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
		Tags: []engine.Tag{
			{Name: "a", Value: "b"},
			{Name: "x", Value: "y"},
		},
		Version:  t.Name(),
		WorkerId: testWorkerId,
	}

	t.Run("create and get BPMN XML", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// when
				process, err := e.CreateProcess(context.Background(), cmd)
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
					Tags: []engine.Tag{
						{Name: "a", Value: "b"},
						{Name: "x", Value: "y"},
					},
					Version: cmd.Version,
				}, process)

				assert.NotEmpty(process.Id)
				assert.NotEmpty(process.CreatedAt)

				results, err := e.CreateQuery().QueryProcesses(context.Background(), engine.ProcessCriteria{Id: process.Id})
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
				bpmnXml, err := e.GetBpmnXml(context.Background(), engine.GetBpmnXmlCmd{ProcessId: process.Id})
				if err != nil {
					t.Fatalf("failed to query process: %v", err)
				}

				// then
				assert.Equal(cmd.BpmnXml, bpmnXml)

				t.Run("returns existing process when created again", func(t *testing.T) {
					// when
					existingProcess, err := e.CreateProcess(context.Background(), cmd)
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
			})
		}
	})

	t.Run("returns error when created again with a different BPMN XML", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// when
				cmd.BpmnXml += " "
				_, err := e.CreateProcess(context.Background(), cmd)

				// then
				assert.IsTypef(engine.Error{}, err, "expected engine error")

				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorConflict, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
			})
		}
	})

	t.Run("returns error when BPMN XML cannot be parsed", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// when
				_, err := e.CreateProcess(context.Background(), engine.CreateProcessCmd{
					BpmnProcessId: "",
					BpmnXml:       "",
					Version:       t.Name(),
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
		}
	})

	t.Run("returns error when BPMN model has no process", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// when
				_, err := e.CreateProcess(context.Background(), engine.CreateProcessCmd{
					BpmnProcessId: "notExisting",
					BpmnXml:       bpmnXml,
					Version:       t.Name(),
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
		}
	})

	t.Run("returns error when BPMN process is invalid", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// when
				_, err := e.CreateProcess(context.Background(), engine.CreateProcessCmd{
					BpmnProcessId: "processNotExecutableTest",
					BpmnXml:       mustReadBpmnFile(t, "invalid/process-not-executable.bpmn"),
					Version:       t.Name(),
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
	})
}

func TestCreateProcessWithErrorCode(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	// given
	bpmnXml := mustReadBpmnFile(t, "event/error-boundary-event.bpmn")

	t.Run("returns error when BPMN element is not an error boundary event", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd := engine.CreateProcessCmd{
					BpmnProcessId: "errorBoundaryEventTest",
					BpmnXml:       bpmnXml,
					Errors: []engine.ErrorDefinition{
						{BpmnElementId: "errorBoundaryEvent", ErrorCode: "TEST_CODE"},
						{BpmnElementId: "endEvent", ErrorCode: "TEST_CODE"},
					},
					Version:  t.Name(),
					WorkerId: testWorkerId,
				}

				// when
				_, err := e.CreateProcess(context.Background(), cmd)
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
					BpmnProcessId: "errorBoundaryEventTest",
					BpmnXml:       bpmnXml,
					Errors: []engine.ErrorDefinition{
						{BpmnElementId: "errorBoundaryEvent", ErrorCode: "TEST_CODE"},
						{BpmnElementId: "not-existing", ErrorCode: "TEST_CODE"},
					},
					Version:  t.Name(),
					WorkerId: testWorkerId,
				}

				// when
				_, err := e.CreateProcess(context.Background(), cmd)
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
				cmd := engine.CreateProcessCmd{
					BpmnProcessId: "errorBoundaryEventTest",
					BpmnXml:       bpmnXml,
					Errors: []engine.ErrorDefinition{
						{BpmnElementId: "errorBoundaryEvent", ErrorCode: "TEST_CODE"},
					},
					Version:  t.Name(),
					WorkerId: testWorkerId,
				}

				// when
				process, err := e.CreateProcess(context.Background(), cmd)
				require.NoError(err, "failed to create process")

				// then
				elements, err := e.CreateQuery().QueryElements(context.Background(), engine.ElementCriteria{
					ProcessId: process.Id,

					BpmnElementId: "errorBoundaryEvent",
				})
				require.NoError(err, "failed to query elements")
				require.Len(elements, 1)

				assert.Equal(&engine.EventDefinition{
					ErrorCode: "TEST_CODE",
				}, elements[0].EventDefinition)
			})
		}
	})
}

func TestCreateProcessWithMessage(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	// given
	bpmnXml1 := mustReadBpmnFile(t, "event/message-start.bpmn")
	bpmnXml2 := mustReadBpmnFile(t, "event/message-start.v2.bpmn")

	t.Run("returns error when message name is missing", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd := engine.CreateProcessCmd{
					BpmnProcessId: "messageStartTest",
					BpmnXml:       bpmnXml1,
					Version:       "1",
					WorkerId:      testWorkerId,
				}

				// when
				_, err := e.CreateProcess(context.Background(), cmd)
				require.IsType(engine.Error{}, err)

				// then
				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorValidation, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
			})
		}
	})

	t.Run("returns error when BPMN element is not a message start event", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd := engine.CreateProcessCmd{
					BpmnProcessId: "messageStartTest",
					BpmnXml:       bpmnXml1,
					Messages: []engine.MessageDefinition{
						{BpmnElementId: "messageStartEvent", MessageName: "start-message"},
						{BpmnElementId: "endEvent", MessageName: "error"},
					},
					Version:  "2",
					WorkerId: testWorkerId,
				}

				// when
				_, err := e.CreateProcess(context.Background(), cmd)
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
					BpmnProcessId: "messageStartTest",
					BpmnXml:       bpmnXml1,
					Messages: []engine.MessageDefinition{
						{BpmnElementId: "messageStartEvent", MessageName: "start-message"},
						{BpmnElementId: "not-existing", MessageName: "error"},
					},
					Version:  "3",
					WorkerId: testWorkerId,
				}

				// when
				_, err := e.CreateProcess(context.Background(), cmd)
				require.IsType(engine.Error{}, err)

				// then
				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorValidation, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
			})
		}
	})

	t.Run("returns error when start message name is not unique within process", func(t *testing.T) {
		bpmnXml := mustReadBpmnFile(t, "event/message-start-multiple.bpmn")

		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd := engine.CreateProcessCmd{
					BpmnProcessId: "messageStartMultipleTest",
					BpmnXml:       bpmnXml,
					Messages: []engine.MessageDefinition{
						{BpmnElementId: "messageStartEventA", MessageName: t.Name()},
						{BpmnElementId: "messageStartEventB", MessageName: t.Name()},
					},
					Version:  t.Name(),
					WorkerId: testWorkerId,
				}

				// when
				_, err := e.CreateProcess(context.Background(), cmd)
				require.IsType(engine.Error{}, err)

				// then
				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorValidation, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
				assert.Len(engineErr.Causes, 1)

				cause := engineErr.Causes[0]
				assert.Regexp("/messageStartMultipleTest/(messageStartEventA|messageStartEventB)", cause.Pointer)
				assert.Equal("message_event", cause.Type)
				assert.Contains(cause.Detail, "must be unique")
			})
		}
	})

	t.Run("returns error when start message name is not unique within not suspended event definitions", func(t *testing.T) {
		for i, e := range engines {
			startDefinitionBpmnXml := mustReadBpmnFile(t, "event/message-start-definition.bpmn")

			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd1 := engine.CreateProcessCmd{
					BpmnProcessId: "messageStartDefinitionTest",
					BpmnXml:       startDefinitionBpmnXml,
					Version:       t.Name(),
					WorkerId:      testWorkerId,
				}

				process1, err := e.CreateProcess(context.Background(), cmd1)
				if err != nil {
					t.Fatalf("failed to create process: %v", err)
				}

				cmd2 := engine.CreateProcessCmd{
					BpmnProcessId: "messageStartTest",
					BpmnXml:       bpmnXml1,
					Messages: []engine.MessageDefinition{
						{BpmnElementId: "messageStartEvent", MessageName: "startMessageName"},
					},
					Version:  t.Name(),
					WorkerId: testWorkerId,
				}

				// when
				_, err = e.CreateProcess(context.Background(), cmd2)
				require.IsType(engine.Error{}, err)

				// then
				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorValidation, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
				assert.Len(engineErr.Causes, 1)

				cause := engineErr.Causes[0]
				assert.Equal("/messageStartTest/messageStartEvent", cause.Pointer)
				assert.Equal("message_event", cause.Type)
				assert.Contains(cause.Detail, fmt.Sprintf("must be unique - already defined in process %s:%s", process1.BpmnProcessId, process1.Version))
			})
		}
	})

	t.Run("create", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd1 := engine.CreateProcessCmd{
					BpmnProcessId: "messageStartTest",
					BpmnXml:       bpmnXml1,
					Messages: []engine.MessageDefinition{
						{BpmnElementId: "messageStartEvent", MessageName: "start-message"},
					},
					Version:  "4",
					WorkerId: testWorkerId,
				}

				// when
				process1, err := e.CreateProcess(context.Background(), cmd1)
				require.NoError(err, "failed to create process")

				// then
				elements, err := e.CreateQuery().QueryElements(context.Background(), engine.ElementCriteria{
					ProcessId: process1.Id,

					BpmnElementId: "messageStartEvent",
				})
				require.NoError(err, "failed to query elements")
				require.Len(elements, 1)

				assert.Equal(&engine.EventDefinition{
					IsSuspended: false,
					MessageName: "start-message",
				}, elements[0].EventDefinition)

				// given
				cmd2 := engine.CreateProcessCmd{
					BpmnProcessId: "messageStartTest",
					BpmnXml:       bpmnXml2,
					Messages: []engine.MessageDefinition{
						{BpmnElementId: "messageStartEvent", MessageName: "start-message*"},
					},
					Version:  "5",
					WorkerId: testWorkerId,
				}

				// when
				process2, err := e.CreateProcess(context.Background(), cmd2)
				require.NoError(err, "failed to create process")

				// then
				elements, err = e.CreateQuery().QueryElements(context.Background(), engine.ElementCriteria{
					ProcessId: process1.Id,

					BpmnElementId: "messageStartEvent",
				})
				require.NoError(err, "failed to query elements")
				require.Len(elements, 1)

				assert.Equal(&engine.EventDefinition{
					IsSuspended: true,
					MessageName: "start-message",
				}, elements[0].EventDefinition)

				elements, err = e.CreateQuery().QueryElements(context.Background(), engine.ElementCriteria{
					ProcessId: process2.Id,

					BpmnElementId: "messageStartEvent",
				})
				require.NoError(err, "failed to query elements")
				require.Len(elements, 1)

				assert.Equal(&engine.EventDefinition{
					IsSuspended: false,
					MessageName: "start-message*",
				}, elements[0].EventDefinition)
			})
		}
	})
}

func TestCreateProcessWithSignal(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	t.Run("returns error when start signal name is not unique within process", func(t *testing.T) {
		bpmnXml := mustReadBpmnFile(t, "event/signal-start-multiple.bpmn")

		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				cmd := engine.CreateProcessCmd{
					BpmnProcessId: "signalStartMultipleTest",
					BpmnXml:       bpmnXml,
					Signals: []engine.SignalDefinition{
						{BpmnElementId: "signalStartEventA", SignalName: t.Name()},
						{BpmnElementId: "signalStartEventB", SignalName: t.Name()},
					},
					Version:  t.Name(),
					WorkerId: testWorkerId,
				}

				// when
				_, err := e.CreateProcess(context.Background(), cmd)
				require.IsType(engine.Error{}, err)

				// then
				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorValidation, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
				assert.Len(engineErr.Causes, 1)

				cause := engineErr.Causes[0]
				assert.Regexp("/signalStartMultipleTest/(signalStartEventA|signalStartEventB)", cause.Pointer)
				assert.Equal("signal_event", cause.Type)
				assert.Contains(cause.Detail, "must be unique")
			})
		}
	})
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
				_, err := e.CreateProcess(context.Background(), cmd)
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
					Timers: []engine.TimerDefinition{
						{BpmnElementId: "timerStartEvent", Timer: &engine.Timer{}},
						{BpmnElementId: "endEvent", Timer: &engine.Timer{}},
					},
					Version:  "2",
					WorkerId: testWorkerId,
				}

				// when
				_, err := e.CreateProcess(context.Background(), cmd)
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
					Timers: []engine.TimerDefinition{
						{BpmnElementId: "timerStartEvent", Timer: &engine.Timer{}},
						{BpmnElementId: "not-existing", Timer: &engine.Timer{}},
					},
					Version:  "3",
					WorkerId: testWorkerId,
				}

				// when
				_, err := e.CreateProcess(context.Background(), cmd)
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
					Timers: []engine.TimerDefinition{
						{BpmnElementId: "timerStartEvent", Timer: &engine.Timer{TimeCycle: "0 * * * *"}},
					},
					Version:  "4",
					WorkerId: testWorkerId,
				}

				// when
				process1, err := e.CreateProcess(context.Background(), cmd1)
				require.NoError(err, "failed to create process")

				// then
				tasks, err := e.CreateQuery().QueryTasks(context.Background(), engine.TaskCriteria{ProcessId: process1.Id, Type: engine.TaskTriggerEvent})
				require.NoError(err, "failed to query tasks")
				require.Len(tasks, 1)

				assert.Equal(engine.Task{
					Partition: tasks[0].Partition,
					Id:        tasks[0].Id,

					ElementId:         tasks[0].ElementId,
					ElementInstanceId: int32(0),
					ProcessId:         process1.Id,
					ProcessInstanceId: int32(0),

					BpmnElementId:  "timerStartEvent",
					CreatedAt:      tasks[0].CreatedAt,
					CreatedBy:      testWorkerId,
					DueAt:          tasks[0].DueAt,
					SerializedTask: tasks[0].SerializedTask,
					State:          engine.WorkCreated,
					Type:           engine.TaskTriggerEvent,
				}, tasks[0])

				// given
				cmd2 := engine.CreateProcessCmd{
					BpmnProcessId: "timerStartTest",
					BpmnXml:       bpmnXml2,
					Timers: []engine.TimerDefinition{
						{BpmnElementId: "timerStartEvent1", Timer: &engine.Timer{TimeCycle: "0 * * * *"}},
						{BpmnElementId: "timerStartEvent2", Timer: &engine.Timer{TimeCycle: "0 * * * *"}},
					},
					Version:  "5",
					WorkerId: testWorkerId,
				}

				// when
				process2, err := e.CreateProcess(context.Background(), cmd2)
				require.NoError(err, "failed to create process")

				err = e.SetTime(context.Background(), engine.SetTimeCmd{
					Time: time.Now().Add(time.Hour),
				})
				require.NoError(err, "failed to set time")

				// then
				tasks, err = e.CreateQuery().QueryTasks(context.Background(), engine.TaskCriteria{ProcessId: process2.Id, Type: engine.TaskTriggerEvent})
				require.NoError(err, "failed to query tasks")
				require.Len(tasks, 2)

				// when
				completedTasks, failedTasks, err := e.ExecuteTasks(context.Background(), engine.ExecuteTasksCmd{Type: engine.TaskTriggerEvent, Limit: 3})
				require.NoError(err, "failed to execute tasks")

				// then
				assert.Len(completedTasks, 3)
				assert.Len(failedTasks, 0)

				processInstances, err := e.CreateQuery().QueryProcessInstances(context.Background(), engine.ProcessInstanceCriteria{})
				require.NoError(err, "failed to query process instances")
				require.Len(processInstances, 2)

				assert.Equal(processInstances[0].ProcessId, process2.Id)
				assert.Equal(processInstances[1].ProcessId, process2.Id)

				tasks, err = e.CreateQuery().QueryTasks(context.Background(), engine.TaskCriteria{ProcessId: process2.Id, Type: engine.TaskTriggerEvent})
				require.NoError(err, "failed to query tasks")
				require.Len(tasks, 4, "expected two new tasks for the next time cycle")
			})
		}
	})
}
