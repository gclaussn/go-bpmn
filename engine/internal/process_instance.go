package internal

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ProcessInstanceEntity struct {
	Partition time.Time
	Id        int32

	ParentId pgtype.Int4
	RootId   pgtype.Int4

	MessageId pgtype.Int8 // internal field
	ProcessId int32

	BpmnProcessId  string
	CorrelationKey pgtype.Text
	CreatedAt      time.Time
	CreatedBy      string
	EndedAt        pgtype.Timestamp
	StartedAt      pgtype.Timestamp
	State          engine.InstanceState
	Tags           pgtype.Text
	Version        string
}

func (e ProcessInstanceEntity) ProcessInstance() engine.ProcessInstance {
	var tags []engine.Tag
	if e.Tags.Valid {
		var tagMap map[string]string
		_ = json.Unmarshal([]byte(e.Tags.String), &tagMap)

		tagNames := slices.Sorted(maps.Keys(tagMap))

		tags = make([]engine.Tag, len(tagNames))
		for i, tagName := range tagNames {
			tags[i] = engine.Tag{
				Name:  tagName,
				Value: tagMap[tagName],
			}
		}
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

	Query(engine.ProcessInstanceCriteria, engine.QueryOptions) ([]engine.ProcessInstance, error)
}

func CreateProcessInstance(ctx Context, cmd engine.CreateProcessInstanceCmd, parent *ProcessInstanceEntity, parentExecution *ElementInstanceEntity) (engine.ProcessInstance, error) {
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
		tagMap := make(map[string]string, len(cmd.Tags))
		for _, tag := range cmd.Tags {
			tagMap[tag.Name] = tag.Value
		}

		b, err := json.Marshal(tagMap)
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
		Tags:           pgtype.Text{String: tags, Valid: tags != ""},
		Version:        process.Version,
	}

	if parent != nil {
		rootId := parent.RootId
		if !rootId.Valid {
			rootId = pgtype.Int4{Int32: parent.Id, Valid: true}
		}

		processInstance.Partition = parent.Partition
		processInstance.ParentId = pgtype.Int4{Int32: parent.Id, Valid: true}
		processInstance.RootId = rootId
		processInstance.State = parent.State // possible transition to suspended
	}

	if err := ctx.ProcessInstances().Insert(&processInstance); err != nil {
		return engine.ProcessInstance{}, err
	}

	encryption := ctx.Options().Encryption

	variables := make([]*VariableEntity, 0, len(cmd.Variables))
	variableNames := make(map[string]bool, len(cmd.Variables))
	for _, variable := range cmd.Variables {
		if _, ok := variableNames[variable.Name]; ok {
			continue // skip already processed variable
		}

		variableNames[variable.Name] = true

		data := variable.Data
		if data == nil {
			continue
		}

		if err := encryption.EncryptData(data); err != nil {
			return engine.ProcessInstance{}, fmt.Errorf("failed to encrypt variable %s: %v", variable.Name, err)
		}

		variable := VariableEntity{
			Partition: processInstance.Partition,

			ProcessId:         process.Id,
			ProcessInstanceId: processInstance.Id,

			CreatedAt:   processInstance.CreatedAt,
			CreatedBy:   processInstance.CreatedBy,
			Encoding:    data.Encoding,
			IsEncrypted: data.IsEncrypted,
			Name:        variable.Name,
			UpdatedAt:   processInstance.CreatedAt,
			UpdatedBy:   processInstance.CreatedBy,
			Value:       data.Value,
		}

		variables = append(variables, &variable)
	}

	if err := enqueueProcessInstance(ctx, &processInstance); err != nil {
		return engine.ProcessInstance{}, fmt.Errorf("failed to enqueue process instance: %v", err)
	}

	scope := process.graph.createProcessScope(&processInstance)

	if parentExecution != nil {
		scope.parent = parentExecution
	}

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
		executions:       []*ElementInstanceEntity{&scope, &execution},
		process:          process,
		processInstance:  &processInstance,
	}

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return engine.ProcessInstance{}, err
		} else {
			return engine.ProcessInstance{}, fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
		}
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

	executions, err := ctx.ElementInstances().SelectByProcessInstanceAndState(processInstance)
	if err != nil {
		return err
	}

	for _, execution := range executions {
		if execution.ExecutionCount <= 0 {
			continue // skip non scope
		}

		execution.State = engine.InstanceStarted
	}

	processInstance.State = engine.InstanceStarted

	ec := executionContext{
		engineOrWorkerId: cmd.WorkerId,
		executions:       executions,
		process:          process,
		processInstance:  processInstance,
	}

	if err := ec.continueExecutions(ctx); err != nil {
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
		if execution.ExecutionCount <= 0 {
			continue // skip non scope
		}

		execution.State = engine.InstanceSuspended

		if err := ctx.ElementInstances().Update(execution); err != nil {
			return err
		}
	}

	processInstance.State = engine.InstanceSuspended

	return ctx.ProcessInstances().Update(processInstance)
}

