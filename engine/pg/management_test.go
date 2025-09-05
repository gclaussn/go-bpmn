package pg

import (
	"context"
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

	q := e.CreateQuery()

	// given
	now := time.Date(2023, 12, 24, 13, 14, 15, 123456789, time.UTC)

	// when
	if err := pgEngine.execute(func(pgCtx *pgContext) error {
		pgCtx.time = now

		return prepareDatabase(pgCtx)
	}); err != nil {
		t.Fatalf("failed to prepare database: %v", err)
	}

	t.Run("schema version set", func(t *testing.T) {
		// when
		if err := pgEngine.execute(func(pgCtx *pgContext) error {
			schemaVersion, err := selectSchemaVersion(pgCtx)

			// then
			assert.NotEmpty(schemaVersion)

			return err
		}); err != nil {
			t.Fatalf("failed to select schema version: %v", err)
		}
	})

	t.Run("tasks created", func(t *testing.T) {
		// then
		results, err := q.QueryTasks(context.Background(), engine.TaskCriteria{Partition: engine.Partition(now.AddDate(0, 0, 1))})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		assert.Lenf(results, 1, "expected one task")

		createPartition := results[0]
		assert.Equal(time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC), time.Time(createPartition.Partition))
		assert.Equal(time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC), createPartition.DueAt)
		assert.Equal(engine.TaskCreatePartition, createPartition.Type)

		var createPartitionTask createPartitionTask
		if err := json.Unmarshal([]byte(createPartition.SerializedTask), &createPartitionTask); err != nil {
			t.Fatalf("failed to unmarshal create partition task: %v", err)
		}

		assert.Equal(time.Date(2023, 12, 27, 0, 0, 0, 0, time.UTC), time.Time(createPartitionTask.Partition))

		// then
		results, err = q.QueryTasks(context.Background(), engine.TaskCriteria{Partition: engine.Partition(now.AddDate(0, 0, 2))})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		assert.Len(results, 1)

		detachPartition := results[0]
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
		results, err := q.QueryTasks(context.Background(), engine.TaskCriteria{Partition: engine.Partition(now.AddDate(0, 0, 1))})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		assert.Len(results, 1)

		// then
		results, err = q.QueryTasks(context.Background(), engine.TaskCriteria{Partition: engine.Partition(now.AddDate(0, 0, 2))})
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

	q := e.CreateQuery()

	executeTask := func(t *testing.T, task internal.Task) {
		if err := pgEngine.execute(func(pgCtx *pgContext) error {
			return task.Execute(pgCtx, nil)
		}); err != nil {
			t.Fatalf("failed to execute task: %v", err)
		}
	}

	// given
	date, _ := time.Parse(time.DateOnly, "2023-12-24")

	// when
	executeTask(t, createPartitionTask{Partition: engine.Partition(date), initial: true})

	// then
	if err := pgEngine.execute(func(pgCtx *pgContext) error {
		for _, partitionedTable := range partitionedTables {
			partitionExists, err := selectTablePartitionExists(pgCtx, partitionedTable, date)
			if err != nil {
				return err
			}
			assert.True(partitionExists, "partition %s of table %s not exists", engine.Partition(date), partitionedTable)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// when
	executeTask(t, detachPartitionTask{Partition: engine.Partition(date)})
	executeTask(t, dropPartitionTask{Partition: engine.Partition(date)})

	// then
	if err := pgEngine.execute(func(pgCtx *pgContext) error {
		for _, partitionedTable := range partitionedTables {
			partitionExists, err := selectTablePartitionExists(pgCtx, partitionedTable, date)
			if err != nil {
				return err
			}
			assert.False(partitionExists, "partition %s of table %s exists", engine.Partition(date), partitionedTable)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("detach partition with DropPartitionEnabled option", func(t *testing.T) {
		// given
		date, _ := time.Parse(time.DateOnly, "2023-12-25")

		executeTask(t, createPartitionTask{Partition: engine.Partition(date), initial: true})

		// when
		if err := pgEngine.execute(func(pgCtx *pgContext) error {
			options := pgCtx.options
			options.DropPartitionEnabled = true

			pgCtx.options = options

			detachPartitionTask := detachPartitionTask{Partition: engine.Partition(date)}
			return detachPartitionTask.Execute(pgCtx, nil)
		}); err != nil {
			t.Fatalf("failed to detach partition: %v", err)
		}

		// then
		results, err := q.QueryTasks(context.Background(), engine.TaskCriteria{Type: engine.TaskDropPartition})
		if err != nil {
			t.Fatalf("failed to query task: %v", err)
		}

		assert.Lenf(results, 1, "expected one task")

		assert.NotEmpty(results[0].DueAt)

		completedTasks, _, err := e.ExecuteTasks(context.Background(), engine.ExecuteTasksCmd{
			Id:        results[0].Id,
			Partition: results[0].Partition,
		})
		if err != nil {
			t.Fatalf("failed to execute task: %v", err)
		}

		assert.False(completedTasks[0].HasError())

		if err := pgEngine.execute(func(pgCtx *pgContext) error {
			for _, partitionedTable := range partitionedTables {
				partitionExists, err := selectTablePartitionExists(pgCtx, partitionedTable, date)
				if err != nil {
					return err
				}
				assert.False(partitionExists, "partition %s of table %s exists", engine.Partition(date), partitionedTable)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})
}
