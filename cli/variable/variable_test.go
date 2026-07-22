package variable

import (
	"bytes"
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestQuery(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"query",

		"-p", "2026-07-16",

		"--element-instance-id", "100",
		"--process-instance-id", "200",

		"-n", "a",
		"--name", "b",

		"--limit", "2",
		"--offset", "3",
	)

	assert.Nil(err)

	criteria := e.query.criteria
	assert.Equal("2026-07-16", criteria.Partition.String())

	assert.Equal(int32(100), criteria.ElementInstanceId)
	assert.Equal(int32(200), criteria.ProcessInstanceId)

	assert.Len(criteria.Names, 2)
	assert.Equal("a", criteria.Names[0])
	assert.Equal("b", criteria.Names[1])

	options := e.query.options
	assert.Equal(2, options.Limit)
	assert.Equal(3, options.Offset)
}

type testEngine struct {
	engine.Engine

	query testQuery
}

func (e *testEngine) CreateQuery() engine.Query {
	return &e.query
}

type testQuery struct {
	engine.Query

	criteria engine.VariableCriteria
	options  engine.QueryOptions
}

func (q *testQuery) QueryVariables(ctx context.Context, criteria engine.VariableCriteria) ([]engine.Variable, error) {
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
