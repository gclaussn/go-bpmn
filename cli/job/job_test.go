package job

import (
	"bytes"
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestComplete(t *testing.T) {
	assert := assert.New(t)

	t.Run("no arguments", func(t *testing.T) {
		out, err := execute(context.Background(), "complete")
		assert.Contains(out, "Usage:")
		assert.Nil(err)
	})

	t.Run("help", func(t *testing.T) {
		out, err := execute(context.Background(), "complete", "help")
		assert.Contains(out, "Usage:")
		assert.Nil(err)
	})

	t.Run("help flag", func(t *testing.T) {
		out, err := execute(context.Background(), "complete", "-h")
		assert.Contains(out, "Usage:")
		assert.Nil(err)
	})
}

func TestFail(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"fail",

		"-p", "2026-07-16",
		"-i", "123",

		"--error", "an error",
		"--retry-limit", "2",
		"--retry-timer", "PT1H",

		// element variables
		"--ev", `x={"value": "a"}`,
		"--ev-encoding", "x=json",
		"--ev-encrypted", "x=true",
		"--ev", "y=1",
		"--ev-bpmn-element-id", "y=b",
		"--ev-encoding", "y=text",

		// process variables
		"--pv", `x={"value": "a"}`,
		"--pv-encoding", "x=json",
		"--pv-encrypted", "x=true",
		"--pv", "y=1",
		"--pv-encoding", "y=text",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.Nil(cmd.Completion)
	assert.Equal("an error", cmd.Error)
	assert.Equal(2, cmd.RetryLimit)
	assert.Equal("PT1H", cmd.RetryTimer.String())
	assert.Equal("test-worker", cmd.WorkerId)

	assert.Len(cmd.ElementVariables, 2)

	assert.Equal("x", cmd.ElementVariables[0].Name)
	assert.Empty(cmd.ElementVariables[0].BpmnElementId)
	assert.Equal("json", cmd.ElementVariables[0].Data.Encoding)
	assert.True(cmd.ElementVariables[0].Data.IsEncrypted)
	assert.Equal(`{"value": "a"}`, cmd.ElementVariables[0].Data.Value)

	assert.Equal("y", cmd.ElementVariables[1].Name)
	assert.Equal("b", cmd.ElementVariables[1].BpmnElementId)
	assert.Equal("text", cmd.ElementVariables[1].Data.Encoding)
	assert.False(cmd.ElementVariables[1].Data.IsEncrypted)
	assert.Equal("1", cmd.ElementVariables[1].Data.Value)

	assert.Len(cmd.ProcessVariables, 2)

	assert.Equal("x", cmd.ProcessVariables[0].Name)
	assert.Equal("json", cmd.ProcessVariables[0].Data.Encoding)
	assert.True(cmd.ProcessVariables[0].Data.IsEncrypted)
	assert.Equal(`{"value": "a"}`, cmd.ProcessVariables[0].Data.Value)

	assert.Equal("y", cmd.ProcessVariables[1].Name)
	assert.Equal("text", cmd.ProcessVariables[1].Data.Encoding)
	assert.False(cmd.ProcessVariables[1].Data.IsEncrypted)
	assert.Equal("1", cmd.ProcessVariables[1].Data.Value)
}

func TestLock(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"lock",

		"-p", "2026-07-16",
		"-i", "123",

		"--process-id", "3",
		"--process-id", "4",
		"--process-instance-id", "5",

		"--limit", "2",
	)

	assert.Nil(err)

	cmd := e.lockJobsCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.Len(cmd.ProcessIds, 2)
	assert.Equal(int32(3), cmd.ProcessIds[0])
	assert.Equal(int32(4), cmd.ProcessIds[1])
	assert.Equal(int32(5), cmd.ProcessInstanceId)
	assert.Equal(2, cmd.Limit)
	assert.Equal("test-worker", cmd.WorkerId)
}

func TestUnlock(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"unlock",

		"-p", "2026-07-16",
		"-i", "123",

		"--worker-id", "example-worker",
	)

	assert.Nil(err)

	cmd := e.unlockJobsCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.Equal("example-worker", cmd.WorkerId)
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

	options := e.query.options
	assert.Equal(2, options.Limit)
	assert.Equal(3, options.Offset)
}

type testEngine struct {
	engine.Engine

	completeJobCmd engine.CompleteJobCmd
	lockJobsCmd    engine.LockJobsCmd
	unlockJobsCmd  engine.UnlockJobsCmd

	query testQuery
}

func (e *testEngine) CompleteJob(ctx context.Context, cmd engine.CompleteJobCmd) (engine.Job, error) {
	e.completeJobCmd = cmd
	return engine.Job{}, nil
}

func (e *testEngine) CreateQuery() engine.Query {
	return &e.query
}

func (e *testEngine) LockJobs(ctx context.Context, cmd engine.LockJobsCmd) ([]engine.Job, error) {
	e.lockJobsCmd = cmd
	return nil, nil
}

func (e *testEngine) UnlockJobs(ctx context.Context, cmd engine.UnlockJobsCmd) (int, error) {
	e.unlockJobsCmd = cmd
	return 0, nil
}

type testQuery struct {
	engine.Query

	criteria engine.JobCriteria
	options  engine.QueryOptions
}

func (q *testQuery) QueryJobs(ctx context.Context, criteria engine.JobCriteria) ([]engine.Job, error) {
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
