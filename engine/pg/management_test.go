package pg

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/stretchr/testify/assert"
)

func TestMigrateAndPrepareDatabase(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	pgEngine := e.(*pgEngine)

	// given
	now := time.Date(2023, 12, 24, 13, 14, 15, 123456789, time.UTC)

	w, cancel := pgEngine.withTimeout()

	// when
	ctx, err := w.require()
	if err != nil {
		cancel()
		t.Fatalf("failed to require context: %v", err)
	}

	ctx.time = now

	err = prepareDatabase(ctx)
	if err := w.release(ctx, err); err != nil {
		cancel()
		t.Fatalf("failed to prepare database: %v", err)
	}

	cancel()

	t.Run("schema version set", func(t *testing.T) {
		// when
		w, cancel := pgEngine.withTimeout()
		defer cancel()

		ctx, err := w.require()
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		schemaVersion, err := selectSchemaVersion(ctx)
		if err := w.release(ctx, err); err != nil {
			t.Fatalf("failed to select schema version: %v", err)
		}

		// then
		assert.NotEmpty(schemaVersion)
	})

	t.Run("tasks created", func(t *testing.T) {
		// then
		results, err := e.Query(engine.TaskCriteria{Partition: engine.Partition(now.AddDate(0, 0, 1))})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		assert.Lenf(results, 1, "expected one task")

		createPartition := results[0].(engine.Task)
		assert.Equal(time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC), time.Time(createPartition.Partition))
		assert.Equal(time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC), createPartition.DueAt)
		assert.Equal(engine.TaskCreatePartition, createPartition.Type)

		var createPartitionTask createPartitionTask
		if err := json.Unmarshal([]byte(createPartition.SerializedTask), &createPartitionTask); err != nil {
			t.Fatalf("failed to unmarshal create partition task: %v", err)
		}

		assert.Equal(time.Date(2023, 12, 27, 0, 0, 0, 0, time.UTC), time.Time(createPartitionTask.Partition))

		// then
		results, err = e.Query(engine.TaskCriteria{Partition: engine.Partition(now.AddDate(0, 0, 2))})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		assert.Len(results, 1)

		detachPartition := results[0].(engine.Task)
		assert.Equal(time.Date(2023, 12, 26, 0, 0, 0, 0, time.UTC), time.Time(detachPartition.Partition))
		assert.Equal(time.Date(2023, 12, 26, 0, 5, 0, 0, time.UTC), detachPartition.DueAt)
		assert.Equal(engine.TaskDetachPartition, detachPartition.Type)

		var detachPartitionTask detachPartitionTask
		if err := json.Unmarshal([]byte(detachPartition.SerializedTask), &detachPartitionTask); err != nil {
			t.Fatalf("failed to unmarshal create partition task: %v", err)
		}

		assert.Equal(time.Date(2023, 12, 24, 0, 0, 0, 0, time.UTC), time.Time(detachPartitionTask.Partition))
	})

	t.Run("no tasks created when prepared again ", func(t *testing.T) {
		// when
		if err := pgEngine.migrateAndPrepareDatabase(); err != nil {
			t.Fatalf("failed to migrate and prepare database: %v", err)
		}

		// then
		results, err := e.Query(engine.TaskCriteria{Partition: engine.Partition(now.AddDate(0, 0, 1))})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		assert.Len(results, 1)

		// then
		results, err = e.Query(engine.TaskCriteria{Partition: engine.Partition(now.AddDate(0, 0, 2))})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		assert.Len(results, 1)
	})
}

func TestPartitionManagementTasks(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t, func(o *Options) {
		o.DropPartitionEnabled = false
	})
	defer e.Shutdown()

	pgEngine := e.(*pgEngine)

	execute := func(t *testing.T, task internal.Task) {
		w, cancel := pgEngine.withTimeout()
		defer cancel()

		ctx, err := w.require()
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		err = task.Execute(ctx, nil)
		if err := w.release(ctx, err); err != nil {
			t.Fatalf("failed to execute task: %v", err)
		}
	}

	// given
	date, _ := time.Parse(time.DateOnly, "2023-12-24")

	// when
	execute(t, createPartitionTask{Partition: engine.Partition(date), initial: true})

	// then
	w, cancel := pgEngine.withTimeout()
	defer cancel()

	ctx, err := w.require()
	if err != nil {
		t.Fatalf("failed to require context: %v", err)
	}

	for _, partitionedTable := range partitionedTables {
		partitionExists, err := selectTablePartitionExists(ctx, partitionedTable, date)
		if err != nil {
			w.release(ctx, err)
			t.Fatal(err)
		}

		assert.True(partitionExists, "partition %s of table %s not exists", engine.Partition(date), partitionedTable)
	}

	w.release(ctx, nil)

	// when
	execute(t, detachPartitionTask{Partition: engine.Partition(date)})
	execute(t, dropPartitionTask{Partition: engine.Partition(date)})

	// then
	ctx, err = w.require()
	if err != nil {
		t.Fatalf("failed to require context: %v", err)
	}

	for _, partitionedTable := range partitionedTables {
		partitionExists, err := selectTablePartitionExists(ctx, partitionedTable, date)
		if err != nil {
			w.release(ctx, err)
			t.Fatal(err)
		}

		assert.False(partitionExists, "partition %s of table %s exists", engine.Partition(date), partitionedTable)
	}

	w.release(ctx, nil)

	t.Run("detach partition with DropPartitionEnabled option", func(t *testing.T) {
		// given
		date, _ := time.Parse(time.DateOnly, "2023-12-25")

		execute(t, createPartitionTask{Partition: engine.Partition(date), initial: true})

		// when
		ctx, err := w.require()
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		options := ctx.options
		options.DropPartitionEnabled = true

		ctx.options = options

		detachPartitionTask := detachPartitionTask{Partition: engine.Partition(date)}
		err = detachPartitionTask.Execute(ctx, nil)
		if err := w.release(ctx, err); err != nil {
			t.Fatalf("failed to detach partition: %v", err)
		}

		// then
		results, err := e.Query(engine.TaskCriteria{Type: engine.TaskDropPartition})
		if err != nil {
			t.Fatalf("failed to query task: %v", err)
		}

		assert.Lenf(results, 1, "expected one task")

		task := results[0].(engine.Task)
		assert.NotEmpty(task.DueAt)

		completedTasks, _, err := e.ExecuteTasks(engine.ExecuteTasksCmd{
			Id:        task.Id,
			Partition: task.Partition,
		})
		if err != nil {
			t.Fatalf("failed to execute task: %v", err)
		}

		assert.False(completedTasks[0].HasError())

		ctx, err = w.require()
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		for _, partitionedTable := range partitionedTables {
			partitionExists, err := selectTablePartitionExists(ctx, partitionedTable, date)
			if err != nil {
				w.release(ctx, err)
				t.Fatal(err)
			}

			assert.False(partitionExists, "partition %s of table %s exists", engine.Partition(date), partitionedTable)
		}

		w.release(ctx, nil)
	})
}
