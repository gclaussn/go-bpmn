package cli

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/require"
)

func TestSendSignal(t *testing.T) {
	require := require.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	mustExecute(t, e, []string{
		"process",
		"create",
		"--bpmn-file",
		"../test/bpmn/event/signal-start.bpmn",
		"--bpmn-process-id",
		"signalStartTest",
		"--version",
		"1",
		"--signal-name",
		"signalStartEvent=start-signal",
	})

	mustExecute(t, e, []string{
		"event",
		"send-signal",
		"--name",
		"start-signal",
	})

	results, err := e.CreateQuery().QueryTasks(context.Background(), engine.TaskCriteria{Type: engine.TaskTriggerEvent})
	require.NoError(err, "failed to query tasks")
	require.NotEmpty(results, "no tasks queried")
}
