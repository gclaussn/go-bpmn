package cli

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJobCompleteSetTimer(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	// given
	mustExecute(t, e, []string{
		"process",
		"create",
		"--bpmn-file",
		"../test/bpmn/event/timer-catch.bpmn",
		"--bpmn-process-id",
		"timerCatchTest",
		"--version",
		"1",
	})

	t.Run("time", func(t *testing.T) {
		mustExecute(t, e, []string{
			"process-instance",
			"create",
			"--bpmn-process-id",
			"timerCatchTest",
			"--version",
			"1",
		})

		lockedJobs, err := e.LockJobs(context.Background(), engine.LockJobsCmd{WorkerId: program})
		require.NoError(err, "failed to lock job")
		require.NotEmpty(lockedJobs, "no job locked")

		triggerAt := time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond)

		mustExecute(t, e, []string{
			"job",
			"complete",
			"--partition",
			lockedJobs[0].Partition.String(),
			"--id",
			strconv.Itoa(int(lockedJobs[0].Id)),
			"--time",
			triggerAt.Format(time.RFC3339Nano),
		})

		results, err := e.CreateQuery().QueryTasks(context.Background(), engine.TaskCriteria{Partition: lockedJobs[0].Partition, ProcessInstanceId: lockedJobs[0].ProcessInstanceId})
		require.NoError(err, "failed to query task")

		assert.Equal(triggerAt, results[0].DueAt)
	})

	t.Run("time-cycle", func(t *testing.T) {
		mustExecute(t, e, []string{
			"process-instance",
			"create",
			"--bpmn-process-id",
			"timerCatchTest",
			"--version",
			"1",
		})

		lockedJobs, err := e.LockJobs(context.Background(), engine.LockJobsCmd{WorkerId: program})
		require.NoError(err, "failed to lock job")
		require.NotEmpty(lockedJobs, "no job locked")

		mustExecute(t, e, []string{
			"job",
			"complete",
			"--partition",
			lockedJobs[0].Partition.String(),
			"--id",
			strconv.Itoa(int(lockedJobs[0].Id)),
			"--time-cycle",
			"0 * * * *",
		})

		jobs, err := e.CreateQuery().QueryJobs(context.Background(), engine.JobCriteria{Partition: lockedJobs[0].Partition, Id: lockedJobs[0].Id})
		require.NoError(err, "failed to query job")

		require.Falsef(jobs[0].HasError(), "completed job has error: %s", jobs[0].Error)

		tasks, err := e.CreateQuery().QueryTasks(context.Background(), engine.TaskCriteria{Partition: jobs[0].Partition, ProcessInstanceId: jobs[0].ProcessInstanceId})
		require.NoError(err, "failed to query task")
		require.NotEmpty(tasks, "no task queried")

		assert.Equal(jobs[0].CompletedAt.Add(time.Hour).Truncate(time.Hour), tasks[0].DueAt)
	})

	t.Run("time-duration", func(t *testing.T) {
		mustExecute(t, e, []string{
			"process-instance",
			"create",
			"--bpmn-process-id",
			"timerCatchTest",
			"--version",
			"1",
		})

		lockedJobs, err := e.LockJobs(context.Background(), engine.LockJobsCmd{WorkerId: program})
		require.NoError(err, "failed to lock job")
		require.NotEmpty(lockedJobs, "no job locked")

		mustExecute(t, e, []string{
			"job",
			"complete",
			"--partition",
			lockedJobs[0].Partition.String(),
			"--id",
			strconv.Itoa(int(lockedJobs[0].Id)),
			"--time-duration",
			"PT1H",
		})

		jobs, err := e.CreateQuery().QueryJobs(context.Background(), engine.JobCriteria{Partition: lockedJobs[0].Partition, Id: lockedJobs[0].Id})
		require.NoError(err, "failed to query job")

		require.Falsef(jobs[0].HasError(), "completed job has error: %s", jobs[0].Error)

		tasks, err := e.CreateQuery().QueryTasks(context.Background(), engine.TaskCriteria{Partition: jobs[0].Partition, ProcessInstanceId: jobs[0].ProcessInstanceId})
		require.NoError(err, "failed to query task")
		require.NotEmpty(tasks, "no task queried")

		assert.Equal(jobs[0].CompletedAt.Add(time.Hour), tasks[0].DueAt)
	})
}
