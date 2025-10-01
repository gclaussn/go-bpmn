package pg

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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

		assert.Len(results, 3)

		detachPartition := results[0]
		assert.Equal(time.Date(2023, 12, 26, 0, 0, 0, 0, time.UTC), time.Time(detachPartition.Partition))
		assert.Equal(time.Date(2023, 12, 26, 0, 5, 0, 0, time.UTC), detachPartition.DueAt)
		assert.Equal(engine.TaskDetachPartition, detachPartition.Type)

		var detachPartitionTask detachPartitionTask
		if err := json.Unmarshal([]byte(detachPartition.SerializedTask), &detachPartitionTask); err != nil {
			t.Fatalf("failed to unmarshal create partition task: %v", err)
		}

		assert.Equal(time.Date(2023, 12, 24, 0, 0, 0, 0, time.UTC), time.Time(detachPartitionTask.Partition))

		purgeMessages := results[1]
		assert.Equal(time.Date(2023, 12, 26, 0, 0, 0, 0, time.UTC), time.Time(purgeMessages.Partition))
		assert.Equal(time.Date(2023, 12, 26, 0, 0, 0, 0, time.UTC), purgeMessages.DueAt)
		assert.Equal(engine.TaskPurgeMessages, purgeMessages.Type)

		purgeSignals := results[2]
		assert.Equal(time.Date(2023, 12, 26, 0, 0, 0, 0, time.UTC), time.Time(purgeSignals.Partition))
		assert.Equal(time.Date(2023, 12, 26, 0, 0, 0, 0, time.UTC), purgeSignals.DueAt)
		assert.Equal(engine.TaskPurgeSignals, purgeSignals.Type)
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

		assert.Len(results, 3)
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

func TestPurgeMessages(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	pgEngine := e.(*pgEngine)

	message1 := &internal.MessageEntity{
		CorrelationKey: "1",
		ExpiresAt:      pgtype.Timestamp{Time: time.Now().Add(time.Hour), Valid: true},
		Name:           "1",
	}
	message2 := &internal.MessageEntity{
		CorrelationKey: "2",
		ExpiresAt:      pgtype.Timestamp{Time: time.Now().Add(time.Hour * 2), Valid: true},
		Name:           "2",
	}
	message3 := &internal.MessageEntity{
		CorrelationKey: "3",
		ExpiresAt:      pgtype.Timestamp{},
		Name:           "3",
	}

	mustInsertEntities(t, e, []any{message1, message2, message3})

	messageVariableA := &internal.MessageVariableEntity{
		MessageId: message1.Id,

		Name: "a",
	}
	messageVariableB := &internal.MessageVariableEntity{
		MessageId: message1.Id,

		Name: "b",
	}
	messageVariableC := &internal.MessageVariableEntity{
		MessageId: message2.Id,

		Name: "c",
	}
	messageVariableD := &internal.MessageVariableEntity{
		MessageId: message3.Id,

		Name: "d",
	}

	mustInsertEntities(t, e, []any{messageVariableA, messageVariableB, messageVariableC, messageVariableD})

	if err := pgEngine.execute(func(pgCtx *pgContext) error {
		task := purgeMessagesTask{}

		if err := pgEngine.execute(func(pgCtx *pgContext) error {
			return task.Execute(pgCtx, &internal.TaskEntity{DueAt: message1.ExpiresAt.Time})
		}); err != nil {
			return err
		}

		_, err1 := pgCtx.Messages().Select(message1.Id)
		assert.ErrorIs(err1, pgx.ErrNoRows)

		_, err2 := pgCtx.Messages().Select(message2.Id)
		assert.NoError(err2)

		_, err3 := pgCtx.Messages().Select(message3.Id)
		assert.NoError(err3)

		messageVariables1, err1 := pgCtx.MessageVariables().SelectByMessageId(message1.Id)
		assert.NoError(err1)
		assert.Len(messageVariables1, 0)

		messageVariables2, err2 := pgCtx.MessageVariables().SelectByMessageId(message2.Id)
		assert.NoError(err2)
		assert.Len(messageVariables2, 1)

		messageVariables3, err3 := pgCtx.MessageVariables().SelectByMessageId(message3.Id)
		assert.NoError(err3)
		assert.Len(messageVariables3, 1)

		return nil
	}); err != nil {
		t.Fatalf("failed to purge messages: %v", err)
	}
}

func TestPurgeSignals(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	pgEngine := e.(*pgEngine)

	signal1 := &internal.SignalEntity{
		ActiveSubscriberCount: 0,
		Name:                  "1",
		SubscriberCount:       1,
	}
	signal2 := &internal.SignalEntity{
		ActiveSubscriberCount: 2,
		Name:                  "2",
		SubscriberCount:       2,
	}

	mustInsertEntities(t, e, []any{signal1, signal2})

	signalVariableA := &internal.SignalVariableEntity{
		SignalId: signal1.Id,

		Name: "a",
	}
	signalVariableB := &internal.SignalVariableEntity{
		SignalId: signal1.Id,

		Name: "b",
	}
	signalVariableC := &internal.SignalVariableEntity{
		SignalId: signal2.Id,

		Name: "c",
	}
	signalVariableD := &internal.SignalVariableEntity{
		SignalId: signal2.Id,

		Name: "d",
	}

	mustInsertEntities(t, e, []any{signalVariableA, signalVariableB, signalVariableC, signalVariableD})

	if err := pgEngine.execute(func(pgCtx *pgContext) error {
		task := purgeSignalsTask{}

		if err := pgEngine.execute(func(pgCtx *pgContext) error {
			return task.Execute(pgCtx, nil)
		}); err != nil {
			return err
		}

		_, err1 := pgCtx.Signals().Select(signal1.Id)
		assert.ErrorIs(err1, pgx.ErrNoRows)

		_, err2 := pgCtx.Signals().Select(signal2.Id)
		assert.NoError(err2)

		signalVariables1, err1 := pgCtx.SignalVariables().SelectBySignalId(signal1.Id)
		assert.NoError(err1)
		assert.Len(signalVariables1, 0)

		signalVariables2, err2 := pgCtx.SignalVariables().SelectBySignalId(signal2.Id)
		assert.NoError(err2)
		assert.Len(signalVariables2, 2)

		return nil
	}); err != nil {
		t.Fatalf("failed to purge signals: %v", err)
	}
}
