package message

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestSend(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	now := time.Now().UTC().Truncate(time.Millisecond)

	_, err := execute(ctx,
		"send",

		"--correlation-key", "test-correlation-key",
		"--expiration-time", now.Format(time.RFC3339Nano),
		"--expiration-time-cycle", "0 * * * *",
		"--expiration-time-duration", "P1D",
		"--name", "test-name",
		"--unique-key", "test-unique-key",

		"--variable", `x={"value": "a"}`,
		"--variable-encoding", "x=json",
		"--variable-encrypted", "x=true",
		"--variable", "y=1",
		"--variable-encoding", "y=text",
	)

	assert.Nil(err)

	cmd := e.sendMessageCmd
	assert.Equal("test-correlation-key", cmd.CorrelationKey)
	assert.NotNil(cmd.ExpirationTimer)
	assert.Equal(now, cmd.ExpirationTimer.Time)
	assert.Equal("0 * * * *", cmd.ExpirationTimer.TimeCycle)
	assert.Equal("P1D", cmd.ExpirationTimer.TimeDuration.String())
	assert.Equal("test-name", cmd.Name)
	assert.Equal("test-unique-key", cmd.UniqueKey)
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

func TestQuery(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"query",

		"-i", "123",

		"--exclude-expired", "true",
		"--name", "test-name",

		"--limit", "2",
		"--offset", "3",
	)

	assert.Nil(err)

	criteria := e.query.criteria
	assert.Equal(int64(123), criteria.Id)

	assert.True(criteria.ExcludeExpired)
	assert.Equal("test-name", criteria.Name)

	options := e.query.options
	assert.Equal(2, options.Limit)
	assert.Equal(3, options.Offset)
}

func TestQuerySubscriptions(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"query-subscriptions",

		"-p", "2026-07-16",

		"--process-instance-id", "200",

		"--correlation-key", "test-correlation-key",
		"--name", "test-name",

		"--limit", "2",
		"--offset", "3",
	)

	assert.Nil(err)

	criteria := e.query.subscriptionCriteria
	assert.Equal("2026-07-16", criteria.Partition.String())

	assert.Equal(int32(200), criteria.ProcessInstanceId)

	assert.Equal("test-correlation-key", criteria.CorrelationKey)
	assert.Equal("test-name", criteria.Name)

	options := e.query.options
	assert.Equal(2, options.Limit)
	assert.Equal(3, options.Offset)
}

type testEngine struct {
	engine.Engine

	sendMessageCmd engine.SendMessageCmd

	query testQuery
}

func (e *testEngine) CreateQuery() engine.Query {
	return &e.query
}

func (e *testEngine) SendMessage(ctx context.Context, cmd engine.SendMessageCmd) (engine.Message, error) {
	e.sendMessageCmd = cmd
	return engine.Message{}, nil
}

type testQuery struct {
	engine.Query

	criteria             engine.MessageCriteria
	subscriptionCriteria engine.MessageSubscriptionCriteria

	options engine.QueryOptions
}

func (q *testQuery) QueryMessages(ctx context.Context, criteria engine.MessageCriteria) ([]engine.Message, error) {
	q.criteria = criteria
	return nil, nil
}

func (q *testQuery) QueryMessageSubscriptions(ctx context.Context, criteria engine.MessageSubscriptionCriteria) ([]engine.MessageSubscription, error) {
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
