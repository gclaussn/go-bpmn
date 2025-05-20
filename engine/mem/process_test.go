package mem

import (
	"testing"

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

		ctx := memEngine.wlock()
		defer memEngine.unlock()

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

		ctx := memEngine.wlock()
		defer memEngine.unlock()

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

		ctx := memEngine.wlock()
		defer memEngine.unlock()

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
