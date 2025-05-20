package pg

import (
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
			Id:         incident.Id,
			Partition:  engine.Partition(date),
			RetryCount: 1,
			WorkerId:   "test-worker",
		}

		if err := e.ResolveIncident(cmd); err != nil {
			t.Fatalf("failed to resolve incident: %v", err)
		}

		// then
		results, err := e.Query(engine.IncidentCriteria{Partition: engine.Partition(date), Id: incident.Id})
		if err != nil {
			t.Fatalf("failed to query incident: %v", err)
		}

		assert.Lenf(results, 1, "expected one incident")

		resolvedIncident := results[0].(engine.Incident)

		assert.Equal(engine.Incident{
			Partition: engine.Partition(date),
			Id:        incident.Id,

			JobId: job.Id,

			CreatedAt:  resolvedIncident.CreatedAt,
			ResolvedAt: resolvedIncident.ResolvedAt,
			ResolvedBy: cmd.WorkerId,
		}, resolvedIncident)

		// then
		results, err = e.Query(engine.JobCriteria{Partition: engine.Partition(date)})
		if err != nil {
			t.Fatalf("failed to query jobs: %v", err)
		}

		retryJob := results[len(results)-1].(engine.Job)

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
			RetryCount:     cmd.RetryCount,
			RetryTimer:     engine.ISO8601Duration(""),
			Type:           job.Type,
		}, retryJob)

		// when resolved again
		err = e.ResolveIncident(cmd)

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
			RetryCount: 3,
			RetryTimer: engine.ISO8601Duration("PT1H"),
			WorkerId:   "test-worker",
		}

		if err := e.ResolveIncident(cmd); err != nil {
			t.Fatalf("failed to resolve incident: %v", err)
		}

		// then
		results, err := e.Query(engine.JobCriteria{Partition: engine.Partition(date)})
		if err != nil {
			t.Fatalf("failed to query jobs: %v", err)
		}

		retryJob := results[len(results)-1].(engine.Job)

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
			RetryCount:     cmd.RetryCount,
			RetryTimer:     cmd.RetryTimer,
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
			Id:         incident.Id,
			Partition:  engine.Partition(incident.Partition),
			RetryCount: 1,
			WorkerId:   "test-worker",
		}

		if err := e.ResolveIncident(cmd); err != nil {
			t.Fatalf("failed to resolve incident: %v", err)
		}

		// then
		results, err := e.Query(engine.IncidentCriteria{Partition: engine.Partition(date), Id: incident.Id})
		if err != nil {
			t.Fatalf("failed to query incident: %v", err)
		}

		assert.Lenf(results, 1, "expected one incident")

		resolvedIncident := results[0].(engine.Incident)

		assert.Equal(engine.Incident{
			Partition: engine.Partition(date),
			Id:        incident.Id,

			TaskId: task.Id,

			CreatedAt:  resolvedIncident.CreatedAt,
			ResolvedAt: resolvedIncident.ResolvedAt,
			ResolvedBy: cmd.WorkerId,
		}, resolvedIncident)

		// then
		results, err = e.Query(engine.TaskCriteria{Partition: engine.Partition(date)})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		retryTask := results[len(results)-1].(engine.Task)

		assert.Equal(engine.Task{
			Partition: engine.Partition(date),
			Id:        retryTask.Id,

			ElementId:         task.ElementId.Int32,
			ElementInstanceId: task.ElementInstanceId.Int32,
			ProcessId:         task.ProcessId.Int32,
			ProcessInstanceId: task.ProcessInstanceId.Int32,

			CreatedAt:      retryTask.CreatedAt,
			CreatedBy:      cmd.WorkerId,
			DueAt:          retryTask.CreatedAt,
			RetryCount:     cmd.RetryCount,
			SerializedTask: retryTask.SerializedTask,
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
			RetryCount: 1,
			RetryTimer: engine.ISO8601Duration("P1D"),
			WorkerId:   "test-worker",
		}

		if err := e.ResolveIncident(cmd); err != nil {
			t.Fatalf("failed to resolve incident: %v", err)
		}

		// then
		results, err := e.Query(engine.TaskCriteria{Partition: engine.Partition(date)})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		retryTask := results[len(results)-1].(engine.Task)

		assert.Equal(engine.Task{
			Partition: engine.Partition(date),
			Id:        retryTask.Id,

			ElementId:         task.ElementId.Int32,
			ElementInstanceId: task.ElementInstanceId.Int32,
			ProcessId:         task.ProcessId.Int32,
			ProcessInstanceId: task.ProcessInstanceId.Int32,

			CreatedAt:      retryTask.CreatedAt,
			CreatedBy:      cmd.WorkerId,
			DueAt:          retryTask.CreatedAt.Add(24 * time.Hour),
			RetryCount:     cmd.RetryCount,
			RetryTimer:     cmd.RetryTimer,
			SerializedTask: retryTask.SerializedTask,
			Type:           task.Type,
		}, retryTask)
	})
}
