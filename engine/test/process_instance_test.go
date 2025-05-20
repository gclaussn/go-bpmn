package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestCreateProcessInstance(t *testing.T) {
	assert := assert.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	t.Run("returns error when process not exists", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// when
				_, err := e.CreateProcessInstance(engine.CreateProcessInstanceCmd{})

				// then
				assert.IsTypef(engine.Error{}, err, "expected engine error")

				engineErr := err.(engine.Error)
				assert.Equal(engine.ErrorNotFound, engineErr.Type)
				assert.NotEmpty(engineErr.Title)
				assert.NotEmpty(engineErr.Detail)
			})
		}
	})

	t.Run("create", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				process := mustCreateProcess(t, e, "task/service.bpmn", "serviceTest")

				cmd := engine.CreateProcessInstanceCmd{
					BpmnProcessId:  process.BpmnProcessId,
					CorrelationKey: "test-key",
					Tags: map[string]string{
						"a": "b",
						"x": "y",
					},
					Version:  process.Version,
					WorkerId: testWorkerId,
				}

				// when
				processInstance, err := e.CreateProcessInstance(cmd)
				if err != nil {
					t.Fatalf("failed to create process instance: %v", err)
				}

				// then
				assert.Equal(engine.ProcessInstance{
					Partition: processInstance.Partition,
					Id:        processInstance.Id,

					ParentId: 0,
					RootId:   0,

					ProcessId: process.Id,

					BpmnProcessId:  process.BpmnProcessId,
					CorrelationKey: cmd.CorrelationKey,
					CreatedAt:      processInstance.CreatedAt,
					CreatedBy:      cmd.WorkerId,
					EndedAt:        nil,
					StartedAt:      &processInstance.CreatedAt,
					State:          engine.InstanceStarted,
					StateChangedBy: testWorkerId,
					Tags:           cmd.Tags,
					Version:        process.Version,
				}, processInstance)

				assert.NotEmpty(processInstance.Id)
				assert.NotEmpty(processInstance.CreatedAt)
				assert.NotEmpty(processInstance.StartedAt)

				assert.False(processInstance.HasParent())
				assert.False(processInstance.IsEnded())
				assert.True(processInstance.IsRoot())

				piAssert := engine.NewProcessInstanceAssert(t, e, processInstance)

				assert.Equal(engine.ProcessInstance{
					Partition: processInstance.Partition,
					Id:        processInstance.Id,

					ParentId: 0,
					RootId:   0,

					ProcessId: process.Id,

					BpmnProcessId:  process.BpmnProcessId,
					CorrelationKey: cmd.CorrelationKey,
					CreatedAt:      processInstance.CreatedAt,
					CreatedBy:      cmd.WorkerId,
					EndedAt:        nil,
					StartedAt:      &processInstance.CreatedAt,
					State:          engine.InstanceStarted,
					StateChangedBy: testWorkerId,
					Tags:           cmd.Tags,
					Version:        process.Version,
				}, piAssert.ProcessInstance())
			})
		}
	})

	t.Run("create with parallelism", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				bpmnXml := mustReadBpmnFile(t, "task/send.bpmn")

				process, err := e.CreateProcess(engine.CreateProcessCmd{
					BpmnProcessId: "sendTest",
					BpmnXml:       bpmnXml,
					Parallelism:   1,
					Version:       "1",
					WorkerId:      testWorkerId,
				})
				if err != nil {
					t.Fatalf("failed to create process: %v", err)
				}

				// when process instance 1 is created
				processInstance1, err := e.CreateProcessInstance(engine.CreateProcessInstanceCmd{
					BpmnProcessId: process.BpmnProcessId,
					Version:       process.Version,
					WorkerId:      testWorkerId,
				})
				if err != nil {
					t.Fatalf("failed to create process instance: %v", err)
				}

				// then
				assert.NotEmpty(processInstance1.StartedAt)
				assert.Equal(engine.InstanceStarted, processInstance1.State)

				// when process instance 2 is created
				processInstance2, err := e.CreateProcessInstance(engine.CreateProcessInstanceCmd{
					BpmnProcessId: process.BpmnProcessId,
					Version:       process.Version,
					WorkerId:      testWorkerId,
				})
				if err != nil {
					t.Fatalf("failed to create process instance: %v", err)
				}

				// then
				assert.Empty(processInstance2.StartedAt)
				assert.Equal(engine.InstanceQueued, processInstance2.State)

				// when process instance 3 is created
				processInstance3, err := e.CreateProcessInstance(engine.CreateProcessInstanceCmd{
					BpmnProcessId: process.BpmnProcessId,
					Version:       process.Version,
					WorkerId:      testWorkerId,
				})
				if err != nil {
					t.Fatalf("failed to create process instance: %v", err)
				}

				// then
				assert.Empty(processInstance3.StartedAt)
				assert.Equal(engine.InstanceQueued, processInstance3.State)

				piAssert1 := engine.NewProcessInstanceAssert(t, e, processInstance1)
				piAssert2 := engine.NewProcessInstanceAssert(t, e, processInstance2)
				piAssert3 := engine.NewProcessInstanceAssert(t, e, processInstance3)

				// when process instance 1 is ended
				piAssert1.IsWaitingAt("sendTask")
				piAssert1.CompleteJob()

				// when process instance 2 is started and ended
				tasks := piAssert2.ExecuteTasks()
				assert.Len(tasks, 1)
				assert.Equal(engine.TaskStartProcessInstance, tasks[0].Type)
				assert.Empty(tasks[0].Error)

				piAssert2.IsWaitingAt("sendTask")
				piAssert2.CompleteJob()

				// when start process instance 3 is started
				tasks = piAssert3.ExecuteTasks()
				assert.Len(tasks, 1)
				assert.Equal(engine.TaskStartProcessInstance, tasks[0].Type)
				assert.Empty(tasks[0].Error)

				// then
				processInstance1 = piAssert1.ProcessInstance()
				assert.Equal(engine.InstanceEnded, processInstance1.State)
				assert.True(processInstance1.IsEnded())
				piAssert1.IsEnded()

				processInstance2 = piAssert2.ProcessInstance()
				assert.NotEmpty(processInstance2.StartedAt)
				assert.Equal(engine.InstanceEnded, processInstance2.State)
				assert.True(processInstance2.IsEnded())
				piAssert2.IsEnded()

				processInstance3 = piAssert3.ProcessInstance()
				assert.NotEmpty(processInstance3.StartedAt)
				assert.Equal(engine.InstanceStarted, processInstance3.State)
				assert.False(processInstance3.IsEnded())
				piAssert3.IsNotEnded()
			})
		}
	})

	t.Run("create with parallelism and dequeue", func(t *testing.T) {
		for i, e := range engines {
			t.Run(engineTypes[i], func(t *testing.T) {
				// given
				bpmnXml := mustReadBpmnFile(t, "task/script.bpmn")

				createProcess := func(version string, parallelism int) {
					_, err := e.CreateProcess(engine.CreateProcessCmd{
						BpmnProcessId: "scriptTest",
						BpmnXml:       bpmnXml,
						Parallelism:   parallelism,
						Version:       version,
						WorkerId:      testWorkerId,
					})
					if err != nil {
						t.Fatalf("failed to create process: %v", err)
					}
				}

				var piAsserts []*engine.ProcessInstanceAssert

				createProcessInstance := func(version string) {
					processInstance, err := e.CreateProcessInstance(engine.CreateProcessInstanceCmd{
						BpmnProcessId: "scriptTest",
						Version:       version,
						WorkerId:      testWorkerId,
					})
					if err != nil {
						t.Fatalf("failed to create process instance: %v", err)
					}

					piAsserts = append(piAsserts, engine.NewProcessInstanceAssert(t, e, processInstance))
				}

				executeTasks := func(taskType engine.TaskType) []engine.Task {
					completedTasks, _, err := e.ExecuteTasks(engine.ExecuteTasksCmd{
						Type: taskType,
					})
					if err != nil {
						t.Fatalf("failed to execute tasks: %v", err)
					}

					return completedTasks
				}

				// given
				createProcess("1", 4)

				// when
				createProcessInstance("1")
				createProcessInstance("1")
				createProcessInstance("1")
				createProcessInstance("1")
				createProcessInstance("1")
				createProcessInstance("1")

				// then
				assert.Equal(engine.InstanceStarted, piAsserts[0].ProcessInstance().State)
				assert.Equal(engine.InstanceStarted, piAsserts[1].ProcessInstance().State)
				assert.Equal(engine.InstanceStarted, piAsserts[2].ProcessInstance().State)
				assert.Equal(engine.InstanceStarted, piAsserts[3].ProcessInstance().State)

				assert.Equal(engine.InstanceQueued, piAsserts[4].ProcessInstance().State)
				assert.Equal(engine.InstanceQueued, piAsserts[5].ProcessInstance().State)

				// given
				createProcess("2", 8)

				// when
				createProcessInstance("1")

				// then
				assert.Equal(engine.InstanceQueued, piAsserts[6].ProcessInstance().State)

				var tasks []engine.Task

				// when
				tasks = executeTasks(engine.TaskDequeueProcessInstance)
				assert.Len(tasks, 1)

				tasks = executeTasks(engine.TaskStartProcessInstance)
				assert.Len(tasks, 1)

				// then
				assert.Equal(engine.InstanceStarted, piAsserts[4].ProcessInstance().State)

				// given
				createProcess("3", 0)

				// when
				tasks = executeTasks(engine.TaskDequeueProcessInstance)
				assert.Len(tasks, 1)
				tasks = executeTasks(engine.TaskDequeueProcessInstance)
				assert.Len(tasks, 1)

				tasks = executeTasks(engine.TaskStartProcessInstance)
				assert.Len(tasks, 1)
				tasks = executeTasks(engine.TaskStartProcessInstance)
				assert.Len(tasks, 1)

				// then
				assert.Equal(engine.InstanceStarted, piAsserts[5].ProcessInstance().State)
				assert.Equal(engine.InstanceStarted, piAsserts[6].ProcessInstance().State)
			})
		}
	})
}

