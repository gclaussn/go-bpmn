package pg

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// !keep in sync with engine/mem/incident_test.go
func TestResolveIncident(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	q := e.CreateQuery()

	// given
	date := time.Now().UTC().Truncate(24 * time.Hour)

	job := &internal.JobEntity{
		Partition: date,

		ElementId:         1,
		ElementInstanceId: 10,
		ProcessId:         100,
		ProcessInstanceId: 1000,

		BpmnElementId:  "a",
		CorrelationKey: pgtype.Text{String: "ck", Valid: true},
		Type:           engine.JobExecute,
	}

	task := &internal.TaskEntity{
		Partition: date,

		ElementId:         pgtype.Int4{Int32: 1, Valid: true},
		ElementInstanceId: pgtype.Int4{Int32: 10, Valid: true},
		ProcessId:         pgtype.Int4{Int32: 100, Valid: true},
		ProcessInstanceId: pgtype.Int4{Int32: 1000, Valid: true},

		BpmnElementId:  pgtype.Text{String: "test-element", Valid: true},
		SerializedTask: pgtype.Text{String: `{"test":true}`, Valid: true},
		Type:           engine.TaskJoinParallelGateway,
	}

	mustInsertEntities(t, e, []any{job, task})

	t.Run("resolve job incident", func(t *testing.T) {
		// given
		incident := &internal.IncidentEntity{
			Partition: date,

			JobId: pgtype.Int4{Int32: job.Id, Valid: true},
		}

		mustInsertEntities(t, e, []any{incident})

		// when
		cmd := engine.ResolveIncidentCmd{
			Id:        incident.Id,
			Partition: engine.Partition(date),
			WorkerId:  "test-worker",
		}

		if err := e.ResolveIncident(context.Background(), cmd); err != nil {
			t.Fatalf("failed to resolve incident: %v", err)
		}

		// then
		incidents, err := q.QueryIncidents(context.Background(), engine.IncidentCriteria{Partition: engine.Partition(date), Id: incident.Id})
		if err != nil {
			t.Fatalf("failed to query incident: %v", err)
		}

		assert.Lenf(incidents, 1, "expected one incident")

		assert.Equal(engine.Incident{
			Partition: engine.Partition(date),
			Id:        incident.Id,

			JobId: job.Id,

			CreatedAt:  incidents[0].CreatedAt,
			ResolvedAt: incidents[0].ResolvedAt,
			ResolvedBy: cmd.WorkerId,
		}, incidents[0])

		// then
		jobs, err := q.QueryJobs(context.Background(), engine.JobCriteria{Partition: engine.Partition(date)})
		if err != nil {
			t.Fatalf("failed to query jobs: %v", err)
		}

		retryJob := jobs[len(jobs)-1]

		assert.Equal(engine.Job{
			Partition: engine.Partition(date),
			Id:        retryJob.Id,

			ElementId:         job.ElementId,
			ElementInstanceId: job.ElementInstanceId,
			ProcessId:         job.ProcessId,
			ProcessInstanceId: job.ProcessInstanceId,

			BpmnElementId:  job.BpmnElementId,
			CorrelationKey: job.CorrelationKey.String,
			CreatedAt:      retryJob.CreatedAt,
			CreatedBy:      cmd.WorkerId,
			DueAt:          retryJob.CreatedAt,
			RetryCount:     0,
			State:          engine.WorkCreated,
			Type:           job.Type,
		}, retryJob)

		// when resolved again
		err = e.ResolveIncident(context.Background(), cmd)

		// then
		assert.IsTypef(engine.Error{}, err, "expected engine error")

		engineErr := err.(engine.Error)
		assert.Equal(engine.ErrorConflict, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)
	})

	t.Run("resolve job incident with retry timer", func(t *testing.T) {
		// given
		incident := &internal.IncidentEntity{
			Partition: date,

			JobId: pgtype.Int4{Int32: job.Id, Valid: true},
		}

		mustInsertEntities(t, e, []any{incident})

		// when
		cmd := engine.ResolveIncidentCmd{
			Id:         incident.Id,
			Partition:  engine.Partition(incident.Partition),
			RetryTimer: engine.ISO8601Duration("PT1H"),
			WorkerId:   "test-worker",
		}

		if err := e.ResolveIncident(context.Background(), cmd); err != nil {
			t.Fatalf("failed to resolve incident: %v", err)
		}

		// then
		jobs, err := q.QueryJobs(context.Background(), engine.JobCriteria{Partition: engine.Partition(date)})
		if err != nil {
			t.Fatalf("failed to query jobs: %v", err)
		}

		retryJob := jobs[len(jobs)-1]

		assert.Equal(engine.Job{
			Partition: engine.Partition(date),
			Id:        retryJob.Id,

			ElementId:         job.ElementId,
			ElementInstanceId: job.ElementInstanceId,
			ProcessId:         job.ProcessId,
			ProcessInstanceId: job.ProcessInstanceId,

			BpmnElementId:  job.BpmnElementId,
			CorrelationKey: job.CorrelationKey.String,
			CreatedAt:      retryJob.CreatedAt,
			CreatedBy:      cmd.WorkerId,
			DueAt:          retryJob.CreatedAt.Add(1 * time.Hour),
			RetryCount:     0,
			State:          engine.WorkCreated,
			Type:           job.Type,
		}, retryJob)
	})

	t.Run("resolve task incident", func(t *testing.T) {
		// given
		incident := &internal.IncidentEntity{
			Partition: date,

			TaskId: pgtype.Int4{Int32: task.Id, Valid: true},
		}

		mustInsertEntities(t, e, []any{incident})

		// when
		cmd := engine.ResolveIncidentCmd{
			Id:        incident.Id,
			Partition: engine.Partition(incident.Partition),
			WorkerId:  "test-worker",
		}

		if err := e.ResolveIncident(context.Background(), cmd); err != nil {
			t.Fatalf("failed to resolve incident: %v", err)
		}

		// then
		incidents, err := q.QueryIncidents(context.Background(), engine.IncidentCriteria{Partition: engine.Partition(date), Id: incident.Id})
		if err != nil {
			t.Fatalf("failed to query incident: %v", err)
		}

		assert.Lenf(incidents, 1, "expected one incident")

		assert.Equal(engine.Incident{
			Partition: engine.Partition(date),
			Id:        incident.Id,

			TaskId: task.Id,

			CreatedAt:  incidents[0].CreatedAt,
			ResolvedAt: incidents[0].ResolvedAt,
			ResolvedBy: cmd.WorkerId,
		}, incidents[0])

		// then
		tasks, err := q.QueryTasks(context.Background(), engine.TaskCriteria{Partition: engine.Partition(date)})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		retryTask := tasks[len(tasks)-1]

		assert.Equal(engine.Task{
			Partition: engine.Partition(date),
			Id:        retryTask.Id,

			ElementId:         task.ElementId.Int32,
			ElementInstanceId: task.ElementInstanceId.Int32,
			ProcessId:         task.ProcessId.Int32,
			ProcessInstanceId: task.ProcessInstanceId.Int32,

			BpmnElementId:  task.BpmnElementId.String,
			CreatedAt:      retryTask.CreatedAt,
			CreatedBy:      cmd.WorkerId,
			DueAt:          retryTask.CreatedAt,
			RetryCount:     0,
			SerializedTask: retryTask.SerializedTask,
			State:          engine.WorkCreated,
			Type:           task.Type,
		}, retryTask)
	})

	t.Run("resolve task incident with retry timer", func(t *testing.T) {
		// given
		incident := &internal.IncidentEntity{
			Partition: date,

			TaskId: pgtype.Int4{Int32: task.Id, Valid: true},
		}

		mustInsertEntities(t, e, []any{incident})

		// when
		cmd := engine.ResolveIncidentCmd{
			Id:         incident.Id,
			Partition:  engine.Partition(incident.Partition),
			RetryTimer: engine.ISO8601Duration("P1D"),
			WorkerId:   "test-worker",
		}

		if err := e.ResolveIncident(context.Background(), cmd); err != nil {
			t.Fatalf("failed to resolve incident: %v", err)
		}

		// then
		tasks, err := q.QueryTasks(context.Background(), engine.TaskCriteria{Partition: engine.Partition(date)})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		retryTask := tasks[len(tasks)-1]

		assert.Equal(engine.Task{
			Partition: engine.Partition(date),
			Id:        retryTask.Id,

			ElementId:         task.ElementId.Int32,
			ElementInstanceId: task.ElementInstanceId.Int32,
			ProcessId:         task.ProcessId.Int32,
			ProcessInstanceId: task.ProcessInstanceId.Int32,

			BpmnElementId:  task.BpmnElementId.String,
			CreatedAt:      retryTask.CreatedAt,
			CreatedBy:      cmd.WorkerId,
			DueAt:          retryTask.CreatedAt.Add(24 * time.Hour),
			RetryCount:     0,
			SerializedTask: retryTask.SerializedTask,
			State:          engine.WorkCreated,
			Type:           task.Type,
		}, retryTask)
	})
}
