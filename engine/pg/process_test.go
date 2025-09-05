package pg

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// !keep in sync with engine/mem/process_test.go
func TestProcessCache(t *testing.T) {
	assert := assert.New(t)

	var (
		cachedEntity *internal.ProcessEntity
		ok           bool
		err          error
		engineErr    engine.Error
	)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	pgEngine := e.(*pgEngine)

	var processCache *internal.ProcessCache
	pgEngine.execute(func(pgCtx *pgContext) error {
		processCache = pgCtx.processCache
		return nil
	})

	t.Run("get", func(t *testing.T) {
		// when
		cachedEntity, ok = processCache.Get("startEndTest", "1")

		// then
		assert.Nil(cachedEntity)
		assert.False(ok)

		// when
		cachedEntity, ok = processCache.GetById(1)

		// then
		assert.Nil(cachedEntity)
		assert.False(ok)
	})

	t.Run("get or cache when empty", func(t *testing.T) {
		pgCtx, cancel, err := pgEngine.acquire(context.Background())
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		defer cancel()
		defer pgEngine.release(pgCtx, nil)

		// when
		cachedEntity, err = processCache.GetOrCache(pgCtx, "startEndTest", "1")

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		// when
		cachedEntity, err = processCache.GetOrCacheById(pgCtx, 1)

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)
	})

	t.Run("get or cache", func(t *testing.T) {
		// given
		_, err = e.CreateProcess(context.Background(), engine.CreateProcessCmd{
			BpmnProcessId: "startEndTest",
			BpmnXml:       mustReadBpmnFile(t, "start-end.bpmn"),
			Version:       "1",
			WorkerId:      "test-worker",
		})
		if err != nil {
			t.Fatalf("failed to create process: %v", err)
		}

		// when
		cachedEntity, ok = processCache.Get("startEndTest", "1")

		// then
		assert.NotNil(cachedEntity)
		assert.True(ok)

		// when
		cachedEntity, ok = processCache.GetById(1)

		// then
		assert.NotNil(cachedEntity)
		assert.True(ok)

		pgCtx, cancel, err := pgEngine.acquire(context.Background())
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		defer cancel()
		defer pgEngine.release(pgCtx, nil)

		// when
		cachedEntity, err = processCache.GetOrCache(pgCtx, "startEndTest", "1")

		// then
		assert.NotNil(cachedEntity)
		assert.Nil(err)

		// when
		cachedEntity, err = processCache.GetOrCacheById(pgCtx, 1)

		// then
		assert.NotNil(cachedEntity)
		assert.Nil(err)

		// when
		processCache.Clear()

		cachedEntity, err = processCache.GetOrCache(pgCtx, "startEndTest", "1")

		// then
		assert.NotNil(cachedEntity)
		assert.Nil(err)

		// when
		processCache.Clear()

		cachedEntity, err = processCache.GetOrCacheById(pgCtx, 1)

		// then
		assert.NotNil(cachedEntity)
		assert.Nil(err)
	})

	t.Run("cache returns error when process element not exists", func(t *testing.T) {
		// given
		entity := &internal.ProcessEntity{
			BpmnProcessId: "not-existing",
			BpmnXml:       mustReadBpmnFile(t, "start-end.bpmn"),
			Version:       "1",
		}

		mustInsertEntities(t, e, []any{entity})

		pgCtx, cancel, err := pgEngine.acquire(context.Background())
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		defer cancel()
		defer pgEngine.release(pgCtx, nil)

		// when
		cachedEntity, err = processCache.GetOrCache(pgCtx, "not-existing", "1")

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorBug, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)

		// when
		cachedEntity, err = processCache.GetOrCacheById(pgCtx, entity.Id)

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorBug, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)
	})

	t.Run("cache returns error when model is invalid", func(t *testing.T) {
		// given
		entity := &internal.ProcessEntity{
			BpmnProcessId: "processNotExecutableTest",
			BpmnXml:       mustReadBpmnFile(t, "invalid/process-not-executable.bpmn"),
			Version:       "1",
		}

		mustInsertEntities(t, e, []any{entity})

		pgCtx, cancel, err := pgEngine.acquire(context.Background())
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		defer cancel()
		defer pgEngine.release(pgCtx, nil)

		// when
		cachedEntity, err = processCache.GetOrCache(pgCtx, "processNotExecutableTest", "1")

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorBug, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)

		// when
		cachedEntity, err = processCache.GetOrCacheById(pgCtx, entity.Id)

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorBug, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)
	})
}

func TestProcessRepository(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	pgEngine := e.(*pgEngine)

	t.Run("concurrent insert with conflict", func(t *testing.T) {
		pgCtx1, cancel1, err := pgEngine.acquire(context.Background())
		if err != nil {
			t.Errorf("failed to borrow context: %v", err)
		}

		defer cancel1()

		pgCtx2, cancel2, err := pgEngine.acquire(context.Background())
		if err != nil {
			t.Errorf("failed to borrow context: %v", err)
		}

		defer cancel2()

		process1 := internal.ProcessEntity{
			BpmnProcessId: "a",
			BpmnXml:       "xmla",
			BpmnXmlMd5:    "xmlmd5a",
			CreatedAt:     pgCtx1.Time(),
			CreatedBy:     "test",
			Parallelism:   2,
			Tags:          pgtype.Text{String: `{"a": "b"}`, Valid: true},
			Version:       "1",
		}

		process2 := internal.ProcessEntity{
			BpmnProcessId: "a",
			BpmnXml:       "xmla",
			BpmnXmlMd5:    "xmlmd5a",
			CreatedAt:     pgCtx2.Time(),
			CreatedBy:     "test",
			Parallelism:   3,
			Tags:          pgtype.Text{String: `{"a": "b*"}`, Valid: true},
			Version:       "1",
		}

		insert1Ctx, insert1Cancel := context.WithCancel(context.Background())
		insert2Ctx, insert2Cancel := context.WithCancel(context.Background())

		var insertErr1 error
		var insertErr2 error
		go func() {
			insertErr1 = pgCtx1.Processes().Insert(&process1)
			insert1Cancel()

			insertErr2 = pgCtx2.Processes().Insert(&process2)
			insert2Cancel()
		}()

		<-insert1Ctx.Done()

		if insertErr1 != nil {
			t.Errorf("failed to insert process 1: %v", insertErr1)
		}

		pgEngine.release(pgCtx1, insertErr1)

		<-insert2Ctx.Done()

		assert.Equal(pgx.ErrNoRows, insertErr2)

		pgEngine.release(pgCtx2, insertErr2)

		if err := pgEngine.execute(func(pgCtx *pgContext) error {
			selectedProcess, err := pgCtx.Processes().Select(process1.Id)

			assert.Equal(&process1, selectedProcess)

			return err
		}); err != nil {
			t.Errorf("failed to select process: %v", err)
		}
	})
}
