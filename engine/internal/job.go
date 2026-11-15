package internal

import (
	"errors"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type JobEntity struct {
	Partition time.Time
	Id        int32

	ElementId         int32
	ElementInstanceId int32
	ProcessId         int32
	ProcessInstanceId int32

	BpmnElementId   string
	BpmnElementType model.ElementType
	CompletedAt     pgtype.Timestamp
	CreatedAt       time.Time
	CreatedBy       string
	DueAt           time.Time
	Error           pgtype.Text
	LockedAt        pgtype.Timestamp
	LockedBy        pgtype.Text
	RetryCount      int
	State           engine.WorkState
	Type            engine.JobType
}

func (e JobEntity) Job() engine.Job {
	return engine.Job{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		ElementId:         e.ElementId,
		ElementInstanceId: e.ElementInstanceId,
		ProcessId:         e.ProcessId,
		ProcessInstanceId: e.ProcessInstanceId,

		BpmnElementId:   e.BpmnElementId,
		BpmnElementType: e.BpmnElementType,
		CompletedAt:     e.CompletedAt.Time,
		CreatedAt:       e.CreatedAt,
		CreatedBy:       e.CreatedBy,
		DueAt:           e.DueAt,
		Error:           e.Error.String,
		LockedAt:        e.LockedAt.Time,
		LockedBy:        e.LockedBy.String,
		RetryCount:      e.RetryCount,
		State:           e.State,
		Type:            e.Type,
	}
}

// JobPoolEntity represents a job that is either available to be locked or locked by a worker.
type JobPoolEntity struct {
	Id int64

	Partition time.Time // Partition of the related job

	ElementId         int32
	ElementInstanceId int32
	JobId             int32
	ProcessId         int32
	ProcessInstanceId int32

	BpmnElementId   string
	BpmnElementType model.ElementType
	CorrelationKey  pgtype.Text
	DueAt           time.Time
	LockedAt        pgtype.Timestamp
	LockedBy        pgtype.Text
	RetryCount      int
	Type            engine.JobType
}

func (e JobPoolEntity) LockedJob() engine.LockedJob {
	return engine.LockedJob{
		Partition: engine.Partition(e.Partition),
		Id:        e.JobId,

		ElementId:         e.ElementId,
		ElementInstanceId: e.ElementInstanceId,
		ProcessId:         e.ProcessId,
		ProcessInstanceId: e.ProcessInstanceId,

		BpmnElementId:   e.BpmnElementId,
		BpmnElementType: e.BpmnElementType,
		CorrelationKey:  e.CorrelationKey.String,
		DueAt:           e.DueAt,
		LockedAt:        e.LockedAt.Time,
		LockedBy:        e.LockedBy.String,
		RetryCount:      e.RetryCount,
		Type:            e.Type,
	}
}

type JobRepository interface {
	// Insert inserts a job.
	//
	// Additionally the job is inserted into the job pool - so it can be locked when due.
	// For the execution of a locked job, the correlation key of the related process instance is needed.
	Insert(entity *JobEntity, correlationKey pgtype.Text) error

	Select(partition time.Time, id int32) (*JobEntity, error)
	Update(*JobEntity) error

	Query(engine.JobCriteria, engine.QueryOptions, time.Time) ([]engine.Job, error)

	Lock(cmd engine.LockJobsCmd, lockedAt time.Time) ([]JobPoolEntity, error)
	Unlock(engine.UnlockJobsCmd) (int, error)
}

func CompleteJob(ctx Context, cmd engine.CompleteJobCmd) (engine.Job, error) {
	processInstance, err := ctx.ProcessInstances().SelectByJob(time.Time(cmd.Partition), cmd.Id)
	if err == pgx.ErrNoRows {
		return engine.Job{}, engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to complete job",
			Detail: fmt.Sprintf("job %s/%d could not be found", cmd.Partition, cmd.Id),
		}
	}
	if err != nil {
		return engine.Job{}, err
	}

	job, err := ctx.Jobs().Select(processInstance.Partition, cmd.Id)
	if err != nil {
		return engine.Job{}, err
	}
	if job.CompletedAt.Valid {
		return engine.Job{}, engine.Error{
			Type:   engine.ErrorConflict,
			Title:  "failed to complete job",
			Detail: fmt.Sprintf("job %s/%d is completed", cmd.Partition, cmd.Id),
		}
	}
	if !job.LockedAt.Valid {
		return engine.Job{}, engine.Error{
			Type:   engine.ErrorConflict,
			Title:  "failed to complete job",
			Detail: fmt.Sprintf("job %s/%d is not locked", cmd.Partition, cmd.Id),
		}
	}
	if job.LockedBy.String != cmd.WorkerId {
		return engine.Job{}, engine.Error{
			Type:  engine.ErrorConflict,
			Title: "failed to complete job",
			Detail: fmt.Sprintf(
				"job %s/%d is not locked by worker %s, but %s",
				cmd.Partition,
				cmd.Id,
				cmd.WorkerId,
				job.LockedBy.String,
			),
		}
	}

	execution, err := ctx.ElementInstances().Select(job.Partition, job.ElementInstanceId)
	if err != nil {
		return engine.Job{}, err
	}
	if execution.EndedAt.Valid {
		job.CompletedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		job.State = engine.WorkCanceled

		if err := ctx.Jobs().Update(job); err != nil {
			return engine.Job{}, err
		}

		return job.Job(), nil
	}

	elementVariables, err := mapElementVariables(ctx, execution, cmd.ElementVariables, cmd.WorkerId)
	if err != nil {
		return engine.Job{}, err
	}

	processVariables, err := mapProcessVariables(ctx, processInstance, cmd.ProcessVariables, cmd.WorkerId)
	if err != nil {
		return engine.Job{}, err
	}

	if cmd.Error != "" {
		job.Error = pgtype.Text{String: cmd.Error, Valid: true}
	}

	if !job.Error.Valid {
		process, err := ctx.ProcessCache().GetOrCacheById(ctx, processInstance.ProcessId)
		if err != nil {
			return engine.Job{}, err
		}

		scope, err := ctx.ElementInstances().Select(execution.Partition, execution.ParentId.Int32)
		if err != nil {
			return engine.Job{}, err
		}

		execution.parent = scope

		ec := executionContext{
			engineOrWorkerId: cmd.WorkerId,
			executions:       []*ElementInstanceEntity{scope, execution},
			process:          process,
			processInstance:  processInstance,
		}

		if err := ec.handleJob(ctx, job, cmd); err != nil {
			return engine.Job{}, err
		}
	}

	// set or delete element variables
	for _, variable := range elementVariables {
		if variable.ElementId.Valid {
			if err := ctx.Variables().Upsert(variable); err != nil {
				return engine.Job{}, err
			}
		} else {
			if err := ctx.Variables().Delete(variable); err != nil {
				return engine.Job{}, err
			}
		}
	}

	// set or delete process variables
	for _, variable := range processVariables {
		if variable.ProcessId != 0 {
			if err := ctx.Variables().Upsert(variable); err != nil {
				return engine.Job{}, err
			}
		} else {
			if err := ctx.Variables().Delete(variable); err != nil {
				return engine.Job{}, err
			}
		}
	}

	// update job
	job.CompletedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}

	if !job.Error.Valid {
		job.State = engine.WorkDone
	} else if cmd.RetryLimit > job.RetryCount {
		job.State = engine.WorkCausedRetry
	} else {
		job.State = engine.WorkCausedIncident
	}

	if err := ctx.Jobs().Update(job); err != nil {
		return engine.Job{}, err
	}

	if !job.Error.Valid {
		return job.Job(), nil
	}

	if cmd.RetryLimit > job.RetryCount {
		ec := executionContext{
			engineOrWorkerId: cmd.WorkerId,
			processInstance:  processInstance,
		}

		retryTimer := engine.ISO8601Duration(cmd.RetryTimer)
		if err := ec.retryJob(ctx, job, job.RetryCount+1, retryTimer); err != nil {
			return engine.Job{}, err
		}
	} else {
		incident := IncidentEntity{
			Partition: job.Partition,

			ElementId:         pgtype.Int4{Int32: job.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: job.ElementInstanceId, Valid: true},
			JobId:             pgtype.Int4{Int32: job.Id, Valid: true},
			ProcessId:         pgtype.Int4{Int32: job.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: job.ProcessInstanceId, Valid: true},

			CreatedAt: ctx.Time(),
			CreatedBy: cmd.WorkerId,
		}

		if err := ctx.Incidents().Insert(&incident); err != nil {
			return engine.Job{}, err
		}
	}

	return job.Job(), nil
}

