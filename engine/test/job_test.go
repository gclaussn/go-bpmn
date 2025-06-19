package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestCompleteJob(t *testing.T) {
	assert := assert.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	for i, e := range engines {
		// given
		process := mustCreateProcess(t, e, "task/service.bpmn", "serviceTest")

		newJob := func() engine.Job {
			processInstance, err := e.CreateProcessInstance(engine.CreateProcessInstanceCmd{
				BpmnProcessId:  process.BpmnProcessId,
				CorrelationKey: "ck",
				Version:        process.Version,
				WorkerId:       process.CreatedBy,
			})
			if err != nil {
				t.Fatalf("failed to create process instance: %v", err)
			}

			results, err := e.Query(engine.JobCriteria{Partition: processInstance.Partition, ProcessInstanceId: processInstance.Id})
			if err != nil {
				t.Fatalf("failed to query job: %v", err)
			}

			assert.Lenf(results, 1, "expected one job")

			return results[0].(engine.Job)
		}

		t.Run(engineTypes[i]+"returns error when job not exists", func(t *testing.T) {
			// when
			_, err := e.CompleteJob(engine.CompleteJobCmd{})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorNotFound, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"returns error when job is completed", func(t *testing.T) {
			// given
			job := newJob()

			cmd := engine.CompleteJobCmd{
				Partition: engine.Partition(job.Partition),
				Id:        job.Id,
				Error:     "test-error",
				WorkerId:  testWorkerId,
			}

			// when
			lockedJobs, err := e.LockJobs(engine.LockJobsCmd{
				Partition: job.Partition,
				Id:        job.Id,
				WorkerId:  testWorkerId,
			})
			if err != nil {
				t.Fatalf("failed to lock job: %v", err)
			}

			if len(lockedJobs) == 0 {
				t.Fatal("no job locked")
			}

			_, err = e.CompleteJob(cmd)
			if err != nil {
				t.Fatalf("failed to complete job: %v", err)
			}

			_, err = e.CompleteJob(cmd)

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorConflict, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"returns error when job is not locked", func(t *testing.T) {
			// given
			job := newJob()

			// when
			_, err := e.CompleteJob(engine.CompleteJobCmd{
				Partition: job.Partition,
				Id:        job.Id,
				WorkerId:  testWorkerId,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorConflict, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"returns error when job is locked by different worker", func(t *testing.T) {
			// given
			job := newJob()

			// when
			lockedJobs, err := e.LockJobs(engine.LockJobsCmd{
				Partition: job.Partition,
				Id:        job.Id,
				WorkerId:  "different-worker",
			})
			if err != nil {
				t.Fatalf("failed to lock job: %v", err)
			}

			if len(lockedJobs) == 0 {
				t.Fatal("no job locked")
			}

			_, err = e.CompleteJob(engine.CompleteJobCmd{
				Partition: job.Partition,
				Id:        job.Id,
				WorkerId:  testWorkerId,
			})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorConflict, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})

		t.Run(engineTypes[i]+"complete", func(t *testing.T) {
			// given
			job := newJob()

			// when
			lockedJobs, err := e.LockJobs(engine.LockJobsCmd{
				Partition: job.Partition,
				Id:        job.Id,
				WorkerId:  testWorkerId,
			})
			if err != nil {
				t.Fatalf("failed to lock job: %v", err)
			}

			if len(lockedJobs) == 0 {
				t.Fatal("no job locked")
			}

			completedJob, err := e.CompleteJob(engine.CompleteJobCmd{
				Partition: job.Partition,
				Id:        job.Id,
				WorkerId:  testWorkerId,
			})
			if err != nil {
				t.Fatalf("failed to complete job: %v", err)
			}

			// then
			assert.Equal(engine.Job{
				Partition: job.Partition,
				Id:        job.Id,

				ElementId:         job.ElementId,
				ElementInstanceId: job.ElementInstanceId,
				ProcessId:         job.ProcessId,
				ProcessInstanceId: job.ProcessInstanceId,

				BpmnElementId:      "serviceTask",
				BpmnErrorCode:      "",
				BpmnEscalationCode: "",
				CompletedAt:        completedJob.CompletedAt,
				CorrelationKey:     "ck",
				CreatedAt:          job.CreatedAt,
				CreatedBy:          testWorkerId,
				DueAt:              job.DueAt,
				Error:              "",
				LockedAt:           lockedJobs[0].LockedAt,
				LockedBy:           testWorkerId,
				RetryCount:         1,
				RetryTimer:         engine.ISO8601Duration(""),
				Type:               engine.JobExecute,
			}, completedJob)

			assert.NotEmpty(completedJob.CompletedAt)

			results, err := e.Query(engine.JobCriteria{Partition: job.Partition, Id: job.Id})
			if err != nil {
				t.Fatalf("failed to query job: %v", err)
			}

			assert.Lenf(results, 1, "expected one job")

			assert.Equal(engine.Job{
				Partition: job.Partition,
				Id:        job.Id,

				ElementId:         job.ElementId,
				ElementInstanceId: job.ElementInstanceId,
				ProcessId:         job.ProcessId,
				ProcessInstanceId: job.ProcessInstanceId,

				BpmnElementId:      "serviceTask",
				BpmnErrorCode:      "",
				BpmnEscalationCode: "",
				CompletedAt:        completedJob.CompletedAt,
				CorrelationKey:     "ck",
				CreatedAt:          job.CreatedAt,
				CreatedBy:          testWorkerId,
				DueAt:              job.DueAt,
				Error:              "",
				LockedAt:           lockedJobs[0].LockedAt,
				LockedBy:           testWorkerId,
				RetryCount:         1,
				RetryTimer:         engine.ISO8601Duration(""),
				Type:               engine.JobExecute,
			}, results[0])
		})

		t.Run(engineTypes[i]+"create retry job when completed with an error and retries left", func(t *testing.T) {
			// given
			job := newJob()

			// when
			lockedJobs, err := e.LockJobs(engine.LockJobsCmd{
				Partition: job.Partition,
				Id:        job.Id,
				WorkerId:  testWorkerId,
			})
			if err != nil {
				t.Fatalf("failed to lock job: %v", err)
			}

			if len(lockedJobs) == 0 {
				t.Fatal("no job locked")
			}

			cmd := engine.CompleteJobCmd{
				Partition:  job.Partition,
				Id:         job.Id,
				Error:      "test-error",
				RetryCount: 2,
				RetryTimer: engine.ISO8601Duration("PT1M"),
				WorkerId:   testWorkerId,
			}

			completedJob, err := e.CompleteJob(cmd)
			if err != nil {
				t.Fatalf("failed to complete job: %v", err)
			}

			// then
			assert.NotEmpty(completedJob.CompletedAt)
			assert.Equal("test-error", completedJob.Error)

			results, err := e.Query(engine.JobCriteria{Partition: job.Partition, Id: job.Id + 1})
			if err != nil {
				t.Fatalf("failed to query job: %v", err)
			}

			assert.Lenf(results, 1, "expected one job")

			assert.Equal(engine.Job{
				Partition: job.Partition,
				Id:        job.Id + 1,

				ElementId:         job.ElementId,
				ElementInstanceId: job.ElementInstanceId,
				ProcessId:         job.ProcessId,
				ProcessInstanceId: job.ProcessInstanceId,

				BpmnElementId:      "serviceTask",
				BpmnErrorCode:      "",
				BpmnEscalationCode: "",
				CompletedAt:        nil,
				CorrelationKey:     "ck",
				CreatedAt:          *completedJob.CompletedAt,
				CreatedBy:          testWorkerId,
				DueAt:              results[0].(engine.Job).DueAt,
				Error:              "",
				LockedAt:           nil,
				LockedBy:           "",
				RetryCount:         cmd.RetryCount,
				RetryTimer:         cmd.RetryTimer,
				Type:               engine.JobExecute,
			}, results[0])
		})

		t.Run(engineTypes[i]+"create incident when completed with an error and no retry left", func(t *testing.T) {
			// given
			job := newJob()

			// when
			lockedJobs, err := e.LockJobs(engine.LockJobsCmd{
				Partition: job.Partition,
				Id:        job.Id,
				WorkerId:  testWorkerId,
			})
			if err != nil {
				t.Fatalf("failed to lock job: %v", err)
			}

			if len(lockedJobs) == 0 {
				t.Fatal("no job locked")
			}

			cmd := engine.CompleteJobCmd{
				Partition: job.Partition,
				Id:        job.Id,
				Error:     "test-error",
				WorkerId:  testWorkerId,
			}

			completedJob, err := e.CompleteJob(cmd)
			if err != nil {
				t.Fatalf("failed to complete job: %v", err)
			}

			// then
			assert.NotEmpty(completedJob.CompletedAt)
			assert.Equal("test-error", completedJob.Error)

			results, err := e.Query(engine.IncidentCriteria{Partition: job.Partition, JobId: job.Id})
			if err != nil {
				t.Fatalf("failed to query incidents: %v", err)
			}

			assert.Lenf(results, 1, "expected one incident")

			incident := results[0].(engine.Incident)

			assert.Equal(engine.Incident{
				Partition: job.Partition,
				Id:        incident.Id,

				ElementId:         job.ElementId,
				ElementInstanceId: job.ElementInstanceId,
				JobId:             job.Id,
				ProcessId:         job.ProcessId,
				ProcessInstanceId: job.ProcessInstanceId,
				TaskId:            0,

				CreatedAt:  *completedJob.CompletedAt,
				CreatedBy:  testWorkerId,
				ResolvedAt: nil,
				ResolvedBy: "",
			}, incident)

			assert.NotEmpty(incident.CreatedAt)
		})
	}
}
