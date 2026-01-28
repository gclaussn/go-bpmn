package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
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

	BpmnElementId  string
	CompletedAt    pgtype.Timestamp
	CorrelationKey pgtype.Text
	CreatedAt      time.Time
	CreatedBy      string
	DueAt          time.Time
	Error          pgtype.Text
	LockedAt       pgtype.Timestamp
	LockedBy       pgtype.Text
	RetryCount     int
	State          engine.WorkState
	Type           engine.JobType
}

func (e JobEntity) Job() engine.Job {
	return engine.Job{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		ElementId:         e.ElementId,
		ElementInstanceId: e.ElementInstanceId,
		ProcessId:         e.ProcessId,
		ProcessInstanceId: e.ProcessInstanceId,

		BpmnElementId:  e.BpmnElementId,
		CompletedAt:    timeOrNil(e.CompletedAt),
		CorrelationKey: e.CorrelationKey.String,
		CreatedAt:      e.CreatedAt,
		CreatedBy:      e.CreatedBy,
		DueAt:          e.DueAt,
		Error:          e.Error.String,
		LockedAt:       timeOrNil(e.LockedAt),
		LockedBy:       e.LockedBy.String,
		RetryCount:     e.RetryCount,
		State:          e.State,
		Type:           e.Type,
	}
}

type JobRepository interface {
	Insert(*JobEntity) error
	Select(partition time.Time, id int32) (*JobEntity, error)
	Update(*JobEntity) error

	Query(engine.JobCriteria, engine.QueryOptions, time.Time) ([]engine.Job, error)

	Lock(cmd engine.LockJobsCmd, lockedAt time.Time) ([]*JobEntity, error)
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

	encryption := ctx.Options().Encryption

	variables := make([]*VariableEntity, 0, len(cmd.ProcessVariables)+len(cmd.ElementVariables))

	processVariableNames := make(map[string]bool, len(cmd.ProcessVariables))
	for _, variable := range cmd.ProcessVariables {
		if _, ok := processVariableNames[variable.Name]; ok {
			continue // skip already processed variable
		}

		processVariableNames[variable.Name] = true

		data := variable.Data
		if data == nil {
			variable := VariableEntity{ // with fields, needed for deletion
				Partition:         job.Partition,
				ProcessInstanceId: job.ProcessInstanceId,
				Name:              variable.Name,
			}

			variables = append(variables, &variable)
			continue
		}

		if err := encryption.EncryptData(data); err != nil {
			return engine.Job{}, fmt.Errorf("failed to encrypt process variable %s: %v", variable.Name, err)
		}

		variable := VariableEntity{
			Partition: job.Partition,

			ProcessId:         job.ProcessId,
			ProcessInstanceId: job.ProcessInstanceId,

			CreatedAt:   ctx.Time(),
			CreatedBy:   job.LockedBy.String,
			Encoding:    data.Encoding,
			IsEncrypted: data.IsEncrypted,
			Name:        variable.Name,
			UpdatedAt:   ctx.Time(),
			UpdatedBy:   job.LockedBy.String,
			Value:       data.Value,
		}

		variables = append(variables, &variable)
	}

	elementVariableNames := make(map[string]bool, len(cmd.ElementVariables))
	for _, variable := range cmd.ElementVariables {
		if _, ok := elementVariableNames[variable.Name]; ok {
			continue // skip already processed variable
		}

		elementVariableNames[variable.Name] = true

		data := variable.Data
		if data == nil {
			variable := VariableEntity{ // with fields, needed for deletion
				Partition:         job.Partition,
				ElementInstanceId: pgtype.Int4{Int32: job.ElementInstanceId, Valid: true},
				Name:              variable.Name,
			}

			variables = append(variables, &variable)
			continue
		}

		if err := encryption.EncryptData(data); err != nil {
			return engine.Job{}, fmt.Errorf("failed to encrypt element variable %s: %v", variable.Name, err)
		}

		variable := VariableEntity{
			Partition: job.Partition,

			ElementId:         pgtype.Int4{Int32: job.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: job.ElementInstanceId, Valid: true},
			ProcessId:         job.ProcessId,
			ProcessInstanceId: job.ProcessInstanceId,

			CreatedAt:   ctx.Time(),
			CreatedBy:   job.LockedBy.String,
			Encoding:    data.Encoding,
			IsEncrypted: data.IsEncrypted,
			Name:        variable.Name,
			UpdatedAt:   ctx.Time(),
			UpdatedBy:   job.LockedBy.String,
			Value:       data.Value,
		}

		variables = append(variables, &variable)
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

	for _, variable := range variables {
		if variable.Value == "" {
			if err := ctx.Variables().Delete(variable); err != nil {
				return engine.Job{}, err
			}
		} else {
			if err := ctx.Variables().Upsert(variable); err != nil {
				return engine.Job{}, err
			}
		}
	}

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
		retryTimer := engine.ISO8601Duration(cmd.RetryTimer)

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
			DueAt:          retryTimer.Calculate(ctx.Time()),
			RetryCount:     job.RetryCount + 1,
			State:          engine.WorkCreated,
			Type:           job.Type,
		}

		if err := ctx.Jobs().Insert(&retry); err != nil {
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

func LockJobs(ctx Context, cmd engine.LockJobsCmd) ([]engine.Job, error) {
	if cmd.Limit <= 0 {
		cmd.Limit = 1
	}

	lockedJobs, err := ctx.Jobs().Lock(cmd, ctx.Time())
	if err != nil {
		return nil, err
	}

	jobs := make([]engine.Job, len(lockedJobs))
	for i, lockedJob := range lockedJobs {
		jobs[i] = lockedJob.Job()
	}

	return jobs, nil
}

func UnlockJobs(ctx Context, cmd engine.UnlockJobsCmd) (int, error) {
	return ctx.Jobs().Unlock(cmd)
}
