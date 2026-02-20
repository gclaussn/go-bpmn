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

	BpmnElementId string
	CreatedAt     time.Time
	CreatedBy     string
	Name          string
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

	for _, variable := range cmd.Variables {
		if variable.Data == nil {
			continue
		}
		if err := encryption.EncryptData(variable.Data); err != nil {
			return engine.Signal{}, fmt.Errorf("failed to encrypt variable %s: %v", variable.Name, err)
		}
	}

	// find subscribers
	signalSubscriptions, err := ctx.SignalSubscriptions().DeleteByName(cmd.Name)
	if err != nil {
		return engine.Signal{}, err
	}

	subscriberCount := len(signalSubscriptions)

	eventDefinitions, err := ctx.EventDefinitions().SelectBySignalName(cmd.Name)
	if err != nil {
		return engine.Signal{}, err
	}
	for _, eventDefinition := range eventDefinitions {
		if eventDefinition.BpmnElementType == model.ElementSignalStartEvent {
			subscriberCount++
		}
	}

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
	signalVariableNames := make(map[string]bool, len(cmd.Variables))
	for _, variable := range cmd.Variables {
		if _, ok := signalVariableNames[variable.Name]; ok {
			continue // skip already processed variable
		}

		signalVariableNames[variable.Name] = true

		data := variable.Data
		if data == nil {
			signalVariables = append(signalVariables, &SignalVariableEntity{
				SignalId: signal.Id,

				Name: variable.Name,
			})
			continue
		}

		signalVariables = append(signalVariables, &SignalVariableEntity{
			SignalId: signal.Id,

			Encoding:    pgtype.Text{String: data.Encoding, Valid: true},
			IsEncrypted: pgtype.Bool{Bool: data.IsEncrypted, Valid: true},
			Name:        variable.Name,
			Value:       pgtype.Text{String: data.Value, Valid: true},
		})
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

			BpmnElementId: pgtype.Text{String: signalSubscription.BpmnElementId, Valid: true},
			CreatedAt:     signal.CreatedAt,
			CreatedBy:     signal.CreatedBy,
			DueAt:         signal.CreatedAt,
			State:         engine.WorkCreated,
			Type:          engine.TaskTriggerEvent,

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

			BpmnElementId: pgtype.Text{String: eventDefinition.BpmnElementId, Valid: true},
			CreatedAt:     signal.CreatedAt,
			CreatedBy:     signal.CreatedBy,
			DueAt:         signal.CreatedAt,
			State:         engine.WorkCreated,
			Type:          engine.TaskTriggerEvent,

			Instance: TriggerEventTask{SignalId: signal.Id},
		})
	}

	if err := ctx.Tasks().InsertBatch(triggerEventTasks); err != nil {
		return engine.Signal{}, err
	}

	return signal.Signal(), nil
}

func (ec *executionContext) triggerSignalBoundaryEvent(ctx Context, signalId int64, interrupting bool) error {
	execution := ec.executions[1]

	signal, err := ctx.Signals().Select(signalId)
	if err != nil {
		return err
	}

	ec.engineOrWorkerId = signal.CreatedBy

	newExecution, err := ec.startBoundaryEvent(ctx, interrupting)
	if err != nil {
		return err
	}

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
		}
	}

	signalVariables, err := ctx.SignalVariables().SelectBySignalId(signalId)
	if err != nil {
		return err
	}

	for _, variable := range signalVariables {
		if !variable.Value.Valid {
			if err := ctx.Variables().Delete(&VariableEntity{ // with fields, needed for deletion
				Partition:         execution.Partition,
				ProcessInstanceId: execution.ProcessInstanceId,
				Name:              variable.Name,
			}); err != nil {
				return err
			}

			continue
		}

		if err := ctx.Variables().Insert(&VariableEntity{
			Partition: execution.Partition,

			ProcessId:         execution.ProcessId,
			ProcessInstanceId: execution.ProcessInstanceId,

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

	if newExecution != nil {
		signalSubscription := SignalSubscriptionEntity{
			Partition: newExecution.Partition,

			ElementId:         newExecution.ElementId,
			ElementInstanceId: newExecution.Id,
			ProcessId:         newExecution.ProcessId,
			ProcessInstanceId: newExecution.ProcessInstanceId,

			BpmnElementId: newExecution.BpmnElementId,
			CreatedAt:     ctx.Time(),
			CreatedBy:     signal.CreatedBy,
			Name:          signal.Name,
		}

		if err := ctx.SignalSubscriptions().Insert(&signalSubscription); err != nil {
			return err
		}
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

func (ec *executionContext) triggerSignalCatchEvent(ctx Context, signalId int64) error {
	execution := ec.executions[1]

	signal, err := ctx.Signals().Select(signalId)
	if err != nil {
		return err
	}

	ec.engineOrWorkerId = signal.CreatedBy

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
		}
	}

	signalVariables, err := ctx.SignalVariables().SelectBySignalId(signalId)
	if err != nil {
		return err
	}

	for _, variable := range signalVariables {
		if !variable.Value.Valid {
			if err := ctx.Variables().Delete(&VariableEntity{ // with fields, needed for deletion
				Partition:         execution.Partition,
				ProcessInstanceId: execution.ProcessInstanceId,
				Name:              variable.Name,
			}); err != nil {
				return err
			}

			continue
		}

		if err := ctx.Variables().Insert(&VariableEntity{
			Partition: execution.Partition,

			ProcessId:         execution.ProcessId,
			ProcessInstanceId: execution.ProcessInstanceId,

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

func (ec *executionContext) triggerSignalStartEvent(ctx Context, task *TaskEntity, startElement *model.Element, signalId int64) error {
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

			ProcessId: ec.process.Id,

			BpmnProcessId: ec.process.BpmnProcessId,
			CreatedAt:     ctx.Time(),
			CreatedBy:     signal.CreatedBy,
			StartedAt:     pgtype.Timestamp{Time: ctx.Time(), Valid: true},
			State:         engine.InstanceStarted,
			Version:       ec.process.Version,
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

	scope := ec.process.graph.createProcessScope(processInstance)

	execution, err := ec.process.graph.createExecutionAt(&scope, startElement.Id)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create execution",
			Detail: err.Error(),
		}
	}

	ec.processInstance = processInstance
	ec.addExecution(&scope)
	ec.addExecution(&execution)

	if err := ec.continueExecutions(ctx); err != nil {
		if _, ok := err.(engine.Error); ok {
			return err
		} else {
			return fmt.Errorf("failed to continue executions %+v: %v", ec.executions, err)
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
