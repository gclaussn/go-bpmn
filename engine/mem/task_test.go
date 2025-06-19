package mem

import (
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestExecuteTask(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	memEngine := e.(*memEngine)

	t.Run("completes with error when instance is not mapped", func(t *testing.T) {
		// given
		entity := &internal.TaskEntity{
			ElementId:         pgtype.Int4{Int32: 2, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: 20, Valid: true},
			ProcessId:         pgtype.Int4{Int32: 1, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: 10, Valid: true},

			RetryCount: 1,
			Type:       engine.TaskStartProcessInstance,
		}

		mustInsertEntities(t, e, []any{entity})

		// when
		ctx := memEngine.wlock()
		defer memEngine.unlock()

		err := internal.ExecuteTask(ctx, entity)

		// then
		assert.Nil(err)
		assert.True(entity.Error.Valid)
		assert.Equal("failed to map task of type START_PROCESS_INSTANCE", entity.Error.String)

		results, err := ctx.Incidents().Query(engine.IncidentCriteria{TaskId: entity.Id}, engine.QueryOptions{})
		assert.Nil(err)
		assert.Len(results, 1)

		incident := results[0].(engine.Incident)
		assert.Equal(engine.Partition(entity.Partition), incident.Partition)

		assert.Equal(entity.ElementId.Int32, incident.ElementId)
		assert.Equal(entity.ElementInstanceId.Int32, incident.ElementInstanceId)
		assert.Equal(int32(0), incident.JobId)
		assert.Equal(entity.ProcessId.Int32, incident.ProcessId)
		assert.Equal(entity.ProcessInstanceId.Int32, incident.ProcessInstanceId)
		assert.Equal(entity.Id, incident.TaskId)

		assert.NotEmpty(incident.CreatedAt)
		assert.Equal(engine.DefaultEngineId, incident.CreatedBy)
	})

	t.Run("create retry task when completed with an error and retries left", func(t *testing.T) {
		// given
		entity := &internal.TaskEntity{
			ElementId:         pgtype.Int4{Int32: 2, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: 20, Valid: true},
			ProcessId:         pgtype.Int4{Int32: 1, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: 10, Valid: true},

			RetryCount:     2,
			RetryTimer:     pgtype.Text{String: "PT1H", Valid: true},
			SerializedTask: pgtype.Text{String: "{...}", Valid: true},
			Type:           engine.TaskStartProcessInstance,

			Instance: dummyTask{},
		}

		mustInsertEntities(t, e, []any{entity})

		// when
		ctx := memEngine.wlock()
		defer memEngine.unlock()

		err := internal.ExecuteTask(ctx, entity)

		// then
		assert.Nil(err)
		assert.True(entity.Error.Valid)
		assert.Equal("dummy title: dummy detail", entity.Error.String)

		results, err := ctx.Tasks().Query(engine.TaskCriteria{Id: entity.Id + 1}, engine.QueryOptions{})
		assert.Nil(err)
		assert.Len(results, 1)

		task := results[0].(engine.Task)
		assert.Equal(engine.Partition(entity.Partition), task.Partition)

		assert.Equal(entity.ElementId.Int32, task.ElementId)
		assert.Equal(entity.ElementInstanceId.Int32, task.ElementInstanceId)
		assert.Equal(entity.ProcessId.Int32, task.ProcessId)
		assert.Equal(entity.ProcessInstanceId.Int32, task.ProcessInstanceId)

		assert.NotEmpty(task.CreatedAt)
		assert.Equal(engine.DefaultEngineId, task.CreatedBy)
		assert.Equal(ctx.Time().Add(time.Hour), task.DueAt)
		assert.Equal(1, task.RetryCount)
		assert.Equal(entity.RetryTimer.String, task.RetryTimer.String())
		assert.Equal(entity.SerializedTask.String, task.SerializedTask)
		assert.Equal(entity.Type, task.Type)
	})
}

// !keep in sync with engine/pg/task_test.go
func TestLockTasks(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	var (
		now       = time.Now().UTC()
		date      = now.Truncate(24 * time.Hour)
		datePlus1 = date.AddDate(0, 0, 1)

		partitions = []time.Time{date, date, date, datePlus1, datePlus1, datePlus1, datePlus1}

		elementInstanceIds = []int32{1, 2, 3, 4, 5, 6, 7}
		processIds         = []int32{1, 1, 1, 2, 2, 3, 4}
		processInstanceIds = []int32{10, 10, 10, 20, 20, 30, 40}

		dueAts = []time.Time{
			now.Add(time.Minute * -5),
			now.Add(time.Minute * -4),
			now.Add(time.Minute * -3),
			now.Add(time.Minute * -2),
			now.Add(time.Minute * -1),
			now.Add(time.Second * -15),
			now.Add(time.Hour), // not due
		}

		types = []engine.TaskType{
			engine.TaskStartProcessInstance,
			engine.TaskJoinParallelGateway,
			engine.TaskStartProcessInstance,
			engine.TaskJoinParallelGateway,
			engine.TaskStartProcessInstance,
			engine.TaskJoinParallelGateway,
			engine.TaskStartProcessInstance,
		}

		instances = []internal.Task{
			internal.StartProcessInstanceTask{},
			internal.JoinParallelGatewayTask{},
			internal.StartProcessInstanceTask{},
			internal.JoinParallelGatewayTask{},
			internal.StartProcessInstanceTask{},
			internal.JoinParallelGatewayTask{},
			internal.StartProcessInstanceTask{},
		}
	)

	tasks := make([]*internal.TaskEntity, len(partitions))
	entities := make([]any, len(tasks))
	for i := range tasks {
		tasks[i] = &internal.TaskEntity{
			Partition: partitions[i],

			ElementInstanceId: pgtype.Int4{Int32: elementInstanceIds[i], Valid: true},
			ProcessId:         pgtype.Int4{Int32: processIds[i], Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: processInstanceIds[i], Valid: true},

			DueAt: dueAts[i],
			Type:  types[i],

			Instance: instances[i],
		}

		entities[i] = tasks[i]
	}

	mustInsertEntities(t, e, entities)

	runTaskLockTests(t, e, []taskLockTest{
		{
			"no limit",
			engine.ExecuteTasksCmd{},
			func(assert *assert.Assertions, lockedTasks []engine.Task) {
				assert.Len(lockedTasks, 1)
				assert.Equal(engine.Partition(date), lockedTasks[0].Partition)
				assert.Equal(tasks[0].Id, lockedTasks[0].Id)
			},
		},
		{
			"limit",
			engine.ExecuteTasksCmd{Limit: 3},
			func(assert *assert.Assertions, lockedTasks []engine.Task) {
				assert.Len(lockedTasks, 3)
				assert.Equal(engine.Partition(date), lockedTasks[0].Partition)
				assert.Equal(tasks[1].Id, lockedTasks[0].Id)
				assert.Equal(engine.Partition(date), lockedTasks[1].Partition)
				assert.Equal(tasks[2].Id, lockedTasks[1].Id)
				assert.Equal(engine.Partition(datePlus1), lockedTasks[2].Partition)
				assert.Equal(tasks[3].Id, lockedTasks[2].Id)
			},
		},
	})

	runTaskLockTests(t, e, []taskLockTest{
		{
			"by partition",
			engine.ExecuteTasksCmd{Partition: engine.Partition(datePlus1), Limit: len(entities)},
			func(assert *assert.Assertions, lockedTasks []engine.Task) {
				assert.Len(lockedTasks, 3)
				assert.Equal(engine.Partition(datePlus1), lockedTasks[0].Partition)
				assert.Equal(tasks[3].Id, lockedTasks[0].Id)
				assert.Equal(engine.Partition(datePlus1), lockedTasks[1].Partition)
				assert.Equal(tasks[4].Id, lockedTasks[1].Id)
				assert.Equal(engine.Partition(datePlus1), lockedTasks[2].Partition)
				assert.Equal(tasks[5].Id, lockedTasks[2].Id)
			},
		},
	})

	runTaskLockTests(t, e, []taskLockTest{
		{
			"by partition and ID",
			engine.ExecuteTasksCmd{Partition: engine.Partition(date), Id: 2, Limit: len(entities)},
			func(assert *assert.Assertions, lockedTasks []engine.Task) {
				assert.Len(lockedTasks, 1)
				assert.Equal(int32(2), lockedTasks[0].Id)
			},
		},
		{
			"by partition and ID, but already locked",
			engine.ExecuteTasksCmd{Partition: engine.Partition(date), Id: 2, Limit: len(entities)},
			func(assert *assert.Assertions, lockedTasks []engine.Task) {
				assert.Len(lockedTasks, 0)
			},
		},
	})

	runTaskLockTests(t, e, []taskLockTest{
		{
			"by element instance ID",
			engine.ExecuteTasksCmd{ElementInstanceId: 3, Limit: len(entities)},
			func(assert *assert.Assertions, lockedTasks []engine.Task) {
				assert.Len(lockedTasks, 1)
				assert.Equal(int32(3), lockedTasks[0].ElementInstanceId)
			},
		},
	})

	runTaskLockTests(t, e, []taskLockTest{
		{
			"by process instance ID",
			engine.ExecuteTasksCmd{ProcessInstanceId: 10, Limit: len(partitions)},
			func(assert *assert.Assertions, lockedTasks []engine.Task) {
				assert.Len(lockedTasks, 3)
				assert.Equal(int32(10), lockedTasks[0].ProcessInstanceId)
				assert.Equal(int32(10), lockedTasks[1].ProcessInstanceId)
				assert.Equal(int32(10), lockedTasks[2].ProcessInstanceId)
			},
		},
	})

	runTaskLockTests(t, e, []taskLockTest{
		{
			"by type",
			engine.ExecuteTasksCmd{Type: engine.TaskJoinParallelGateway, Limit: len(partitions)},
			func(assert *assert.Assertions, lockedTasks []engine.Task) {
				assert.Len(lockedTasks, 3)
				assert.Equal(engine.TaskJoinParallelGateway, lockedTasks[0].Type)
				assert.Equal(engine.TaskJoinParallelGateway, lockedTasks[1].Type)
				assert.Equal(engine.TaskJoinParallelGateway, lockedTasks[2].Type)
			},
		},
	})
}

// !keep in sync with engine/pg/task_test.go
func TestUnlockTasks(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	var (
		now       = time.Now().UTC()
		date      = now.Truncate(24 * time.Hour)
		datePlus1 = date.AddDate(0, 0, 1)

		partitions = []time.Time{date, date, date, datePlus1, datePlus1}
	)

	tasks := make([]*internal.TaskEntity, len(partitions))
	entities := make([]any, len(tasks))
	for i := range tasks {
		tasks[i] = &internal.TaskEntity{
			Partition: partitions[i],
		}

		entities[i] = tasks[i]
	}

	mustInsertEntities(t, e, entities)

	runTaskUnlockTests(t, e, []taskUnlockTest{
		{
			"by different engine ID",
			engine.UnlockTasksCmd{EngineId: "different-engine"},
			0,
		},
		{
			"by engine ID",
			engine.UnlockTasksCmd{EngineId: engine.DefaultEngineId},
			len(entities),
		},
		{
			"by engine ID, but already unlocked",
			engine.UnlockTasksCmd{EngineId: engine.DefaultEngineId},
			0,
		},
	})

	runTaskUnlockTests(t, e, []taskUnlockTest{
		{
			"by partition",
			engine.UnlockTasksCmd{EngineId: engine.DefaultEngineId, Partition: engine.Partition(date)},
			3,
		},
		{
			"by partition already unlocked",
			engine.UnlockTasksCmd{EngineId: engine.DefaultEngineId, Partition: engine.Partition(date)},
			0,
		},
	})

	runTaskUnlockTests(t, e, []taskUnlockTest{
		{
			"by partition and ID",
			engine.UnlockTasksCmd{EngineId: engine.DefaultEngineId, Partition: engine.Partition(date), Id: 3},
			1,
		},
		{
			"by partition and ID, but already unlocked",
			engine.UnlockTasksCmd{EngineId: engine.DefaultEngineId, Partition: engine.Partition(date), Id: 3},
			0,
		},
	})

	// complete tasks
	for i := range tasks {
		tasks[i].CompletedAt = pgtype.Timestamp{Time: now, Valid: true}
	}

	mustUpdateEntities(t, e, entities)

	runTaskUnlockTests(t, e, []taskUnlockTest{
		{
			"by engine ID, but already completed",
			engine.UnlockTasksCmd{EngineId: engine.DefaultEngineId},
			0,
		},
	})
}

type taskLockTest struct {
	name     string
	cmd      engine.ExecuteTasksCmd
	assertFn func(*assert.Assertions, []engine.Task)
}

type taskUnlockTest struct {
	name          string
	cmd           engine.UnlockTasksCmd
	expectedCount int
}

func runTaskLockTests(t *testing.T, e engine.Engine, tests []taskLockTest) {
	_, err := e.UnlockTasks(engine.UnlockTasksCmd{
		EngineId: engine.DefaultEngineId,
	})
	if err != nil {
		t.Fatalf("failed to unlock tasks: %v", err)
	}

	assert := assert.New(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.cmd

			memEngine := e.(*memEngine)

			ctx := memEngine.wlock()
			defer memEngine.unlock()

			entities, err := ctx.Tasks().Lock(cmd, ctx.Time())
			if err != nil {
				t.Fatalf("failed to lock tasks: %v", err)
			}

			lockedTasks := make([]engine.Task, len(entities))
			for i := range lockedTasks {
				lockedTasks[i] = entities[i].Task()
			}

			test.assertFn(assert, lockedTasks)
		})
	}
}

func runTaskUnlockTests(t *testing.T, e engine.Engine, tests []taskUnlockTest) {
	memEngine := e.(*memEngine)

	ctx := memEngine.wlock()
	_, err := ctx.Tasks().Lock(engine.ExecuteTasksCmd{Limit: 100}, ctx.Time())
	memEngine.unlock()

	if err != nil {
		t.Fatalf("failed to lock tasks: %v", err)
	}

	assert := assert.New(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			count, err := e.UnlockTasks(test.cmd)
			if err != nil {
				t.Fatalf("failed to unlock tasks: %v", err)
			}

			assert.Equal(test.expectedCount, count)
		})
	}
}

type dummyTask struct {
}

func (t dummyTask) Execute(ctx internal.Context, task *internal.TaskEntity) error {
	return engine.Error{
		Type:   engine.ErrorNotFound,
		Title:  "dummy title",
		Detail: "dummy detail",
	}
}
