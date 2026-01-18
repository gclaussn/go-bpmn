package pg

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// !keep in sync with engine/mem/task_test.go
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
			State: engine.WorkCreated,
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
				assert.Equal(engine.WorkLocked, lockedTasks[0].State)
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
			"by process ID",
			engine.ExecuteTasksCmd{ProcessId: 2, Limit: len(partitions)},
			func(assert *assert.Assertions, lockedTasks []engine.Task) {
				assert.Len(lockedTasks, 2)
				assert.Equal(int32(2), lockedTasks[0].ProcessId)
				assert.Equal(int32(2), lockedTasks[1].ProcessId)
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

// !keep in sync with engine/mem/task_test.go
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
	_, err := e.UnlockTasks(context.Background(), engine.UnlockTasksCmd{
		EngineId: engine.DefaultEngineId,
	})
	if err != nil {
		t.Fatalf("failed to unlock tasks: %v", err)
	}

	assert := assert.New(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := test.cmd

			pgEngine := e.(*pgEngine)

			pgCtx, cancel, err := pgEngine.acquire(context.Background())
			if err != nil {
				t.Fatalf("failed to require context: %v", err)
			}

			defer cancel()

			entities, err := pgCtx.Tasks().Lock(cmd, pgCtx.Time())
			if err := pgEngine.release(pgCtx, err); err != nil {
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
	pgEngine := e.(*pgEngine)

	if err := pgEngine.execute(func(pgCtx *pgContext) error {
		_, err := pgCtx.Tasks().Lock(engine.ExecuteTasksCmd{Limit: 100}, pgCtx.Time())
		return err
	}); err != nil {
		t.Fatalf("failed to lock tasks: %v", err)
	}

	assert := assert.New(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			count, err := e.UnlockTasks(context.Background(), test.cmd)
			if err != nil {
				t.Fatalf("failed to unlock tasks: %v", err)
			}

			assert.Equal(test.expectedCount, count)
		})
	}
}
