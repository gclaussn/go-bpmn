package pg

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// !keep in sync with engine/mem/job_test.go
func TestLockJobs(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	var (
		now       = time.Now().UTC().Truncate(time.Millisecond)
		date      = now.Truncate(24 * time.Hour)
		datePlus1 = date.AddDate(0, 0, 1)

		partitions = []time.Time{date, date, date, datePlus1, datePlus1, datePlus1, datePlus1}

		elementIds         = []int32{7, 6, 5, 4, 3, 2, 1}
		elementInstanceIds = []int32{1, 2, 3, 4, 5, 6, 7}
		processIds         = []int32{1, 1, 1, 2, 2, 3, 4}
		processInstanceIds = []int32{10, 10, 10, 20, 20, 30, 40}

		bpmnElementIds = []string{"a", "b", "c", "a", "d", "f", "z"}
		dueAts         = []time.Time{
			now.Add(time.Minute * -5),
			now.Add(time.Minute * -4),
			now.Add(time.Minute * -3),
			now.Add(time.Minute * -2),
			now.Add(time.Minute * -1),
			now.Add(time.Second * -15),
			now.Add(time.Hour),
		}
	)

	jobs := make([]*internal.JobEntity, len(partitions))
	entities := make([]any, len(jobs))
	for i := range jobs {
		jobs[i] = &internal.JobEntity{
			Partition: partitions[i],

			ElementId:         elementIds[i],
			ElementInstanceId: elementInstanceIds[i],
			ProcessId:         processIds[i],
			ProcessInstanceId: processInstanceIds[i],

			BpmnElementId:   bpmnElementIds[i],
			BpmnElementType: model.ElementServiceTask,
			CorrelationKey:  pgtype.Text{String: "ck", Valid: true},
			DueAt:           dueAts[i],
			RetryCount:      1,
			State:           engine.WorkCreated,
			Type:            engine.JobExecute,
		}

		entities[i] = jobs[i]
	}

	mustInsertEntities(t, e, entities)

	lock := func(cmd engine.LockJobsCmd) []engine.LockedJob {
		cmd.WorkerId = testWorkerId

		lockedJobs, err := e.LockJobs(context.Background(), cmd)
		if err != nil {
			t.Fatalf("failed to lock jobs: %v", err)
		}

		return lockedJobs
	}

	unlock := func() {
		_, err := e.UnlockJobs(context.Background(), engine.UnlockJobsCmd{
			WorkerId: testWorkerId,
		})
		if err != nil {
			t.Fatalf("failed to unlock jobs: %v", err)
		}
	}

	var lockedJobs []engine.LockedJob

	t.Run("limit", func(t *testing.T) {
		unlock()

		lockedJobs = lock(engine.LockJobsCmd{ProcessIds: processIds})
		require.Len(lockedJobs, 1)

		assert.Equal(engine.Partition(date), lockedJobs[0].Partition)
		assert.Equal(jobs[0].Id, lockedJobs[0].Id)

		assert.Equal(jobs[0].ElementId, lockedJobs[0].ElementId)
		assert.Equal(jobs[0].ElementInstanceId, lockedJobs[0].ElementInstanceId)
		assert.Equal(jobs[0].ProcessId, lockedJobs[0].ProcessId)
		assert.Equal(jobs[0].ProcessInstanceId, lockedJobs[0].ProcessInstanceId)

		assert.Equal(jobs[0].BpmnElementId, lockedJobs[0].BpmnElementId)
		assert.Equal(jobs[0].BpmnElementType, lockedJobs[0].BpmnElementType)
		assert.Equal(jobs[0].CorrelationKey.String, lockedJobs[0].CorrelationKey)
		assert.Equal(jobs[0].DueAt, lockedJobs[0].DueAt)
		assert.NotZero(lockedJobs[0].LockedAt)
		assert.Equal(testWorkerId, lockedJobs[0].LockedBy)
		assert.Equal(jobs[0].RetryCount, lockedJobs[0].RetryCount)
		assert.Equal(jobs[0].Type, lockedJobs[0].Type)

		lockedJobs = lock(engine.LockJobsCmd{ProcessIds: processIds, Limit: 3})
		assert.Len(lockedJobs, 3)
		assert.Equal(engine.Partition(date), lockedJobs[0].Partition)
		assert.Equal(jobs[1].Id, lockedJobs[0].Id)
		assert.Equal(engine.Partition(date), lockedJobs[1].Partition)
		assert.Equal(jobs[2].Id, lockedJobs[1].Id)
		assert.Equal(engine.Partition(datePlus1), lockedJobs[2].Partition)
		assert.Equal(jobs[3].Id, lockedJobs[2].Id)

		jobs, err := e.CreateQuery().QueryJobs(context.Background(), engine.JobCriteria{
			Partition: lockedJobs[0].Partition,
			Id:        lockedJobs[0].Id,
		})
		if err != nil {
			t.Fatalf("failed to query jobs: %v", err)
		}

		require.Len(jobs, 1)
		assert.NotZero(jobs[0].LockedAt)
		assert.Equal(testWorkerId, jobs[0].LockedBy)
		assert.Equal(engine.WorkLocked, jobs[0].State)
	})

	t.Run("by partition", func(t *testing.T) {
		unlock()

		lockedJobs = lock(engine.LockJobsCmd{ProcessIds: processIds, Partition: engine.Partition(datePlus1), Limit: len(partitions)})
		require.Len(lockedJobs, 3)
		assert.Equal(engine.Partition(datePlus1), lockedJobs[0].Partition)
		assert.Equal(jobs[3].Id, lockedJobs[0].Id)
		assert.Equal(engine.Partition(datePlus1), lockedJobs[1].Partition)
		assert.Equal(jobs[4].Id, lockedJobs[1].Id)
		assert.Equal(engine.Partition(datePlus1), lockedJobs[2].Partition)
		assert.Equal(jobs[5].Id, lockedJobs[2].Id)
	})

	t.Run("by partition and ID", func(t *testing.T) {
		unlock()

		lockedJobs = lock(engine.LockJobsCmd{ProcessIds: processIds, Partition: engine.Partition(date), Id: 2, Limit: len(partitions)})
		require.Len(lockedJobs, 1)
		assert.Equal(int32(2), lockedJobs[0].Id)

		// when already locked
		lockedJobs = lock(engine.LockJobsCmd{ProcessIds: processIds, Partition: engine.Partition(date), Id: 2, Limit: len(partitions)})
		assert.Len(lockedJobs, 0)
	})

	t.Run("by process IDs", func(t *testing.T) {
		unlock()

		lockedJobs = lock(engine.LockJobsCmd{ProcessIds: []int32{1, 3}, Limit: len(partitions)})
		require.Len(lockedJobs, 4)
		assert.Equal(int32(1), lockedJobs[0].ProcessId)
		assert.Equal(int32(1), lockedJobs[1].ProcessId)
		assert.Equal(int32(1), lockedJobs[2].ProcessId)
		assert.Equal(int32(3), lockedJobs[3].ProcessId)
	})

	t.Run("by process instance ID", func(t *testing.T) {
		unlock()

		lockedJobs = lock(engine.LockJobsCmd{ProcessIds: processIds, ProcessInstanceId: 10, Limit: len(partitions)})
		require.Len(lockedJobs, 3)
		assert.Equal(int32(10), lockedJobs[0].ProcessInstanceId)
		assert.Equal(int32(10), lockedJobs[1].ProcessInstanceId)
		assert.Equal(int32(10), lockedJobs[2].ProcessInstanceId)
	})
}

