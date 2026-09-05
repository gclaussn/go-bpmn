package job

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/stretchr/testify/assert"
)

func TestCallProcess(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "call-process",

		"-p", "2026-07-16",
		"-i", "123",

		"--bpmn-process-id", "test-bpmn-process-id",
		"--correlation-key", "test-correlation-key",
		"--tag", "a=b",
		"--tag", "x=y",
		"--version", "v1",

		// variables
		"--variable", `x={"value": "a"}`,
		"--variable-encoding", "x=json",
		"--variable-encrypted", "x=true",
		"--variable", "y=1",
		"--variable-encoding", "y=text",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.NotNil(completion.CalledProcess)

	calledProcess := completion.CalledProcess
	assert.Equal("test-bpmn-process-id", calledProcess.BpmnProcessId)
	assert.Equal("test-correlation-key", calledProcess.CorrelationKey)
	assert.Len(calledProcess.Tags, 2)
	assert.Equal("a", calledProcess.Tags[0].Name)
	assert.Equal("b", calledProcess.Tags[0].Value)
	assert.Equal("x", calledProcess.Tags[1].Name)
	assert.Equal("y", calledProcess.Tags[1].Value)
	assert.Equal("v1", calledProcess.Version)

	assert.Len(calledProcess.Variables, 2)

	assert.Equal("x", calledProcess.Variables[0].Name)
	assert.Equal("json", calledProcess.Variables[0].Data.Encoding)
	assert.True(calledProcess.Variables[0].Data.IsEncrypted)
	assert.Equal(`{"value": "a"}`, calledProcess.Variables[0].Data.Value)

	assert.Equal("y", calledProcess.Variables[1].Name)
	assert.Equal("text", calledProcess.Variables[1].Data.Encoding)
	assert.False(calledProcess.Variables[1].Data.IsEncrypted)
	assert.Equal("1", calledProcess.Variables[1].Data.Value)
}

func TestEvaluateExclusiveGateway(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "evaluate-exclusive-gateway",

		"-p", "2026-07-16",
		"-i", "123",

		"--decision", "element-a",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.Equal("element-a", completion.ExclusiveGatewayDecision)
}

func TestEvaluateInclusiveGateway(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "evaluate-inclusive-gateway",

		"-p", "2026-07-16",
		"-i", "123",

		"--decision", "element-b",
		"--decision", "element-c",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.Len(completion.InclusiveGatewayDecision, 2)
	assert.Equal("element-b", completion.InclusiveGatewayDecision[0])
	assert.Equal("element-c", completion.InclusiveGatewayDecision[1])
}

func TestExecute(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "execute",

		"-p", "2026-07-16",
		"-i", "123",

		"--error-code", "test-error",
		"--escalation-code", "test-escalation",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.Equal("test-error", completion.ErrorCode)
	assert.Equal("test-escalation", completion.EscalationCode)
}

func TestPassVariables(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "pass-variables",

		"-p", "2026-07-16",
		"-i", "123",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.Nil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)
}

func TestSetErrorCode(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "set-error-code",

		"-p", "2026-07-16",
		"-i", "123",

		"--code", "test-error",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.Equal("test-error", completion.ErrorCode)
}

func TestSetEscalationCode(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "set-escalation-code",

		"-p", "2026-07-16",
		"-i", "123",

		"--code", "test-escalation",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.Equal("test-escalation", completion.EscalationCode)
}

func TestSetMessageCorrelationKey(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "set-message-correlation-key",

		"-p", "2026-07-16",
		"-i", "123",

		"--correlation-key", "test-correlation-key",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.Equal("test-correlation-key", completion.MessageCorrelationKey)
}

func TestSetSignalName(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "set-signal-name",

		"-p", "2026-07-16",
		"-i", "123",

		"--name", "test-signal",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.Equal("test-signal", completion.SignalName)
}

func TestSetTimer(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	now := time.Now().UTC().Truncate(time.Millisecond)

	_, err := execute(ctx,
		"complete", "set-timer",

		"-p", "2026-07-16",
		"-i", "123",

		"--time", now.Format(time.RFC3339Nano),
		"--time-cycle", "0 * * * *",
		"--time-duration", "P1D",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.NotNil(completion.Timer)

	timer := completion.Timer
	assert.Equal(now, timer.Time)
	assert.Equal("0 * * * *", timer.TimeCycle)
	assert.Equal("P1D", timer.TimeDuration.String())
}

func TestSubscribeMessage(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "subscribe-message",

		"-p", "2026-07-16",
		"-i", "123",

		"--correlation-key", "test-correlation-key",
		"--name", "test-message",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.Equal("test-correlation-key", completion.MessageCorrelationKey)
	assert.Equal("test-message", completion.MessageName)
}

func TestSubscribeSignal(t *testing.T) {
	assert := assert.New(t)

	e := &testEngine{}

	ctx := common.SetEngineAndWorkerId(context.Background(), e, "test-worker")

	_, err := execute(ctx,
		"complete", "subscribe-signal",

		"-p", "2026-07-16",
		"-i", "123",

		"--name", "test-signal",
	)

	assert.Nil(err)

	cmd := e.completeJobCmd
	assert.Equal("2026-07-16", cmd.Partition.String())
	assert.Equal(int32(123), cmd.Id)

	assert.NotNil(cmd.Completion)
	assert.Equal("test-worker", cmd.WorkerId)

	completion := cmd.Completion
	assert.Equal("test-signal", completion.SignalName)
}
