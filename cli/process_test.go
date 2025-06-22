package cli

import (
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobCreate(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	triggerAt := time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond)

	t.Run("time", func(t *testing.T) {
		mustExecute(t, e, []string{
			"process",
			"create",
			"--bpmn-file",
			"../test/bpmn/event/timer-start.bpmn",
			"--bpmn-process-id",
			"timerStartTest",
			"--version",
			"1",
			"--time",
			"timerStartEvent=" + triggerAt.Format(time.RFC3339Nano),
		})

		results, err := e.Query(engine.TaskCriteria{Type: engine.TaskTriggerTimerEvent})
		require.NoError(err, "failed to query tasks")
		require.NotEmpty(results, "no tasks queried")

		task := results[len(results)-1].(engine.Task)
		assert.Equal(triggerAt, task.DueAt)
	})

	t.Run("time-cycle", func(t *testing.T) {
		now := time.Now().UTC()

		mustExecute(t, e, []string{
			"process",
			"create",
			"--bpmn-file",
			"../test/bpmn/event/timer-start.bpmn",
			"--bpmn-process-id",
			"timerStartTest",
			"--version",
			"2",
			"--time-cycle",
			"timerStartEvent=0 * * * *",
		})

		results, err := e.Query(engine.TaskCriteria{Type: engine.TaskTriggerTimerEvent})
		require.NoError(err, "failed to query tasks")
		require.NotEmpty(results, "no tasks queried")

		task := results[len(results)-1].(engine.Task)
		assert.Equal(now.Add(time.Hour).Truncate(time.Hour), task.DueAt)
	})

	t.Run("time-duration", func(t *testing.T) {
		mustExecute(t, e, []string{
			"process",
			"create",
			"--bpmn-file",
			"../test/bpmn/event/timer-start.bpmn",
			"--bpmn-process-id",
			"timerStartTest",
			"--version",
			"3",
			"--time-duration",
			"timerStartEvent=PT1H",
		})

		results, err := e.Query(engine.ProcessCriteria{})
		require.NoError(err, "failed to query processes")
		require.NotEmpty(results, "no processes queried")

		prcoess := results[len(results)-1].(engine.Process)

		results, err = e.Query(engine.TaskCriteria{ProcessId: prcoess.Id, Type: engine.TaskTriggerTimerEvent})
		require.NoError(err, "failed to query task")
		require.NotEmpty(results, "no task queried")

		task := results[0].(engine.Task)
		assert.Equal(prcoess.CreatedAt.Add(time.Hour), task.DueAt)
	})
}
