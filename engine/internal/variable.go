package internal

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type VariableEntity struct {
	Partition time.Time
	Id        int32

	ElementId         pgtype.Int4 // NULL in case of an event
	ElementInstanceId pgtype.Int4 // NULL in case of an event
	EventId           pgtype.Int4
	ProcessId         pgtype.Int4 // NULL in case of an event
	ProcessInstanceId pgtype.Int4 // NULL in case of an event

	CreatedAt   time.Time
	CreatedBy   string
	Encoding    pgtype.Text // can be NULL in case of a signal, when a process instance variable should be deleted
	IsEncrypted pgtype.Bool // can be NULL in case of a signal, when a process instance variable should be deleted
	Name        string
	UpdatedAt   time.Time
	UpdatedBy   string
	Value       pgtype.Text // can be NULL in case of a signal, when a process instance variable should be deleted
}

func (e VariableEntity) Variable() engine.Variable {
	return engine.Variable{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		ElementId:         e.ElementId.Int32,
		ElementInstanceId: e.ElementInstanceId.Int32,
		EventId:           e.EventId.Int32,
		ProcessId:         e.ProcessId.Int32,
		ProcessInstanceId: e.ProcessInstanceId.Int32,

		CreatedAt:   e.CreatedAt,
		CreatedBy:   e.CreatedBy,
		Encoding:    e.Encoding.String,
		IsEncrypted: e.IsEncrypted.Bool,
		Name:        e.Name,
		UpdatedAt:   e.UpdatedAt,
		UpdatedBy:   e.UpdatedBy,
	}
}

type VariableRepository interface {
	Delete(*VariableEntity) error
	Insert(*VariableEntity) error
	SelectByElementInstance(engine.GetElementVariablesCmd) ([]*VariableEntity, error)
	SelectByEvent(partition time.Time, eventId int32) ([]*VariableEntity, error)
	SelectByProcessInstance(engine.GetProcessVariablesCmd) ([]*VariableEntity, error)
	Upsert(*VariableEntity) error

	Query(engine.VariableCriteria, engine.QueryOptions) ([]any, error)
}

func GetElementVariables(ctx Context, cmd engine.GetElementVariablesCmd) (map[string]engine.Data, error) {
	_, err := ctx.ProcessInstances().SelectByElementInstance(time.Time(cmd.Partition), cmd.ElementInstanceId)
	if err == pgx.ErrNoRows {
		return nil, engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to get element variables",
			Detail: fmt.Sprintf("element instance %s/%d could not be found", cmd.Partition, cmd.ElementInstanceId),
		}
	}

	entities, err := ctx.Variables().SelectByElementInstance(cmd)
	if err != nil {
		return nil, err
	}

	variables := make(map[string]engine.Data, len(entities))
	for _, entity := range entities {
		data := engine.Data{
			Encoding:    entity.Encoding.String,
			IsEncrypted: entity.IsEncrypted.Bool,
			Value:       entity.Value.String,
		}

		if err := ctx.Options().Encryption.DecryptData(&data); err != nil {
			return nil, fmt.Errorf("failed to decrypt element variable: %v", err)
		}

		variables[entity.Name] = data
	}

	return variables, nil
}

func GetProcessVariables(ctx Context, cmd engine.GetProcessVariablesCmd) (map[string]engine.Data, error) {
	_, err := ctx.ProcessInstances().Select(time.Time(cmd.Partition), cmd.ProcessInstanceId)
	if err == pgx.ErrNoRows {
		return nil, engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to get process variables",
			Detail: fmt.Sprintf("process instance %s/%d could not be found", cmd.Partition, cmd.ProcessInstanceId),
		}
	}

	entities, err := ctx.Variables().SelectByProcessInstance(cmd)
	if err != nil {
		return nil, err
	}

	variables := make(map[string]engine.Data, len(entities))
	for _, entity := range entities {
		data := engine.Data{
			Encoding:    entity.Encoding.String,
			IsEncrypted: entity.IsEncrypted.Bool,
			Value:       entity.Value.String,
		}

		if err := ctx.Options().Encryption.DecryptData(&data); err != nil {
			return nil, fmt.Errorf("failed to decrypt process variable: %v", err)
		}

		variables[entity.Name] = data
	}

	return variables, nil
}

