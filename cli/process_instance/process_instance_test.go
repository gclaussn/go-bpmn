package process_instance

import (
	"bytes"
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestCreate(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"create",

		"--bpmn-process-id", "test-bpmn-process-id",
		"--correlation-key", "test-correlation-key",
		"-t", "a=b",
		"--tag", "x=y",
		"--version", "v1",

		"--variable", `x={"value": "a"}`,
		"--variable-encoding", "x=json",
		"--variable-encrypted", "x=true",
		"--variable", "y=1",
		"--variable-encoding", "y=text",
	)

	assert.Nil(err)

	cmd := e.createProcessInstanceCmd
	assert.Equal("test-bpmn-process-id", cmd.BpmnProcessId)
	assert.Equal("test-correlation-key", cmd.CorrelationKey)
	assert.Len(cmd.Tags, 2)
	assert.Equal("a", cmd.Tags[0].Name)
	assert.Equal("b", cmd.Tags[0].Value)
	assert.Equal("x", cmd.Tags[1].Name)
	assert.Equal("y", cmd.Tags[1].Value)
	assert.Equal("v1", cmd.Version)
	assert.Equal("test-worker", cmd.WorkerId)

	assert.Len(cmd.Variables, 2)

	assert.Equal("x", cmd.Variables[0].Name)
	assert.Equal("json", cmd.Variables[0].Data.Encoding)
	assert.True(cmd.Variables[0].Data.IsEncrypted)
	assert.Equal(`{"value": "a"}`, cmd.Variables[0].Data.Value)

	assert.Equal("y", cmd.Variables[1].Name)
	assert.Equal("text", cmd.Variables[1].Data.Encoding)
	assert.False(cmd.Variables[1].Data.IsEncrypted)
	assert.Equal("1", cmd.Variables[1].Data.Value)
}

func TestGetVariables(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"get-variables",

		"-p", "2026-07-16",
		"-i", "123",

		"-n", "a",
		"--name", "b",
	)

	assert.Nil(err)

	cmd := e.getProcessVariablesCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.ProcessInstanceId)

	assert.Len(cmd.Names, 2)
	assert.Equal("a", cmd.Names[0])
	assert.Equal("b", cmd.Names[1])
}

func TestResume(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"resume",

		"-p", "2026-07-16",
		"-i", "123",
	)

	assert.Nil(err)

	cmd := e.resumeProcessInstanceCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.Equal("test-worker", cmd.WorkerId)
}

func TestSetVariables(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"set-variables",

		"-p", "2026-07-16",
		"-i", "123",

		"--variable", `x={"value": "a"}`,
		"--variable-encoding", "x=json",
		"--variable-encrypted", "x=true",
		"--variable", "y=1",
		"--variable-encoding", "y=text",
	)

	assert.Nil(err)

	cmd := e.setProcessVariablesCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.ProcessInstanceId)

	assert.Equal("test-worker", cmd.WorkerId)

	assert.Len(cmd.Variables, 2)

	assert.Equal("x", cmd.Variables[0].Name)
	assert.Equal("json", cmd.Variables[0].Data.Encoding)
	assert.True(cmd.Variables[0].Data.IsEncrypted)
	assert.Equal(`{"value": "a"}`, cmd.Variables[0].Data.Value)

	assert.Equal("y", cmd.Variables[1].Name)
	assert.Equal("text", cmd.Variables[1].Data.Encoding)
	assert.False(cmd.Variables[1].Data.IsEncrypted)
	assert.Equal("1", cmd.Variables[1].Data.Value)
}

func TestSuspend(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"suspend",

		"-p", "2026-07-16",
		"-i", "123",
	)

	assert.Nil(err)

	cmd := e.suspendProcessInstanceCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.Equal("test-worker", cmd.WorkerId)
}

func TestQuery(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"query",

		"-p", "2026-07-16",
		"-i", "123",

		"--parent-id", "122",
		"--root-id", "121",

		"--process-id", "20",

		"-t", "a=b",
		"--tag", "x=y",

		"--limit", "2",
		"--offset", "3",
	)

	assert.Nil(err)

	criteria := e.query.criteria
	assert.Equal("2026-07-16", criteria.Partition.String())
	assert.Equal(int32(123), criteria.Id)

	assert.Equal(int32(122), criteria.ParentId)
	assert.Equal(int32(121), criteria.RootId)

	assert.Equal(int32(20), criteria.ProcessId)

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

	createProcessInstanceCmd  engine.CreateProcessInstanceCmd
	getProcessVariablesCmd    engine.GetProcessVariablesCmd
	resumeProcessInstanceCmd  engine.ResumeProcessInstanceCmd
	setProcessVariablesCmd    engine.SetProcessVariablesCmd
	suspendProcessInstanceCmd engine.SuspendProcessInstanceCmd

	query testQuery
}

func (e *testEngine) CreateProcessInstance(ctx context.Context, cmd engine.CreateProcessInstanceCmd) (engine.ProcessInstance, error) {
	e.createProcessInstanceCmd = cmd
	return engine.ProcessInstance{}, nil
}

func (e *testEngine) CreateQuery() engine.Query {
	return &e.query
}

func (e *testEngine) GetProcessVariables(ctx context.Context, cmd engine.GetProcessVariablesCmd) ([]engine.ProcessVariable, error) {
	e.getProcessVariablesCmd = cmd
	return nil, nil
}

func (e *testEngine) ResumeProcessInstance(ctx context.Context, cmd engine.ResumeProcessInstanceCmd) error {
	e.resumeProcessInstanceCmd = cmd
	return nil
}

func (e *testEngine) SetProcessVariables(ctx context.Context, cmd engine.SetProcessVariablesCmd) error {
	e.setProcessVariablesCmd = cmd
	return nil
}

func (e *testEngine) SuspendProcessInstance(ctx context.Context, cmd engine.SuspendProcessInstanceCmd) error {
	e.suspendProcessInstanceCmd = cmd
	return nil
}

type testQuery struct {
	engine.Query

	criteria engine.ProcessInstanceCriteria
	options  engine.QueryOptions
}

func (q *testQuery) QueryProcessInstances(ctx context.Context, criteria engine.ProcessInstanceCriteria) ([]engine.ProcessInstance, error) {
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
