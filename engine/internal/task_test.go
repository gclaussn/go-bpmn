package internal

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

type testTaskExecutorEngine struct {
	engine.Engine

	executeTasksCalled int
}

func (e *testTaskExecutorEngine) ExecuteTasks(ctx context.Context, cmd engine.ExecuteTasksCmd) ([]engine.Task, []engine.Task, error) {
	e.executeTasksCalled++
	return nil, nil, nil
}

func TestTaskExecutor(t *testing.T) {
	assert := assert.New(t)

	// given
	e := &testTaskExecutorEngine{}

	executor := NewTaskExecutor(e, time.Millisecond*100, 1)

	// when
	executor.Execute()
	time.Sleep(time.Millisecond * 350)
	executor.Stop()

	// then
	assert.Equal(3, e.executeTasksCalled)
}