func SetElementVariables(ctx Context, cmd engine.SetElementVariablesCmd) error {
	processInstance, err := ctx.ProcessInstances().SelectByElementInstance(time.Time(cmd.Partition), cmd.ElementInstanceId)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to set element variables",
			Detail: fmt.Sprintf("element instance %s/%d could not be found", cmd.Partition, cmd.ElementInstanceId),
		}
	}
	if err != nil {
		return err
	}

	if processInstance.EndedAt.Valid {
		return engine.Error{
			Type:   engine.ErrorConflict,
			Title:  "failed to set element variables",
			Detail: fmt.Sprintf("process instance %s/%d is ended", cmd.Partition, processInstance.Id),
		}
	}

	elementInstance, err := ctx.ElementInstances().Select(processInstance.Partition, cmd.ElementInstanceId)
	if err != nil {
		return err
	}

	if elementInstance.EndedAt.Valid {
		return engine.Error{
			Type:   engine.ErrorConflict,
			Title:  "failed to set element variables",
			Detail: fmt.Sprintf("element instance %s/%d is ended", cmd.Partition, cmd.ElementInstanceId),
		}
	}

	for variableName, data := range cmd.Variables {
		if data == nil {
			variable := VariableEntity{ // with fields, needed for deletion
				Partition:         processInstance.Partition,
				ElementInstanceId: pgtype.Int4{Int32: elementInstance.Id, Valid: true},
				Name:              variableName,
			}

			if err := ctx.Variables().Delete(&variable); err != nil {
				return err
			}

			continue
		}

		if err := ctx.Options().Encryption.EncryptData(data); err != nil {
			return fmt.Errorf("failed to encrypt element variable: %v", err)
		}

		variable := VariableEntity{
			Partition: processInstance.Partition,

			ElementId:         pgtype.Int4{Int32: elementInstance.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: elementInstance.Id, Valid: true},
			ProcessId:         pgtype.Int4{Int32: elementInstance.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: elementInstance.ProcessInstanceId, Valid: true},

			CreatedAt:   ctx.Time(),
			CreatedBy:   cmd.WorkerId,
			Encoding:    pgtype.Text{String: data.Encoding, Valid: true},
			IsEncrypted: pgtype.Bool{Bool: data.IsEncrypted, Valid: true},
			Name:        variableName,
			UpdatedAt:   ctx.Time(),
			UpdatedBy:   cmd.WorkerId,
			Value:       pgtype.Text{String: data.Value, Valid: true},
		}

		if err := ctx.Variables().Upsert(&variable); err != nil {
			return err
		}
	}

	return nil
}

func SetProcessVariables(ctx Context, cmd engine.SetProcessVariablesCmd) error {
	processInstance, err := ctx.ProcessInstances().Select(time.Time(cmd.Partition), cmd.ProcessInstanceId)
	if err == pgx.ErrNoRows {
		return engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to set process variables",
			Detail: fmt.Sprintf("process instance %s/%d could not be found", cmd.Partition, cmd.ProcessInstanceId),
		}
	}
	if err != nil {
		return err
	}

	if processInstance.EndedAt.Valid {
		return engine.Error{
			Type:   engine.ErrorConflict,
			Title:  "failed to set process variables",
			Detail: fmt.Sprintf("process instance %s/%d is ended", cmd.Partition, cmd.ProcessInstanceId),
		}
	}

	for variableName, data := range cmd.Variables {
		if data == nil {
			variable := VariableEntity{ // with fields, needed for deletion
				Partition:         processInstance.Partition,
				ProcessInstanceId: pgtype.Int4{Int32: processInstance.Id, Valid: true},
				Name:              variableName,
			}

			if err := ctx.Variables().Delete(&variable); err != nil {
				return err
			}

			continue
		}

		if err := ctx.Options().Encryption.EncryptData(data); err != nil {
			return fmt.Errorf("failed to encrypt process variable: %v", err)
		}

		variable := VariableEntity{
			Partition: processInstance.Partition,

			ProcessId:         pgtype.Int4{Int32: processInstance.ProcessId, Valid: true},
			ProcessInstanceId: pgtype.Int4{Int32: processInstance.Id, Valid: true},

			CreatedAt:   ctx.Time(),
			CreatedBy:   cmd.WorkerId,
			Encoding:    pgtype.Text{String: data.Encoding, Valid: true},
			IsEncrypted: pgtype.Bool{Bool: data.IsEncrypted, Valid: true},
			Name:        variableName,
			UpdatedAt:   ctx.Time(),
			UpdatedBy:   cmd.WorkerId,
			Value:       pgtype.Text{String: data.Value, Valid: true},
		}

		if err := ctx.Variables().Upsert(&variable); err != nil {
			return err
		}
	}

	return nil
}
