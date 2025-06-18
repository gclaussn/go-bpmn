package cli

import (
	"strconv"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestJobCompleteSetTimer(t *testing.T) {
	assert := assert.New(t)

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

	var results []any

	t.Run("time", func(t *testing.T) {
		mustExecute(t, e, []string{
			"process-instance",
			"create",
			"--bpmn-process-id",
			"timerCatchTest",
			"--version",
			"1",
		})

		lockedJobs, err := e.LockJobs(engine.LockJobsCmd{WorkerId: program})
		if err != nil {
			t.Fatalf("failed to lock job: %v", err)
		}
		if len(lockedJobs) == 0 {
			t.Fatal("no job locked")
		}

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

		results, err = e.Query(engine.TaskCriteria{Partition: lockedJobs[0].Partition, ProcessInstanceId: lockedJobs[0].ProcessInstanceId})
		if err != nil {
			t.Fatalf("failed to query task: %v", err)
		}

		task := results[0].(engine.Task)
		assert.Equal(triggerAt, task.DueAt)
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

		lockedJobs, err := e.LockJobs(engine.LockJobsCmd{WorkerId: program})
		if err != nil {
			t.Fatalf("failed to lock job: %v", err)
		}
		if len(lockedJobs) == 0 {
			t.Fatal("no job locked")
		}

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

		results, err = e.Query(engine.JobCriteria{Partition: lockedJobs[0].Partition, Id: lockedJobs[0].Id})
		if err != nil {
			t.Fatalf("failed to query job: %v", err)
		}

		job := results[0].(engine.Job)
		if job.HasError() {
			t.Fatalf("completed job has error: %s", job.Error)
		}

		results, err = e.Query(engine.TaskCriteria{Partition: job.Partition, ProcessInstanceId: job.ProcessInstanceId})
		if err != nil {
			t.Fatalf("failed to query task: %v", err)
		}

		task := results[0].(engine.Task)
		assert.Equal(job.CompletedAt.Add(time.Hour).Truncate(time.Hour), task.DueAt)
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

		lockedJobs, err := e.LockJobs(engine.LockJobsCmd{WorkerId: program})
		if err != nil {
			t.Fatalf("failed to lock job: %v", err)
		}
		if len(lockedJobs) == 0 {
			t.Fatal("no job locked")
		}

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

		results, err = e.Query(engine.JobCriteria{Partition: lockedJobs[0].Partition, Id: lockedJobs[0].Id})
		if err != nil {
			t.Fatalf("failed to query job: %v", err)
		}

		job := results[0].(engine.Job)
		if job.HasError() {
			t.Fatalf("completed job has error: %s", job.Error)
		}

		results, err = e.Query(engine.TaskCriteria{Partition: job.Partition, ProcessInstanceId: job.ProcessInstanceId})
		if err != nil {
			t.Fatalf("failed to query task: %v", err)
		}

		task := results[0].(engine.Task)
		assert.Equal(job.CompletedAt.Add(time.Hour), task.DueAt)
	})
}
