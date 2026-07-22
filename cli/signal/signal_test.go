package signal

import (
	"bytes"
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestSend(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"send",

		"--name", "test-name",

		"--variable", `x={"value": "a"}`,
		"--variable-encoding", "x=json",
		"--variable-encrypted", "x=true",
		"--variable", "y=1",
		"--variable-encoding", "y=text",
	)

	assert.Nil(err)

	cmd := e.sendSignalCmd
	assert.Equal("test-name", cmd.Name)
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

func TestQuerySubscriptions(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"query-subscriptions",

		"-p", "2026-07-16",

		"--process-instance-id", "200",

		"--name", "test-name",

		"--limit", "2",
		"--offset", "3",
	)

	assert.Nil(err)

	criteria := e.query.subscriptionCriteria
	assert.Equal("2026-07-16", criteria.Partition.String())

	assert.Equal(int32(200), criteria.ProcessInstanceId)

	assert.Equal("test-name", criteria.Name)

	options := e.query.options
	assert.Equal(2, options.Limit)
	assert.Equal(3, options.Offset)
}

type testEngine struct {
	engine.Engine

	sendSignalCmd engine.SendSignalCmd

	query testQuery
}

func (e *testEngine) CreateQuery() engine.Query {
	return &e.query
}

func (e *testEngine) SendSignal(ctx context.Context, cmd engine.SendSignalCmd) (engine.Signal, error) {
	e.sendSignalCmd = cmd
	return engine.Signal{}, nil
}

type testQuery struct {
	engine.Query

	subscriptionCriteria engine.SignalSubscriptionCriteria

	options engine.QueryOptions
}

func (q *testQuery) QuerySignalSubscriptions(ctx context.Context, criteria engine.SignalSubscriptionCriteria) ([]engine.SignalSubscription, error) {
	q.subscriptionCriteria = criteria
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
