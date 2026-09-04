package mem

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/stretchr/testify/assert"
)

// !keep in sync with engine/pg/process_test.go
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

	memEngine := e.(*memEngine)

	processCache := memEngine.ctx.processCache

	t.Run("get", func(t *testing.T) {
		ctx := memEngine.wlock()
		defer memEngine.unlock()

		// when
		cachedEntity, ok = processCache.Get(ctx, "startEndTest", "1")

		// then
		assert.Nil(cachedEntity)
		assert.False(ok)

		// when
		cachedEntity, ok = processCache.GetById(ctx, 1)

		// then
		assert.Nil(cachedEntity)
		assert.False(ok)
	})

	t.Run("get or cache when empty", func(t *testing.T) {
		ctx := memEngine.wlock()
		defer memEngine.unlock()

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
		_, err = e.CreateProcess(context.Background(), engine.CreateProcessCmd{
			BpmnProcessId: "startEndTest",
			BpmnXml:       mustReadBpmnFile(t, "start-end.bpmn"),
			Version:       "1",
			WorkerId:      "test-worker",
		})
		if err != nil {
			t.Fatalf("failed to create process: %v", err)
		}

		ctx := memEngine.wlock()
		defer memEngine.unlock()

		// when
		cachedEntity, ok = processCache.Get(ctx, "startEndTest", "1")

		// then
		assert.NotNil(cachedEntity)
		assert.True(ok)

		// when
		cachedEntity, ok = processCache.GetById(ctx, 1)

		// then
		assert.NotNil(cachedEntity)
		assert.True(ok)

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

	t.Run("cache returns error when process element not exists", func(t *testing.T) {
		// given
		entity := &internal.ProcessEntity{
			BpmnProcessId: "not-existing",
			BpmnXml:       mustReadBpmnFile(t, "start-end.bpmn"),
			Version:       "1",
		}

		mustInsertEntities(t, e, []any{entity})

		ctx := memEngine.wlock()
		defer memEngine.unlock()

		// when
		cachedEntity, err = processCache.GetOrCache(ctx, "not-existing", "1")

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorBug, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)

		// when
		cachedEntity, err = processCache.GetOrCacheById(ctx, entity.Id)

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

		ctx := memEngine.wlock()
		defer memEngine.unlock()

		// when
		cachedEntity, err = processCache.GetOrCache(ctx, "processNotExecutableTest", "1")

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorBug, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)

		// when
		cachedEntity, err = processCache.GetOrCacheById(ctx, entity.Id)

		// then
		assert.Nil(cachedEntity)
		assert.NotNil(err)

		engineErr = err.(engine.Error)
		assert.Equal(engine.ErrorBug, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)
	})
}

func TestProcessCacheCapacity(t *testing.T) {
	assert := assert.New(t)

	var (
		cachedEntity *internal.ProcessEntity
		ok           bool
	)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	memEngine := e.(*memEngine)

	ctx := memEngine.wlock()
	defer memEngine.unlock()

	processCache := internal.NewProcessCache(2, time.Hour)

	// when capacity is reached
	processCache.Put(ctx, &internal.ProcessEntity{Id: 1, BpmnProcessId: "a", Version: "1"})
	processCache.Put(ctx, &internal.ProcessEntity{Id: 2, BpmnProcessId: "b", Version: "2"})
	processCache.Put(ctx, &internal.ProcessEntity{Id: 3, BpmnProcessId: "c", Version: "3"})

	// then oldest entry is removed
	assert.Equal(2, processCache.Size())

	cachedEntity, ok = processCache.Get(ctx, "a", "1")
	assert.Nil(cachedEntity)
	assert.False(ok)

	cachedEntity, ok = processCache.GetById(ctx, 1)
	assert.Nil(cachedEntity)
	assert.False(ok)

	// when get
	processCache.Get(ctx, "b", "2")
	processCache.Put(ctx, &internal.ProcessEntity{Id: 4, BpmnProcessId: "d", Version: "4"})

	cachedEntity, ok = processCache.GetById(ctx, 3)

	// then entry is moved to the front
	assert.Equal(2, processCache.Size())

	assert.Nil(cachedEntity)
	assert.False(ok)

	// when get by ID
	processCache.GetById(ctx, 4)
	processCache.Put(ctx, &internal.ProcessEntity{Id: 5, BpmnProcessId: "e", Version: "5"})

	cachedEntity, ok = processCache.GetById(ctx, 2)

	// then entry is moved to the front
	assert.Equal(2, processCache.Size())

	assert.Nil(cachedEntity)
	assert.False(ok)

	// when put again
	processCache.Put(ctx, &internal.ProcessEntity{Id: 5, BpmnProcessId: "e", Version: "5", Parallelism: 1})

	// then entry is updated
	assert.Equal(2, processCache.Size())

	cachedEntity, _ = processCache.GetById(ctx, 5)
	assert.Equal(1, cachedEntity.Parallelism)
}

func TestProcessCacheExpiration(t *testing.T) {
	assert := assert.New(t)

	var (
		cachedEntity *internal.ProcessEntity
		ok           bool
	)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	memEngine := e.(*memEngine)

	ctx := memEngine.wlock()
	defer memEngine.unlock()

	putAt := ctx.Time()

	processCache := internal.NewProcessCache(2, time.Hour)

	processCache.Put(ctx, &internal.ProcessEntity{Id: 1, BpmnProcessId: "a", Version: "1"})
	processCache.Put(ctx, &internal.ProcessEntity{Id: 2, BpmnProcessId: "b", Version: "2"})

	// when entry not expired
	ctx.time = putAt.Add(time.Hour).Add(-1 * time.Millisecond)
	cachedEntity, ok = processCache.Get(ctx, "a", "1")

	// then
	assert.NotNil(cachedEntity)
	assert.True(ok)

	assert.Equal(2, processCache.Size())

	// when entry expired
	ctx.time = putAt.Add(time.Hour)
	cachedEntity, ok = processCache.Get(ctx, "a", "1")

	// then
	assert.Nil(cachedEntity)
	assert.False(ok)

	assert.Equal(1, processCache.Size())

	// when entry not expired
	ctx.time = putAt.Add(time.Hour).Add(-1 * time.Millisecond)
	cachedEntity, ok = processCache.GetById(ctx, 2)

	// then
	assert.NotNil(cachedEntity)
	assert.True(ok)

	assert.Equal(1, processCache.Size())

	// when entry expired
	ctx.time = putAt.Add(time.Hour)
	cachedEntity, ok = processCache.GetById(ctx, 2)

	// then
	assert.Nil(cachedEntity)
	assert.False(ok)

	assert.Equal(0, processCache.Size())
}
