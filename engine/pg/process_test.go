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
	w, cancel := pgEngine.withTimeout()

	ctx, err := w.require()
	if err != nil {
		cancel()
		t.Fatalf("failed to require context: %v", err)
	}

	if err := w.release(ctx, nil); err != nil {
		cancel()
		t.Fatalf("failed to release context: %v", err)
	}

	cancel()

	processCache := ctx.processCache

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
		w, cancel := pgEngine.withTimeout()
		defer cancel()

		ctx, err := w.require()
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		defer w.release(ctx, nil)

		// when
		cachedEntity, err = processCache.GetOrCache(ctx, "startEndTest", "1")

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		// when
		cachedEntity, err = processCache.GetOrCacheById(ctx, 1)

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)
	})

	t.Run("get or cache", func(t *testing.T) {
		// given
		_, err = e.CreateProcess(engine.CreateProcessCmd{
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

		w, cancel := pgEngine.withTimeout()
		defer cancel()

		ctx, err := w.require()
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		defer w.release(ctx, nil)

		// when
		cachedEntity, err = processCache.GetOrCache(ctx, "startEndTest", "1")

		// then
		assert.NotNil(cachedEntity)
		assert.Nil(err)

		// when
		cachedEntity, err = processCache.GetOrCacheById(ctx, 1)

		// then
		assert.NotNil(cachedEntity)
		assert.Nil(err)

		// when
		processCache.Clear()

		cachedEntity, err = processCache.GetOrCache(ctx, "startEndTest", "1")

		// then
		assert.NotNil(cachedEntity)
		assert.Nil(err)

		// when
		processCache.Clear()

		cachedEntity, err = processCache.GetOrCacheById(ctx, 1)

		// then
		assert.NotNil(cachedEntity)
		assert.Nil(err)
	})

	t.Run("cache returns error when process element not exist", func(t *testing.T) {
		// given
		entity := &internal.ProcessEntity{
			BpmnProcessId: "not-existing",
			BpmnXml:       mustReadBpmnFile(t, "start-end.bpmn"),
			Version:       "1",
		}

		mustInsertEntities(t, e, []any{entity})

		w, cancel := pgEngine.withTimeout()
		defer cancel()

		ctx, err := w.require()
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		defer w.release(ctx, nil)

		// when
		cachedEntity, err = processCache.GetOrCache(ctx, "not-existing", "1")

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorProcessModel, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)

		// when
		cachedEntity, err = processCache.GetOrCacheById(ctx, entity.Id)

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorProcessModel, engineErr.Type)
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

		w, cancel := pgEngine.withTimeout()
		defer cancel()

		ctx, err := w.require()
		if err != nil {
			t.Fatalf("failed to require context: %v", err)
		}

		defer w.release(ctx, nil)

		// when
		cachedEntity, err = processCache.GetOrCache(ctx, "processNotExecutableTest", "1")

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorProcessModel, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)

		// when
		cachedEntity, err = processCache.GetOrCacheById(ctx, entity.Id)

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorProcessModel, engineErr.Type)
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
		w, cancel := pgEngine.withTimeout()
		defer cancel()

		ctx1, err := w.require()
		if err != nil {
			t.Errorf("failed to borrow context: %v", err)
		}

		ctx2, err := w.require()
		if err != nil {
			t.Errorf("failed to borrow context: %v", err)
		}

		process1 := internal.ProcessEntity{
			BpmnProcessId: "a",
			BpmnXml:       "xmla",
			BpmnXmlMd5:    "xmlmd5a",
			CreatedAt:     ctx1.Time(),
			CreatedBy:     "test",
			Parallelism:   2,
			Tags:          pgtype.Text{String: `{"a": "b"}`, Valid: true},
			Version:       "1",
		}

		process2 := internal.ProcessEntity{
			BpmnProcessId: "a",
			BpmnXml:       "xmla",
			BpmnXmlMd5:    "xmlmd5a",
			CreatedAt:     ctx1.Time(),
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
			insertErr1 = ctx1.Processes().Insert(&process1)
			insert1Cancel()

			insertErr2 = ctx2.Processes().Insert(&process2)
			insert2Cancel()
		}()

		<-insert1Ctx.Done()

		if insertErr1 != nil {
			t.Errorf("failed to insert process 1: %v", insertErr1)
		}

		w.release(ctx1, insertErr1)

		<-insert2Ctx.Done()

		assert.Equal(pgx.ErrNoRows, insertErr2)

		w.release(ctx2, insertErr2)

		ctx, err := w.require()
		if err != nil {
			t.Errorf("failed to borrow context: %v", err)
		}

		defer w.release(ctx, nil)

		selectedProcess, err := ctx.Processes().Select(process1.Id)
		if err != nil {
			t.Errorf("failed to select process: %v", err)
		}

		assert.Equal(&process1, selectedProcess)
	})
}
