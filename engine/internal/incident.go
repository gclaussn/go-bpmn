package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type IncidentEntity struct {
	Partition time.Time
	Id        int32

	ElementId         pgtype.Int4
	ElementInstanceId pgtype.Int4
	JobId             pgtype.Int4
	ProcessId         pgtype.Int4
	ProcessInstanceId pgtype.Int4
	TaskId            pgtype.Int4

	CreatedAt  time.Time
	CreatedBy  string
	ResolvedAt pgtype.Timestamp
	ResolvedBy pgtype.Text
}

func (e IncidentEntity) Incident() engine.Incident {
	return engine.Incident{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		ElementId:         e.ElementId.Int32,
		ElementInstanceId: e.ElementInstanceId.Int32,
		JobId:             e.JobId.Int32,
		ProcessId:         e.ProcessId.Int32,
		ProcessInstanceId: e.ProcessInstanceId.Int32,
		TaskId:            e.TaskId.Int32,

		CreatedAt:  e.CreatedAt,
		CreatedBy:  e.CreatedBy,
		ResolvedAt: timeOrNil(e.ResolvedAt),
		ResolvedBy: e.ResolvedBy.String,
	}
}

type IncidentRepository interface {
	Insert(*IncidentEntity) error
	Select(partition time.Time, id int32) (*IncidentEntity, error)
	Update(*IncidentEntity) error

	Query(engine.IncidentCriteria, engine.QueryOptions) ([]any, error)
}

func ResolveIncident(ctx Context, cmd engine.ResolveIncidentCmd) error {
	if cmd.RetryCount <= 0 {
		cmd.RetryCount = 1
	}

	incident, err := ctx.Incidents().Select(time.Time(cmd.Partition), cmd.Id)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to resolve incident",
			Detail: fmt.Sprintf("incident %s/%d could not be found", cmd.Partition, cmd.Id),
		}
	}
	if err != nil {
		return err
	}
	if incident.ResolvedAt.Valid {
		return engine.Error{
			Type:   engine.ErrorConflict,
			Title:  "failed to resolve incident",
			Detail: fmt.Sprintf("incident %s/%d is resolved", cmd.Partition, cmd.Id),
		}
	}

	incident.ResolvedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
	incident.ResolvedBy = pgtype.Text{String: cmd.WorkerId, Valid: true}

	if err := ctx.Incidents().Update(incident); err != nil {
		return err
	}

	dueAt := cmd.RetryTimer.Calculate(ctx.Time())

	retryTimer := pgtype.Text{String: cmd.RetryTimer.String(), Valid: !cmd.RetryTimer.IsZero()}

	if incident.JobId.Valid {
		job, err := ctx.Jobs().Select(incident.Partition, incident.JobId.Int32)
		if err != nil {
			return err
		}

		retry := JobEntity{
			Partition: job.Partition,

			ElementId:         job.ElementId,
			ElementInstanceId: job.ElementInstanceId,
			ProcessId:         job.ProcessId,
			ProcessInstanceId: job.ProcessInstanceId,

			BpmnElementId:  job.BpmnElementId,
			CorrelationKey: job.CorrelationKey,
			CreatedAt:      ctx.Time(),
			CreatedBy:      cmd.WorkerId,
			DueAt:          dueAt,
			RetryCount:     cmd.RetryCount,
			RetryTimer:     retryTimer,
			Type:           job.Type,
		}

		return ctx.Jobs().Insert(&retry)
	} else {
		task, err := ctx.Tasks().Select(incident.Partition, incident.TaskId.Int32)
		if err != nil {
			return err
		}

		retry := TaskEntity{
			Partition: task.Partition,

			ElementId:         task.ElementId,
			ElementInstanceId: task.ElementInstanceId,
			EventId:           task.EventId,
			ProcessId:         task.ProcessId,
			ProcessInstanceId: task.ProcessInstanceId,

			CreatedAt:      ctx.Time(),
			CreatedBy:      cmd.WorkerId,
			DueAt:          dueAt,
			RetryCount:     cmd.RetryCount,
			RetryTimer:     retryTimer,
			SerializedTask: task.SerializedTask,
			Type:           task.Type,

			Instance: task.Instance,
		}

		return ctx.Tasks().Insert(&retry)
	}
}
