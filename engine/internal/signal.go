package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

type SignalEntity struct {
	Id int64

	ActiveSubscriberCount int // internal field
	CreatedAt             time.Time
	CreatedBy             string
	Name                  string
	SubscriberCount       int
}

func (e SignalEntity) Signal() engine.Signal {
	return engine.Signal{
		Id: e.Id,

		CreatedAt:       e.CreatedAt,
		CreatedBy:       e.CreatedBy,
		Name:            e.Name,
		SubscriberCount: e.SubscriberCount,
	}
}

type SignalRepository interface {
	Insert(*SignalEntity) error
	Select(id int64) (*SignalEntity, error)
	Update(*SignalEntity) error
}

type SignalSubscriptionEntity struct {
	Id int64

	Partition time.Time // Partition of the related process and element instance

	ElementId         int32
	ElementInstanceId int32
	ProcessId         int32
	ProcessInstanceId int32

	CreatedAt time.Time
	CreatedBy string
	Name      string
}

type SignalSubscriptionRepository interface {
	DeleteByName(name string) ([]*SignalSubscriptionEntity, error)
	Insert(*SignalSubscriptionEntity) error
}

type SignalVariableEntity struct {
	Id int64

	SignalId int64

	Encoding    pgtype.Text // NULL, when a process instance variable should be deleted
	IsEncrypted pgtype.Bool // NULL, when a process instance variable should be deleted
	Name        string
	Value       pgtype.Text // NULL, when a process instance variable should be deleted
}

type SignalVariableRepository interface {
	InsertBatch([]*SignalVariableEntity) error
	SelectBySignalId(signalId int64) ([]*SignalVariableEntity, error)
}

func SendSignal(ctx Context, cmd engine.SendSignalCmd) (engine.Signal, error) {
	// encrypt variables
	encryption := ctx.Options().Encryption

	for variableName, data := range cmd.Variables {
		if data == nil {
			continue
		}
		if err := encryption.EncryptData(data); err != nil {
			return engine.Signal{}, fmt.Errorf("failed to encrypt variable %s: %v", variableName, err)
		}
	}

	// find subscribers
	signalSubscriptions, err := ctx.SignalSubscriptions().DeleteByName(cmd.Name)
	if err != nil {
		return engine.Signal{}, err
	}

	eventDefinitions, err := ctx.EventDefinitions().SelectBySignalName(cmd.Name)
	if err != nil {
		return engine.Signal{}, err
	}

	subscriberCount := len(signalSubscriptions) + len(eventDefinitions)

	// insert signal
	signal := SignalEntity{
		ActiveSubscriberCount: subscriberCount,
		CreatedAt:             ctx.Time(),
		CreatedBy:             cmd.WorkerId,
		Name:                  cmd.Name,
		SubscriberCount:       subscriberCount,
	}

	if err := ctx.Signals().Insert(&signal); err != nil {
		return engine.Signal{}, err
	}

	// insert signal variables
	signalVariables := make([]*SignalVariableEntity, 0, len(cmd.Variables))
	for variableName, data := range cmd.Variables {
		if data != nil {
			signalVariables = append(signalVariables, &SignalVariableEntity{
				SignalId: signal.Id,

				Encoding:    pgtype.Text{String: data.Encoding, Valid: true},
				IsEncrypted: pgtype.Bool{Bool: data.IsEncrypted, Valid: true},
				Name:        variableName,
				Value:       pgtype.Text{String: data.Value, Valid: true},
			})
		} else {
			signalVariables = append(signalVariables, &SignalVariableEntity{
				SignalId: signal.Id,

				Name: variableName,
			})
		}
	}

	if err := ctx.SignalVariables().InsertBatch(signalVariables); err != nil {
		return engine.Signal{}, err
	}

	// insert trigger event tasks
	triggerEventTasks := make([]*TaskEntity, 0, signal.SubscriberCount)
	for _, signalSubscription := range signalSubscriptions {
		triggerEventTasks = append(triggerEventTasks, &TaskEntity{
			Partition: signalSubscription.Partition,

			ElementId:         pgtype.Int4{Int32: signalSubscription.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: signalSubscription.ElementInstanceId, Valid: true},
			ProcessId:         pgtype.Int4{Int32: signalSubscription.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: signalSubscription.ProcessInstanceId, Valid: true},

			CreatedAt: signal.CreatedAt,
			CreatedBy: signal.CreatedBy,
			DueAt:     signal.CreatedAt,
			Type:      engine.TaskTriggerEvent,

			Instance: TriggerEventTask{SignalId: signal.Id},
		})
	}
	for _, eventDefinition := range eventDefinitions {
		if eventDefinition.BpmnElementType != model.ElementSignalStartEvent {
			continue
		}

		triggerEventTasks = append(triggerEventTasks, &TaskEntity{
			Partition: ctx.Date(),

			ElementId: pgtype.Int4{Int32: eventDefinition.ElementId, Valid: true},
			ProcessId: pgtype.Int4{Int32: eventDefinition.ProcessId, Valid: true},

			CreatedAt: signal.CreatedAt,
			CreatedBy: signal.CreatedBy,
			DueAt:     signal.CreatedAt,
			Type:      engine.TaskTriggerEvent,

			Instance: TriggerEventTask{SignalId: signal.Id},
		})
	}

	if err := ctx.Tasks().InsertBatch(triggerEventTasks); err != nil {
		return engine.Signal{}, err
	}

	return signal.Signal(), nil
}