type DequeueProcessInstanceTask struct {
	BpmnProcessId string
}

func (t DequeueProcessInstanceTask) Execute(ctx Context, task *TaskEntity) error {
	queue, err := ctx.ProcessInstanceQueues().Select(t.BpmnProcessId)
	if err != nil {
		return err
	}

	if !queue.MustDequeue() {
		task.State = engine.WorkCanceled
		return nil
	}

	head, err := ctx.ProcessInstanceQueues().SelectElement(queue.HeadPartition.Time, queue.HeadId.Int32)
	if err != nil {
		return err
	}

	if head.NextId.Valid {
		queue.HeadId = head.NextId
		queue.HeadPartition = head.NextPartition
	} else {
		queue.HeadId = pgtype.Int4{}
		queue.HeadPartition = pgtype.Date{}
		queue.TailId = pgtype.Int4{}
		queue.TailPartition = pgtype.Date{}
	}

	startProcessInstance := TaskEntity{
		Partition: head.Partition,

		ProcessId:         pgtype.Int4{Int32: head.ProcessId, Valid: true},
		ProcessInstanceId: pgtype.Int4{Int32: head.Id, Valid: true},

		CreatedAt: ctx.Time(),
		CreatedBy: ctx.Options().EngineId,
		DueAt:     ctx.Time(),
		State:     engine.WorkCreated,
		Type:      engine.TaskStartProcessInstance,

		Instance: StartProcessInstanceTask{},
	}

	if err := ctx.Tasks().Insert(&startProcessInstance); err != nil {
		return err
	}

	queue.ActiveCount = queue.ActiveCount + 1
	queue.QueuedCount = queue.QueuedCount - 1

	if err := ctx.ProcessInstanceQueues().Update(queue); err != nil {
		return err
	}

	if !queue.MustDequeue() {
		return nil
	}

	dequeueProcessInstance := TaskEntity{
		Partition: ctx.Date(),

		ProcessId: task.ProcessId,

		CreatedAt: ctx.Time(),
		CreatedBy: ctx.Options().EngineId,
		DueAt:     ctx.Time(),
		State:     engine.WorkCreated,
		Type:      engine.TaskDequeueProcessInstance,

		Instance: DequeueProcessInstanceTask{BpmnProcessId: t.BpmnProcessId},
	}

	return ctx.Tasks().Insert(&dequeueProcessInstance)
}

type StartProcessInstanceTask struct {
}

