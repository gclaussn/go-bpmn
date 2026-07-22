package user_task

import (
	"bytes"
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestUpdate(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"update",

		"-p", "2026-07-16",
		"-i", "123",

		"-r", "9",

		"--completed", "true",
		"--error-code", "test-error",
		"--escalation-code", "test-escalation",
		"-t", "a=b",
		"--tag", "x=y",

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

	cmd := e.updateUserTaskCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.Equal(int32(9), cmd.Revision)

	assert.True(cmd.IsCompleted)
	assert.Equal("test-error", cmd.ErrorCode)
	assert.Equal("test-escalation", cmd.EscalationCode)
	assert.Len(cmd.Tags, 2)
	assert.Equal("a", cmd.Tags[0].Name)
	assert.Equal("b", cmd.Tags[0].Value)
	assert.Equal("x", cmd.Tags[1].Name)
	assert.Equal("y", cmd.Tags[1].Value)
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

func TestQuery(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"query",

		"-p", "2026-07-16",
		"-i", "123",

		"--element-instance-id", "100",
		"--process-id", "20",
		"--process-instance-id", "200",

		"-t", "a=b",
		"--tag", "x=y",

		"--limit", "2",
		"--offset", "3",
	)

	assert.Nil(err)

	criteria := e.query.criteria
	assert.Equal("2026-07-16", criteria.Partition.String())
	assert.Equal(int32(123), criteria.Id)

	assert.Equal(int32(100), criteria.ElementInstanceId)
	assert.Equal(int32(20), criteria.ProcessId)
	assert.Equal(int32(200), criteria.ProcessInstanceId)

	assert.Len(criteria.Tags, 2)
	assert.Equal("a", criteria.Tags[0].Name)
	assert.Equal("b", criteria.Tags[0].Value)
	assert.Equal("x", criteria.Tags[1].Name)
	assert.Equal("y", criteria.Tags[1].Value)

	options := e.query.options
	assert.Equal(2, options.Limit)
	assert.Equal(3, options.Offset)
}

type testEngine struct {
	engine.Engine

	updateUserTaskCmd engine.UpdateUserTaskCmd

	query testQuery
}

func (e *testEngine) CreateQuery() engine.Query {
	return &e.query
}

func (e *testEngine) UpdateUserTask(ctx context.Context, cmd engine.UpdateUserTaskCmd) (engine.UserTask, error) {
	e.updateUserTaskCmd = cmd
	return engine.UserTask{}, nil
}

type testQuery struct {
	engine.Query

	criteria engine.UserTaskCriteria
	options  engine.QueryOptions
}

func (q *testQuery) QueryUserTasks(ctx context.Context, criteria engine.UserTaskCriteria) ([]engine.UserTask, error) {
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