func triggerSignalCatchEvent(ctx Context, task *TaskEntity, signalId int64) error {
	processInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
	if err != nil {
		return err
	}

	if processInstance.EndedAt.Valid {
		return nil
	}

	execution, err := ctx.ElementInstances().Select(task.Partition, task.ElementInstanceId.Int32)
	if err != nil {
		return err
	}

	if execution.EndedAt.Valid {
		return nil
	}

	process, err := ctx.ProcessCache().GetOrCacheById(ctx, task.ProcessId.Int32)
	if err != nil {
		return err
	}

	signal, err := ctx.Signals().Select(signalId)
	if err != nil {
		return err
	}

	ec := executionContext{
		engineOrWorkerId: signal.CreatedBy,
		process:          process,
		processInstance:  processInstance,
	}

	if err := ec.continueExecutions(ctx, []*ElementInstanceEntity{execution}); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue execution %+v: %v", execution, err)
		}
	}

	signalVariables, err := ctx.SignalVariables().SelectBySignalId(signalId)
	if err != nil {
		return err
	}

	for _, variable := range signalVariables {
		if !variable.Value.Valid {
			if err := ctx.Variables().Delete(&VariableEntity{ // with fields, needed for deletion
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
				Name:              variable.Name,
			}); err != nil {
				return err
			}

			continue
		}

		if err := ctx.Variables().Insert(&VariableEntity{
			Partition: processInstance.Partition,

			ProcessId:         processInstance.ProcessId,
			ProcessInstanceId: processInstance.Id,

			CreatedAt:   ctx.Time(),
			CreatedBy:   signal.CreatedBy,
			Encoding:    variable.Encoding.String,
			IsEncrypted: variable.IsEncrypted.Bool,
			Name:        variable.Name,
			UpdatedAt:   ctx.Time(),
			UpdatedBy:   signal.CreatedBy,
			Value:       variable.Value.String,
		}); err != nil {
			return err
		}
	}

	signal.ActiveSubscriberCount--

	if err := ctx.Signals().Update(signal); err != nil {
		return err
	}

	event := EventEntity{
		Partition: execution.Partition,

		ElementInstanceId: execution.Id,

		CreatedAt:  ctx.Time(),
		CreatedBy:  signal.CreatedBy,
		SignalName: pgtype.Text{String: signal.Name, Valid: true},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	return nil
}

func triggerSignalStartEvent(ctx Context, task *TaskEntity, signalId int64) error {
	process, err := ctx.ProcessCache().GetOrCacheById(ctx, task.ProcessId.Int32)
	if err != nil {
		return err
	}

	startNode, ok := process.graph.nodeByElementId(task.ElementId.Int32)
	if !ok {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to find start node",
			Detail: fmt.Sprintf("start node with ID %d could not be found", task.ElementId.Int32),
		}
	}

	signal, err := ctx.Signals().Select(signalId)
	if err != nil {
		return err
	}

	var processInstance *ProcessInstanceEntity
	if task.ProcessInstanceId.Valid {
		selectedProcessInstance, err := ctx.ProcessInstances().Select(task.Partition, task.ProcessInstanceId.Int32)
		if err != nil {
			return err
		}

		if selectedProcessInstance.EndedAt.Valid {
			return nil
		}

		processInstance = selectedProcessInstance
	} else {
		processInstance = &ProcessInstanceEntity{
			Partition: task.Partition,

			ProcessId: process.Id,

			BpmnProcessId:  process.BpmnProcessId,
			CreatedAt:      ctx.Time(),
			CreatedBy:      signal.CreatedBy,
			StartedAt:      pgtype.Timestamp{Time: ctx.Time(), Valid: true},
			State:          engine.InstanceStarted,
			StateChangedBy: signal.CreatedBy,
			Version:        process.Version,
		}

		if err := ctx.ProcessInstances().Insert(processInstance); err != nil {
			return err
		}

		// set ID of inserted process instance for a possible retry
		task.ProcessInstanceId = pgtype.Int4{Int32: processInstance.Id}

		if err := enqueueProcessInstance(ctx, processInstance); err != nil {
			return fmt.Errorf("failed to enqueue process instance: %v", err)
		}
	}

	scope := process.graph.createProcessScope(processInstance)

	execution, err := process.graph.createExecutionAt(&scope, startNode.bpmnElement.Id)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create execution",
			Detail: err.Error(),
		}
	}

	ec := executionContext{
		engineOrWorkerId: signal.CreatedBy,
		process:          process,
		processInstance:  processInstance,
	}

	executions := []*ElementInstanceEntity{&scope, &execution}
	if err := ec.continueExecutions(ctx, executions); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", executions, err)
		}
	}

	signalVariables, err := ctx.SignalVariables().SelectBySignalId(signalId)
	if err != nil {
		return err
	}

	for _, variable := range signalVariables {
		if !variable.Value.Valid {
			continue
		}

		if err := ctx.Variables().Insert(&VariableEntity{
			Partition: processInstance.Partition,

			ProcessId:         processInstance.ProcessId,
			ProcessInstanceId: processInstance.Id,

			CreatedAt:   processInstance.CreatedAt,
			CreatedBy:   processInstance.CreatedBy,
			Encoding:    variable.Encoding.String,
			IsEncrypted: variable.IsEncrypted.Bool,
			Name:        variable.Name,
			UpdatedAt:   processInstance.CreatedAt,
			UpdatedBy:   processInstance.CreatedBy,
			Value:       variable.Value.String,
		}); err != nil {
			return err
		}
	}

	signal.ActiveSubscriberCount--

	if err := ctx.Signals().Update(signal); err != nil {
		return err
	}

	event := EventEntity{
		Partition: execution.Partition,

		ElementInstanceId: execution.Id,

		CreatedAt:  ctx.Time(),
		CreatedBy:  signal.CreatedBy,
		SignalName: pgtype.Text{String: signal.Name, Valid: true},
	}

	if err := ctx.Events().Insert(&event); err != nil {
		return err
	}

	return nil
}
