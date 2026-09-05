package process

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestCreate(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	now := time.Now().UTC().Truncate(time.Millisecond)

	_, err := execute(ctx,
		"create",

		"--bpmn-file", "../../test/bpmn/start-end.bpmn",
		"--bpmn-process-id", "startEndTest",
		"--parallelism", "3",
		"-t", "a=b",
		"--tag", "x=y",
		"--version", "v1",

		"--error", "error1=error-code1",
		"--error", "error2=error-code2",
		"--escalation", "escalation1=escalation-code1",
		"--escalation", "escalation2=escalation-code2",
		"--message", "message1=message-name1",
		"--message", "message2=message-name2",
		"--signal", "signal1=signal-name1",
		"--signal", "signal2=signal-name2",
		"--time", "timer1="+now.Format(time.RFC3339Nano),
		"--time-cycle", "timer2=0 * * * *",
		"--time-duration", "timer3=P1D",
	)

	assert.Nil(err)

	cmd := e.createProcessCmd
	assert.Equal("startEndTest", cmd.BpmnProcessId)
	assert.Contains(cmd.BpmnXml, "startEndTest")
	assert.Equal(3, cmd.Parallelism)
	assert.Len(cmd.Tags, 2)
	assert.Equal("a", cmd.Tags[0].Name)
	assert.Equal("b", cmd.Tags[0].Value)
	assert.Equal("x", cmd.Tags[1].Name)
	assert.Equal("y", cmd.Tags[1].Value)
	assert.Equal("v1", cmd.Version)
	assert.Equal("test-worker", cmd.WorkerId)

	assert.Len(cmd.Errors, 2)
	assert.Equal("error1", cmd.Errors[0].BpmnElementId)
	assert.Equal("error-code1", cmd.Errors[0].ErrorCode)
	assert.Equal("error2", cmd.Errors[1].BpmnElementId)
	assert.Equal("error-code2", cmd.Errors[1].ErrorCode)

	assert.Len(cmd.Escalations, 2)
	assert.Equal("escalation1", cmd.Escalations[0].BpmnElementId)
	assert.Equal("escalation-code1", cmd.Escalations[0].EscalationCode)
	assert.Equal("escalation2", cmd.Escalations[1].BpmnElementId)
	assert.Equal("escalation-code2", cmd.Escalations[1].EscalationCode)

	assert.Len(cmd.Messages, 2)
	assert.Equal("message1", cmd.Messages[0].BpmnElementId)
	assert.Equal("message-name1", cmd.Messages[0].MessageName)
	assert.Equal("message2", cmd.Messages[1].BpmnElementId)
	assert.Equal("message-name2", cmd.Messages[1].MessageName)

	assert.Len(cmd.Signals, 2)
	assert.Equal("signal1", cmd.Signals[0].BpmnElementId)
	assert.Equal("signal-name1", cmd.Signals[0].SignalName)
	assert.Equal("signal2", cmd.Signals[1].BpmnElementId)
	assert.Equal("signal-name2", cmd.Signals[1].SignalName)

	assert.Len(cmd.Timers, 3)
	assert.Equal("timer1", cmd.Timers[0].BpmnElementId)
	assert.Equal(now, cmd.Timers[0].Timer.Time)
	assert.Empty(cmd.Timers[0].Timer.TimeCycle)
	assert.True(cmd.Timers[0].Timer.TimeDuration.IsZero())
	assert.Equal("timer2", cmd.Timers[1].BpmnElementId)
	assert.Zero(cmd.Timers[1].Timer.Time)
	assert.Equal("0 * * * *", cmd.Timers[1].Timer.TimeCycle)
	assert.True(cmd.Timers[1].Timer.TimeDuration.IsZero())
	assert.Equal("timer3", cmd.Timers[2].BpmnElementId)
	assert.Zero(cmd.Timers[2].Timer.Time)
	assert.Empty(cmd.Timers[2].Timer.TimeCycle)
	assert.Equal("P1D", cmd.Timers[2].Timer.TimeDuration.String())
}

func TestGetBpmnXml(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"get-bpmn-xml",

		"-i", "123",
	)

	assert.Nil(err)

	cmd := e.getBpmnXmlCmd
	assert.Equal(int32(123), cmd.ProcessId)
}

func TestQuery(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "")

	_, err := execute(ctx,
		"query",

		"-i", "123",

		"-t", "a=b",
		"--tag", "x=y",

		"--limit", "2",
		"--offset", "3",
	)

	assert.Nil(err)

	criteria := e.query.criteria
	assert.Equal(int32(123), criteria.Id)

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

	createProcessCmd engine.CreateProcessCmd
	getBpmnXmlCmd    engine.GetBpmnXmlCmd

	query testQuery
}

func (e *testEngine) CreateProcess(ctx context.Context, cmd engine.CreateProcessCmd) (engine.Process, error) {
	e.createProcessCmd = cmd
	return engine.Process{}, nil
}

func (e *testEngine) CreateQuery() engine.Query {
	return &e.query
}

func (e *testEngine) GetBpmnXml(ctx context.Context, cmd engine.GetBpmnXmlCmd) (string, error) {
	e.getBpmnXmlCmd = cmd
	return "", nil
}

type testQuery struct {
	engine.Query

	criteria engine.ProcessCriteria
	options  engine.QueryOptions
}

func (q *testQuery) QueryProcesses(ctx context.Context, criteria engine.ProcessCriteria) ([]engine.Process, error) {
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
