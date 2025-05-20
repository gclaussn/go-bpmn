package internal

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProcessInstanceEntity struct {
	Partition time.Time
	Id        int32

	ParentId pgtype.Int4
	RootId   pgtype.Int4

	ProcessId int32

	BpmnProcessId  string
	CorrelationKey pgtype.Text
	CreatedAt      time.Time
	CreatedBy      string
	EndedAt        pgtype.Timestamp
	StartedAt      pgtype.Timestamp
	State          engine.InstanceState
	StateChangedBy string
	Tags           pgtype.Text
	Version        string
}

func (e ProcessInstanceEntity) ProcessInstance() engine.ProcessInstance {
	var tags map[string]string
	if e.Tags.Valid {
		_ = json.Unmarshal([]byte(e.Tags.String), &tags)
	}

	return engine.ProcessInstance{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		ParentId: e.ParentId.Int32,
		RootId:   e.RootId.Int32,

		ProcessId: e.ProcessId,

		BpmnProcessId:  e.BpmnProcessId,
		CorrelationKey: e.CorrelationKey.String,
		CreatedAt:      e.CreatedAt,
		CreatedBy:      e.CreatedBy,
		EndedAt:        timeOrNil(e.EndedAt),
		StartedAt:      timeOrNil(e.StartedAt),
		State:          e.State,
		StateChangedBy: e.StateChangedBy,
		Tags:           tags,
		Version:        e.Version,
	}
}

type ProcessInstanceRepository interface {
	Insert(*ProcessInstanceEntity) error
	Select(partition time.Time, id int32) (*ProcessInstanceEntity, error)
	SelectByElementInstance(partition time.Time, elementInstanceId int32) (*ProcessInstanceEntity, error)
	SelectByJob(partition time.Time, jobId int32) (*ProcessInstanceEntity, error)
	Update(*ProcessInstanceEntity) error

	Query(engine.ProcessInstanceCriteria, engine.QueryOptions) ([]any, error)
}

func CreateProcessInstance(ctx Context, cmd engine.CreateProcessInstanceCmd) (engine.ProcessInstance, error) {
	process, err := ctx.ProcessCache().GetOrCache(ctx, cmd.BpmnProcessId, cmd.Version)
	if err == pgx.ErrNoRows {
		return engine.ProcessInstance{}, engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to create process instance",
			Detail: fmt.Sprintf("process %s:%s could not be found", cmd.BpmnProcessId, cmd.Version),
		}
	}
	if err != nil {
		return engine.ProcessInstance{}, err
	}

	var tags string
	if len(cmd.Tags) != 0 {
		b, err := json.Marshal(cmd.Tags)
		if err != nil {
			return engine.ProcessInstance{}, fmt.Errorf("failed to marshal tags: %v", err)
		}
		tags = string(b)
	}

	processInstance := ProcessInstanceEntity{
		Partition: ctx.Date(),

		ProcessId: process.Id,

		BpmnProcessId:  process.BpmnProcessId,
		CorrelationKey: pgtype.Text{String: cmd.CorrelationKey, Valid: cmd.CorrelationKey != ""},
		CreatedAt:      ctx.Time(),
		CreatedBy:      cmd.WorkerId,
		StartedAt:      pgtype.Timestamp{Time: ctx.Time(), Valid: true},
		State:          engine.InstanceStarted,
		StateChangedBy: cmd.WorkerId,
		Tags:           pgtype.Text{String: tags, Valid: tags != ""},
		Version:        process.Version,
	}

	if err := ctx.ProcessInstances().Insert(&processInstance); err != nil {
		return engine.ProcessInstance{}, err
	}

	encryption := ctx.Options().Encryption

	variables := make([]*VariableEntity, 0, len(cmd.Variables))
	for variableName, data := range cmd.Variables {
		if data == nil {
			continue
		}

		if err := encryption.EncryptData(data); err != nil {
			return engine.ProcessInstance{}, fmt.Errorf("failed to encrypt variable: %v", err)
		}

		variable := VariableEntity{
			Partition: processInstance.Partition,

			ProcessId:         process.Id,
			ProcessInstanceId: processInstance.Id,

			CreatedAt:   processInstance.CreatedAt,
			CreatedBy:   cmd.WorkerId,
			Encoding:    data.Encoding,
			IsEncrypted: data.IsEncrypted,
			Name:        variableName,
			UpdatedAt:   processInstance.CreatedAt,
			UpdatedBy:   cmd.WorkerId,
			Value:       data.Value,
		}

		variables = append(variables, &variable)
	}

	if err := enqueueProcessInstance(ctx, &processInstance); err != nil {
		return engine.ProcessInstance{}, fmt.Errorf("failed to enqueue process instance: %v", err)
	}

	scope := process.graph.createProcessScope(&processInstance)

	execution, err := process.graph.createExecution(&scope)
	if err != nil {
		return engine.ProcessInstance{}, engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create execution",
			Detail: err.Error(),
		}
	}

	ec := executionContext{
		engineOrWorkerId: cmd.WorkerId,
		process:          process,
		processInstance:  &processInstance,
	}

	executions := []*ElementInstanceEntity{&scope, &execution}
	if err := ec.continueExecutions(ctx, executions); err != nil {
		return engine.ProcessInstance{}, err
	}

	for _, variable := range variables {
		if err := ctx.Variables().Insert(variable); err != nil {
			return engine.ProcessInstance{}, err
		}
	}

	return processInstance.ProcessInstance(), nil
}