func TestSuspendAndResumeProcessInstance(t *testing.T) {
	assert := assert.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	for i, e := range engines {
		// given
		process := mustCreateProcess(t, e, "gateway/parallel-service-tasks.bpmn", "parallelServiceTasksTest")
		piAssert := mustCreateProcessInstance(t, e, process)
		processInstance := piAssert.ProcessInstance()

		t.Run(engineTypes[i]+"suspend", func(t *testing.T) {
			// when
			if err := e.SuspendProcessInstance(engine.SuspendProcessInstanceCmd{
				Partition: processInstance.Partition,
				Id:        processInstance.Id,
				WorkerId:  testWorkerId,
			}); err != nil {
				t.Fatalf("failed to suspend process instance: %v", err)
			}

			// then
			processInstance = piAssert.ProcessInstance()
			assert.Equal(engine.InstanceSuspended, processInstance.State)

			piAssert.IsWaitingAt("serviceTaskA")
			piAssert.CompleteJob()
			piAssert.IsWaitingAt("serviceTaskB")
			piAssert.CompleteJob()

			piAssert.IsNotWaitingAt("join")
		})

		t.Run(engineTypes[i]+"suspend returns error when process instance is not started", func(t *testing.T) {
			err := e.SuspendProcessInstance(engine.SuspendProcessInstanceCmd{
				Partition: processInstance.Partition,
				Id:        processInstance.Id,
				WorkerId:  testWorkerId,
			})

			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorConflict, engineErr.Type)
		})

		t.Run(engineTypes[i]+"suspend returns error when process instance not exists", func(t *testing.T) {
			err := e.SuspendProcessInstance(engine.SuspendProcessInstanceCmd{})
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorNotFound, engineErr.Type)
		})

		t.Run(engineTypes[i]+"resume", func(t *testing.T) {
			// when
			if err := e.ResumeProcessInstance(engine.ResumeProcessInstanceCmd{
				Partition: processInstance.Partition,
				Id:        processInstance.Id,
				WorkerId:  testWorkerId,
			}); err != nil {
				t.Fatalf("failed to resume process instance: %v", err)
			}

			// then
			processInstance = piAssert.ProcessInstance()
			assert.Equal(engine.InstanceStarted, processInstance.State)

			piAssert.IsWaitingAt("join")
			piAssert.ExecuteTask()

			piAssert.IsEnded()
		})

		t.Run(engineTypes[i]+"resume returns error when process instance is not suspended", func(t *testing.T) {
			err := e.ResumeProcessInstance(engine.ResumeProcessInstanceCmd{
				Partition: processInstance.Partition,
				Id:        processInstance.Id,
				WorkerId:  testWorkerId,
			})

			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorConflict, engineErr.Type)
		})

		t.Run(engineTypes[i]+"resume returns error when process instance not exists", func(t *testing.T) {
			err := e.ResumeProcessInstance(engine.ResumeProcessInstanceCmd{})
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorNotFound, engineErr.Type)
		})
	}
}