// !keep in sync with engine/mem/job_test.go
func TestUnlockJobs(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	var (
		now       = time.Now().UTC()
		date      = now.Truncate(24 * time.Hour)
		datePlus1 = date.AddDate(0, 0, 1)

		partitions = []time.Time{date, date, date, datePlus1, datePlus1}
	)

	jobs := make([]*internal.JobEntity, len(partitions))
	entities := make([]any, len(jobs))
	for i := range jobs {
		jobs[i] = &internal.JobEntity{
			Partition: partitions[i],

			ProcessId: 1,

			State: engine.WorkCreated,
		}

		entities[i] = jobs[i]
	}

	mustInsertEntities(t, e, entities)

	lock := func() {
		_, err := e.LockJobs(context.Background(), engine.LockJobsCmd{
			ProcessIds: []int32{1},
			Limit:      100,
			WorkerId:   testWorkerId,
		})
		if err != nil {
			t.Fatalf("failed to lock jobs: %v", err)
		}
	}

	unlock := func(cmd engine.UnlockJobsCmd) int {
		count, err := e.UnlockJobs(context.Background(), cmd)
		if err != nil {
			t.Fatalf("failed to unlock jobs: %v", err)
		}

		return count
	}

	var count int

	t.Run("by worker ID", func(t *testing.T) {
		lock()

		// when different worker ID is used
		count = unlock(engine.UnlockJobsCmd{WorkerId: "different-worker"})
		assert.Equal(0, count)

		count = unlock(engine.UnlockJobsCmd{WorkerId: testWorkerId})
		assert.Equal(len(entities), count)

		// when already unlocked
		count = unlock(engine.UnlockJobsCmd{WorkerId: testWorkerId})
		assert.Equal(0, count)

		jobs, err := e.CreateQuery().QueryJobs(context.Background(), engine.JobCriteria{})
		if err != nil {
			t.Fatalf("failed to query jobs: %v", err)
		}

		require.Len(jobs, len(entities))

		for _, job := range jobs {
			assert.Zero(job.LockedAt)
			assert.Empty(job.LockedBy)
			assert.Equal(engine.WorkDue, job.State)
		}
	})

	t.Run("by partition", func(t *testing.T) {
		lock()

		count = unlock(engine.UnlockJobsCmd{WorkerId: testWorkerId, Partition: engine.Partition(date)})
		assert.Equal(3, count)

		// when already unlocked
		count = unlock(engine.UnlockJobsCmd{WorkerId: testWorkerId, Partition: engine.Partition(date)})
		assert.Equal(0, count)
	})

	t.Run("by partition and ID", func(t *testing.T) {
		lock()

		count = unlock(engine.UnlockJobsCmd{WorkerId: testWorkerId, Partition: engine.Partition(date), Id: 3})
		assert.Equal(1, count)

		// when already unlocked
		count = unlock(engine.UnlockJobsCmd{WorkerId: testWorkerId, Partition: engine.Partition(date), Id: 3})
		assert.Equal(0, count)
	})

	t.Run("completed", func(t *testing.T) {
		lock()

		// simulate completion
		for i := range jobs {
			jobs[i].CompletedAt = pgtype.Timestamp{Time: now, Valid: true}
			jobs[i].State = engine.WorkDue
		}

		mustUpdateEntities(t, e, entities)

		count = unlock(engine.UnlockJobsCmd{WorkerId: testWorkerId})
		assert.Equal(0, count)
	})
}