func ResumeProcessInstance(ctx Context, cmd engine.ResumeProcessInstanceCmd) error {
	processInstance, err := ctx.ProcessInstances().Select(time.Time(cmd.Partition), cmd.Id)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to resume process instance",
			Detail: fmt.Sprintf("process instance %s/%d could not be found", cmd.Partition, cmd.Id),
		}
	}
	if err != nil {
		return err
	}
	if processInstance.State != engine.InstanceSuspended {
		return engine.Error{
			Type:  engine.ErrorConflict,
			Title: "failed to resume process instance",
			Detail: fmt.Sprintf(
				"process instance %s/%d is not in state %s, but %s",
				cmd.Partition,
				cmd.Id,
				engine.InstanceSuspended,
				processInstance.State,
			),
		}
	}

	process, err := ctx.ProcessCache().GetOrCacheById(ctx, processInstance.ProcessId)
	if err != nil {
		return err
	}

	ec := executionContext{
		engineOrWorkerId: cmd.WorkerId,
		process:          process,
		processInstance:  processInstance,
	}

	executions, err := ctx.ElementInstances().SelectByProcessInstanceAndState(processInstance)
	if err != nil {
		return err
	}

	for _, execution := range executions {
		if execution.ExecutionCount == 0 {
			continue // skip non scopes
		}

		execution.State = engine.InstanceStarted
		execution.StateChangedBy = ec.engineOrWorkerId
	}

	processInstance.State = engine.InstanceStarted
	processInstance.StateChangedBy = ec.engineOrWorkerId

	if err := ec.continueExecutions(ctx, executions); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", executions, err)
		}
	}

	return ctx.ProcessInstances().Update(processInstance)
}

func SuspendProcessInstance(ctx Context, cmd engine.SuspendProcessInstanceCmd) error {
	processInstance, err := ctx.ProcessInstances().Select(time.Time(cmd.Partition), cmd.Id)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to suspend process instance",
			Detail: fmt.Sprintf("process instance %s/%d could not be found", cmd.Partition, cmd.Id),
		}
	}
	if err != nil {
		return err
	}
	if processInstance.State != engine.InstanceStarted {
		return engine.Error{
			Type:  engine.ErrorConflict,
			Title: "failed to suspend process instance",
			Detail: fmt.Sprintf(
				"process instance %s/%d is not in state %s, but %s",
				cmd.Partition,
				cmd.Id,
				engine.InstanceStarted,
				processInstance.State,
			),
		}
	}

	executions, err := ctx.ElementInstances().SelectByProcessInstanceAndState(processInstance)
	if err != nil {
		return err
	}

	for _, execution := range executions {
		if execution.ExecutionCount == 0 {
			continue // skip non scopes
		}

		execution.State = engine.InstanceSuspended
		execution.StateChangedBy = cmd.WorkerId

		if err := ctx.ElementInstances().Update(execution); err != nil {
			return err
		}
	}

	processInstance.State = engine.InstanceSuspended
	processInstance.StateChangedBy = cmd.WorkerId

	return ctx.ProcessInstances().Update(processInstance)
}

type StartProcessInstanceTask struct {
}

func (t StartProcessInstanceTask) Execute(ctx Context, task *TaskEntity) error {
	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err != nil {
		return fmt.Errorf("failed to select process instance %s/%d: %v", task.Partition.Format(time.DateOnly), task.ProcessInstanceId.Int32, err)
	}
	if processInstance.State != engine.InstanceQueued {
		return nil
	}

	process, err := ctx.ProcessCache().GetOrCacheById(ctx, processInstance.ProcessId)
	if err != nil {
		return err
	}

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		process:          process,
		processInstance:  processInstance,
	}

	executions, err := ctx.ElementInstances().SelectByProcessInstanceAndState(processInstance)
	if err != nil {
		return err
	}

	for _, execution := range executions {
		if execution.ExecutionCount == 0 {
			continue // skip non scopes
		}

		execution.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		execution.State = engine.InstanceStarted
		execution.StateChangedBy = ec.engineOrWorkerId
	}

	processInstance.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
	processInstance.State = engine.InstanceStarted
	processInstance.StateChangedBy = ec.engineOrWorkerId

	if err := ec.continueExecutions(ctx, executions); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", executions, err)
		}
	}

	return ctx.ProcessInstances().Update(processInstance)
}
