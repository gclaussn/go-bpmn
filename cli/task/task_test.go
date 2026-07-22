package task

import (
	"bytes"
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestExecute(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"execute",

		"-p", "2026-07-16",
		"-i", "123",

		"--process-id", "3",
		"--process-instance-id", "5",
		"--type", "TRIGGER_EVENT",

		"--limit", "2",
	)

	assert.Nil(err)

	cmd := e.executeTasksCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.Equal(int32(3), cmd.ProcessId)
	assert.Equal(int32(5), cmd.ProcessInstanceId)
	assert.Equal(engine.TaskTriggerEvent.String(), cmd.Type.String())
	assert.Equal(2, cmd.Limit)
}

func TestUnlock(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"unlock",

		"-p", "2026-07-16",
		"-i", "123",

		"--engine-id", "example-engine",
	)

	assert.Nil(err)

	cmd := e.unlockTasksCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.Equal("example-engine", cmd.EngineId)
}

func TestQuery(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"query",

		"-p", "2026-07-16",
		"-i", "123",

		"--element-id", "10",
		"--element-instance-id", "100",
		"--process-id", "20",
		"--process-instance-id", "200",

		"--type", "START_PROCESS_INSTANCE",

		"--limit", "2",
		"--offset", "3",
	)

	assert.Nil(err)

	criteria := e.query.criteria
	assert.Equal("2026-07-16", criteria.Partition.String())
	assert.Equal(int32(123), criteria.Id)

	assert.Equal(int32(10), criteria.ElementId)
	assert.Equal(int32(100), criteria.ElementInstanceId)
	assert.Equal(int32(20), criteria.ProcessId)
	assert.Equal(int32(200), criteria.ProcessInstanceId)

	assert.Equal(engine.TaskStartProcessInstance.String(), criteria.Type.String())

	options := e.query.options
	assert.Equal(2, options.Limit)
	assert.Equal(3, options.Offset)
}

type testEngine struct {
	engine.Engine

	executeTasksCmd engine.ExecuteTasksCmd
	unlockTasksCmd  engine.UnlockTasksCmd

	query testQuery
}

func (e *testEngine) CreateQuery() engine.Query {
	return &e.query
}

func (e *testEngine) ExecuteTasks(ctx context.Context, cmd engine.ExecuteTasksCmd) ([]engine.Task, []engine.Task, error) {
	e.executeTasksCmd = cmd
	return nil, nil, nil
}

func (e *testEngine) UnlockTasks(ctx context.Context, cmd engine.UnlockTasksCmd) (int, error) {
	e.unlockTasksCmd = cmd
	return 0, nil
}

type testQuery struct {
	engine.Query

	criteria engine.TaskCriteria
	options  engine.QueryOptions
}

func (q *testQuery) QueryTasks(ctx context.Context, criteria engine.TaskCriteria) ([]engine.Task, error) {
	q.criteria = criteria
	return nil, nil
}

func (q *testQuery) SetOptions(options engine.QueryOptions) {
	q.options = options
}

func execute(ctx context.Context, args ...string) (string, error) {
	buffer := bytes.NewBufferString("")

	rootCmd := NewCmd()
	rootCmd.SetArgs(args)
	rootCmd.SetErr(buffer)
	rootCmd.SetOut(buffer)

	err := rootCmd.ExecuteContext(ctx)

	return buffer.String(), err
}
