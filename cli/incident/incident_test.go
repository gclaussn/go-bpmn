package incident

import (
	"bytes"
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestResolve(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"resolve",

		"-p", "2026-07-16",
		"-i", "123",

		"--retry-timer", "PT1H",
	)

	assert.Nil(err)

	cmd := e.resolveIncidentCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.Equal("PT1H", cmd.RetryTimer.String())
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

		"--job-id", "10",
		"--process-instance-id", "200",
		"--task-id", "30",

		"--limit", "2",
		"--offset", "3",
	)

	assert.Nil(err)

	criteria := e.query.criteria
	assert.Equal("2026-07-16", criteria.Partition.String())
	assert.Equal(int32(123), criteria.Id)

	assert.Equal(int32(10), criteria.JobId)
	assert.Equal(int32(200), criteria.ProcessInstanceId)
	assert.Equal(int32(30), criteria.TaskId)

	options := e.query.options
	assert.Equal(2, options.Limit)
	assert.Equal(3, options.Offset)
}

type testEngine struct {
	engine.Engine

	resolveIncidentCmd engine.ResolveIncidentCmd

	query testQuery
}

func (e *testEngine) CreateQuery() engine.Query {
	return &e.query
}

func (e *testEngine) ResolveIncident(ctx context.Context, cmd engine.ResolveIncidentCmd) error {
	e.resolveIncidentCmd = cmd
	return nil
}

type testQuery struct {
	engine.Query

	criteria engine.IncidentCriteria
	options  engine.QueryOptions
}

func (q *testQuery) QueryIncidents(ctx context.Context, criteria engine.IncidentCriteria) ([]engine.Incident, error) {
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