func (t StartProcessInstanceTask) Execute(ctx Context, task *TaskEntity) error {
	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to start process instance",
			Detail: fmt.Sprintf("process instance %s/%d could not be found", task.Partition.Format(time.DateOnly), task.ProcessInstanceId.Int32),
		}
	}
	if err != nil {
		return err
	}
	if processInstance.State != engine.InstanceQueued {
		task.State = engine.WorkCanceled
		return nil
	}

	process, err := ctx.ProcessCache().GetOrCacheById(ctx, processInstance.ProcessId)
	if err != nil {
		return err
	}

	executions, err := ctx.ElementInstances().SelectByProcessInstanceAndState(processInstance)
	if err != nil {
		return err
	}

	for _, execution := range executions {
		if execution.ExecutionCount <= 0 {
			continue // skip non scope
		}

		execution.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
		execution.State = engine.InstanceStarted
	}

	processInstance.StartedAt = pgtype.Timestamp{Time: ctx.Time(), Valid: true}
	processInstance.State = engine.InstanceStarted

	ec := executionContext{
		engineOrWorkerId: ctx.Options().EngineId,
		executions:       executions,
		process:          process,
		processInstance:  processInstance,
	}

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
		}
	}

	return ctx.ProcessInstances().Update(processInstance)
}

type TerminateProcessInstanceTask struct {
}

func (t TerminateProcessInstanceTask) Execute(ctx Context, task *TaskEntity) error {
	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to terminate process instance",
			Detail: fmt.Sprintf("process instance %s/%d could not be found", task.Partition.Format(time.DateOnly), task.ProcessInstanceId.Int32),
		}
	}
	if err != nil {
		return err
	}
	if processInstance.EndedAt.Valid {
		task.State = engine.WorkCanceled
		return nil
	}

	return terminateProcessInstance(ctx, processInstance)
}

func terminateProcessInstance(ctx Context, processInstance *ProcessInstanceEntity) error {
	executions, err := ctx.ElementInstances().SelectActive(processInstance)
	if err != nil {
		return err
	}

	now := pgtype.Timestamp{Time: ctx.Time(), Valid: true}

	// terminate executions
	var terminateProcessInstanceTasks []*TaskEntity
	for _, execution := range executions {
		execution.EndedAt = now
		execution.ExecutionCount = 0
		execution.State = engine.InstanceTerminated

		if execution.BpmnElementType != model.ElementCallActivity {
			continue
		}

		children, err := ctx.ElementInstances().SelectActiveChildren(execution)
		if err != nil {
			return err
		}

		if len(children) == 0 {
			continue // process scope already ended
		}

		processScope := children[0]

		terminateProcessInstanceTask := TaskEntity{
			Partition: processScope.Partition,

			ProcessId:         pgtype.Int4{Int32: processScope.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: processScope.ProcessInstanceId, Valid: true},

			CreatedAt: ctx.Time(),
			CreatedBy: ctx.Options().EngineId,
			DueAt:     ctx.Time(),
			State:     engine.WorkCreated,
			Type:      engine.TaskTerminateProcessInstance,

			Instance: TerminateProcessInstanceTask{},
		}

		terminateProcessInstanceTasks = append(terminateProcessInstanceTasks, &terminateProcessInstanceTask)
	}

	if err := ctx.ElementInstances().UpdateBatch(executions); err != nil {
		return err
	}

	if err := ctx.Tasks().InsertBatch(terminateProcessInstanceTasks); err != nil {
		return err
	}

	// cancel message subscriptions
	messageSubscriptions, err := ctx.MessageSubscriptions().SelectByProcessInstance(processInstance)
	if err != nil {
		return err
	}

	for _, messageSubscription := range messageSubscriptions {
		if err := ctx.MessageSubscriptions().Delete(messageSubscription); err != nil {
			return err
		}
	}

	// cancel signal subscriptions
	signalSubscriptions, err := ctx.SignalSubscriptions().SelectByProcessInstance(processInstance)
	if err != nil {
		return err
	}

	for _, signalSubscription := range signalSubscriptions {
		if err := ctx.SignalSubscriptions().Delete(signalSubscription); err != nil {
			return err
		}
	}

	processInstance.EndedAt = now
	processInstance.State = engine.InstanceTerminated

	if err := ctx.ProcessInstances().Update(processInstance); err != nil {
		return err
	}

	if err := dequeueProcessInstance(ctx, processInstance, ctx.Options().EngineId); err != nil {
		return fmt.Errorf("failed to dequeue process instance: %v", err)
	}

	return nil
}
