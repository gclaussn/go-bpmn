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
)

// !keep in sync with engine/mem/job_test.go
func TestLockJobs(t *testing.T) {
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

	runJobLockTests(t, e, []jobLockTest{
		{
			"no limit",
			engine.LockJobsCmd{ProcessIds: processIds},
			func(assert *assert.Assertions, lockedJobs []engine.LockedJob) {
				assert.Len(lockedJobs, 1)
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
			},
		},
		{
			"limit",
			engine.LockJobsCmd{ProcessIds: processIds, Limit: 3},
			func(assert *assert.Assertions, lockedJobs []engine.LockedJob) {
				assert.Len(lockedJobs, 3)
				assert.Equal(engine.Partition(date), lockedJobs[0].Partition)
				assert.Equal(jobs[1].Id, lockedJobs[0].Id)
				assert.Equal(engine.Partition(date), lockedJobs[1].Partition)
				assert.Equal(jobs[2].Id, lockedJobs[1].Id)
				assert.Equal(engine.Partition(datePlus1), lockedJobs[2].Partition)
				assert.Equal(jobs[3].Id, lockedJobs[2].Id)
			},
		},
	})

	runJobLockTests(t, e, []jobLockTest{
		{
			"by partition",
			engine.LockJobsCmd{ProcessIds: processIds, Partition: engine.Partition(datePlus1), Limit: len(partitions)},
			func(assert *assert.Assertions, lockedJobs []engine.LockedJob) {
				assert.Len(lockedJobs, 3)
				assert.Equal(engine.Partition(datePlus1), lockedJobs[0].Partition)
				assert.Equal(jobs[3].Id, lockedJobs[0].Id)
				assert.Equal(engine.Partition(datePlus1), lockedJobs[1].Partition)
				assert.Equal(jobs[4].Id, lockedJobs[1].Id)
				assert.Equal(engine.Partition(datePlus1), lockedJobs[2].Partition)
				assert.Equal(jobs[5].Id, lockedJobs[2].Id)
			},
		},
	})

	runJobLockTests(t, e, []jobLockTest{
		{
			"by partition and ID",
			engine.LockJobsCmd{ProcessIds: processIds, Partition: engine.Partition(date), Id: 2, Limit: len(partitions)},
			func(assert *assert.Assertions, lockedJobs []engine.LockedJob) {
				assert.Len(lockedJobs, 1)
				assert.Equal(int32(2), lockedJobs[0].Id)
			},
		},
		{
			"by partition and ID, but already locked",
			engine.LockJobsCmd{ProcessIds: processIds, Partition: engine.Partition(date), Id: 2, Limit: len(partitions)},
			func(assert *assert.Assertions, lockedJobs []engine.LockedJob) {
				assert.Len(lockedJobs, 0)
			},
		},
	})

	runJobLockTests(t, e, []jobLockTest{
		{
			"by process IDs",
			engine.LockJobsCmd{ProcessIds: []int32{1, 3}, Limit: len(partitions)},
			func(assert *assert.Assertions, lockedJobs []engine.LockedJob) {
				assert.Len(lockedJobs, 4)
				assert.Equal(int32(1), lockedJobs[0].ProcessId)
				assert.Equal(int32(1), lockedJobs[1].ProcessId)
				assert.Equal(int32(1), lockedJobs[2].ProcessId)
				assert.Equal(int32(3), lockedJobs[3].ProcessId)
			},
		},
	})

	runJobLockTests(t, e, []jobLockTest{
		{
			"by process instance ID",
			engine.LockJobsCmd{ProcessIds: processIds, ProcessInstanceId: 10, Limit: len(partitions)},
			func(assert *assert.Assertions, lockedJobs []engine.LockedJob) {
				assert.Len(lockedJobs, 3)
				assert.Equal(int32(10), lockedJobs[0].ProcessInstanceId)
				assert.Equal(int32(10), lockedJobs[1].ProcessInstanceId)
				assert.Equal(int32(10), lockedJobs[2].ProcessInstanceId)
			},
		},
	})
}

// !keep in sync with engine/pg/job_test.go
func TestUnlockJobs(t *testing.T) {
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
		}

		entities[i] = jobs[i]
	}

	mustInsertEntities(t, e, entities)

	runJobUnlockTests(t, e, []jobUnlockTest{
		{
			"by different worker ID",
			engine.UnlockJobsCmd{WorkerId: "different-worker"},
			0,
		},
		{
			"by worker ID",
			engine.UnlockJobsCmd{WorkerId: testWorkerId},
			len(entities),
		},
		{
			"by worker ID, but already unlocked",
			engine.UnlockJobsCmd{WorkerId: testWorkerId},
			0,
		},
	})

	runJobUnlockTests(t, e, []jobUnlockTest{
		{
			"by partition",
			engine.UnlockJobsCmd{WorkerId: testWorkerId, Partition: engine.Partition(date)},
			3,
		},
		{
			"by partition, but already unlocked",
			engine.UnlockJobsCmd{WorkerId: testWorkerId, Partition: engine.Partition(date)},
			0,
		},
	})

	runJobUnlockTests(t, e, []jobUnlockTest{
		{
			"by partition and ID",
			engine.UnlockJobsCmd{WorkerId: testWorkerId, Partition: engine.Partition(date), Id: 3},
			1,
		},
		{
			"by partition and ID, but already unlocked",
			engine.UnlockJobsCmd{WorkerId: testWorkerId, Partition: engine.Partition(date), Id: 3},
			0,
		},
	})

	// complete jobs
	_, err := e.LockJobs(context.Background(), engine.LockJobsCmd{
		ProcessIds: []int32{1},
		Limit:      100,
		WorkerId:   testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to lock jobs: %v", err)
	}

	for i := range jobs {
		jobs[i].CompletedAt = pgtype.Timestamp{Time: now, Valid: true}
		jobs[i].State = engine.WorkDue
	}

	mustUpdateEntities(t, e, entities)

	runJobUnlockTests(t, e, []jobUnlockTest{
		{
			"by worker ID, but already completed",
			engine.UnlockJobsCmd{WorkerId: testWorkerId},
			0,
		},
	})
}

type jobLockTest struct {
	name     string
	cmd      engine.LockJobsCmd
	assertFn func(*assert.Assertions, []engine.LockedJob)
}

type jobUnlockTest struct {
	name          string
	cmd           engine.UnlockJobsCmd
	expectedCount int
}

func runJobLockTests(t *testing.T, e engine.Engine, tests []jobLockTest) {
	_, err := e.UnlockJobs(context.Background(), engine.UnlockJobsCmd{
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to unlock jobs: %v", err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			cmd := test.cmd
			cmd.WorkerId = testWorkerId

			lockedJobs, err := e.LockJobs(context.Background(), cmd)
			if err != nil {
				t.Fatalf("failed to lock jobs: %v", err)
			}

			test.assertFn(assert, lockedJobs)
		})
	}
}

func runJobUnlockTests(t *testing.T, e engine.Engine, tests []jobUnlockTest) {
	_, err := e.LockJobs(context.Background(), engine.LockJobsCmd{
		ProcessIds: []int32{1},
		Limit:      100,
		WorkerId:   testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to lock jobs: %v", err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert := assert.New(t)

			count, err := e.UnlockJobs(context.Background(), test.cmd)
			if err != nil {
				t.Fatalf("failed to unlock jobs: %v", err)
			}

			assert.Equal(test.expectedCount, count)
		})
	}
}