func LockJobs(ctx Context, cmd engine.LockJobsCmd) ([]engine.LockedJob, error) {
	if len(cmd.ProcessIds) == 0 {
		return nil, errors.New("no process ID specified")
	}

	if cmd.Limit <= 0 {
		cmd.Limit = 1
	}

	entities, err := ctx.Jobs().Lock(cmd, ctx.Time())
	if err != nil {
		return nil, err
	}

	lockedJobs := make([]engine.LockedJob, len(entities))
	for i, entity := range entities {
		lockedJobs[i] = entity.LockedJob()
	}

	// TODO sort by dueAt

	return lockedJobs, nil
}

func UnlockJobs(ctx Context, cmd engine.UnlockJobsCmd) (int, error) {
	return ctx.Jobs().Unlock(cmd)
}

// TODO
func (ec *executionContext) retryJob(ctx Context, job *JobEntity, retryCount int, retryTimer engine.ISO8601Duration) error {
	dueAt := retryTimer.Calculate(ctx.Time())

	entity := JobEntity{
		Partition: job.Partition,

		ElementId:         job.ElementId,
		ElementInstanceId: job.ElementInstanceId,
		ProcessId:         job.ProcessId,
		ProcessInstanceId: job.ProcessInstanceId,

		BpmnElementId:   job.BpmnElementId,
		BpmnElementType: job.BpmnElementType,
		CreatedAt:       ctx.Time(),
		CreatedBy:       ec.engineOrWorkerId,
		DueAt:           dueAt,
		RetryCount:      retryCount,
		State:           engine.WorkCreated,
		Type:            job.Type,
	}

	if err := ctx.Jobs().Insert(
		&entity,
		ec.processInstance.CorrelationKey,
	); err != nil {
		return fmt.Errorf("failed to retry job %s/%d: %v", job.Partition.Format(time.DateOnly), job.Id, err)
	}

	return nil
}
