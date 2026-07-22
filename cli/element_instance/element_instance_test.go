package element_instance

import (
	"bytes"
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestGetVariables(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"get-variables",

		"-p", "2026-07-16",
		"-i", "123",

		"--exclude-parent-variables", "true",
		"-n", "a",
		"--name", "b",
	)

	assert.Nil(err)

	cmd := e.getElementVariablesCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.ElementInstanceId)

	assert.True(cmd.ExcludeParentVariables)
	assert.Len(cmd.Names, 2)
	assert.Equal("a", cmd.Names[0])
	assert.Equal("b", cmd.Names[1])
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
		"--variable-bpmn-element-id", "y=b",
		"--variable-encoding", "y=text",
	)

	assert.Nil(err)

	cmd := e.setElementVariablesCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.ElementInstanceId)

	assert.Equal("test-worker", cmd.WorkerId)

	assert.Len(cmd.Variables, 2)

	assert.Equal("x", cmd.Variables[0].Name)
	assert.Empty(cmd.Variables[0].BpmnElementId)
	assert.Equal("json", cmd.Variables[0].Data.Encoding)
	assert.True(cmd.Variables[0].Data.IsEncrypted)
	assert.Equal(`{"value": "a"}`, cmd.Variables[0].Data.Value)

	assert.Equal("y", cmd.Variables[1].Name)
	assert.Equal("b", cmd.Variables[1].BpmnElementId)
	assert.Equal("text", cmd.Variables[1].Data.Encoding)
	assert.False(cmd.Variables[1].Data.IsEncrypted)
	assert.Equal("1", cmd.Variables[1].Data.Value)
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

		"--process-id", "20",
		"--process-instance-id", "200",

		"--bpmn-element-id", "element-x",
		"--state", "STARTED",
		"--state", "COMPLETED",

		"--limit", "2",
		"--offset", "3",
	)

	assert.Nil(err)

	criteria := e.query.criteria
	assert.Equal("2026-07-16", criteria.Partition.String())
	assert.Equal(int32(123), criteria.Id)

	assert.Equal(int32(122), criteria.ParentId)

	assert.Equal(int32(20), criteria.ProcessId)
	assert.Equal(int32(200), criteria.ProcessInstanceId)

	assert.Equal("element-x", criteria.BpmnElementId)
	assert.Len(criteria.States, 2)
	assert.Equal(engine.InstanceStarted.String(), criteria.States[0].String())
	assert.Equal(engine.InstanceCompleted.String(), criteria.States[1].String())

	options := e.query.options
	assert.Equal(2, options.Limit)
	assert.Equal(3, options.Offset)
}

type testEngine struct {
	engine.Engine

	getElementVariablesCmd engine.GetElementVariablesCmd
	setElementVariablesCmd engine.SetElementVariablesCmd

	query testQuery
}

func (e *testEngine) CreateQuery() engine.Query {
	return &e.query
}

func (e *testEngine) GetElementVariables(ctx context.Context, cmd engine.GetElementVariablesCmd) ([]engine.ElementVariable, error) {
	e.getElementVariablesCmd = cmd
	return nil, nil
}

func (e *testEngine) SetElementVariables(ctx context.Context, cmd engine.SetElementVariablesCmd) error {
	e.setElementVariablesCmd = cmd
	return nil
}

type testQuery struct {
	engine.Query

	criteria engine.ElementInstanceCriteria
	options  engine.QueryOptions
}

func (q *testQuery) QueryElementInstances(ctx context.Context, criteria engine.ElementInstanceCriteria) ([]engine.ElementInstance, error) {
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
