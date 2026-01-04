package internal

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type VariableEntity struct {
	Partition time.Time
	Id        int32

	ElementId         pgtype.Int4
	ElementInstanceId pgtype.Int4
	ProcessId         int32
	ProcessInstanceId int32

	CreatedAt   time.Time
	CreatedBy   string
	Encoding    string
	IsEncrypted bool
	Name        string
	UpdatedAt   time.Time
	UpdatedBy   string
	Value       string
}

func (e VariableEntity) Variable() engine.Variable {
	return engine.Variable{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		ElementId:         e.ElementId.Int32,
		ElementInstanceId: e.ElementInstanceId.Int32,
		ProcessId:         e.ProcessId,
		ProcessInstanceId: e.ProcessInstanceId,

		CreatedAt:   e.CreatedAt,
		CreatedBy:   e.CreatedBy,
		Encoding:    e.Encoding,
		IsEncrypted: e.IsEncrypted,
		Name:        e.Name,
		UpdatedAt:   e.UpdatedAt,
		UpdatedBy:   e.UpdatedBy,
	}
}

type VariableRepository interface {
	Delete(*VariableEntity) error
	Insert(*VariableEntity) error
	SelectByElementInstance(engine.GetElementVariablesCmd) ([]*VariableEntity, error)
	SelectByProcessInstance(engine.GetProcessVariablesCmd) ([]*VariableEntity, error)
	Upsert(*VariableEntity) error

	Query(engine.VariableCriteria, engine.QueryOptions) ([]engine.Variable, error)
}

func GetElementVariables(ctx Context, cmd engine.GetElementVariablesCmd) ([]engine.VariableData, error) {
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

	slices.SortFunc(entities, func(a *VariableEntity, b *VariableEntity) int {
		return strings.Compare(a.Name, b.Name)
	})

	variables := make([]engine.VariableData, len(entities))
	for i, entity := range entities {
		data := engine.Data{
			Encoding:    entity.Encoding,
			IsEncrypted: entity.IsEncrypted,
			Value:       entity.Value,
		}

		if err := ctx.Options().Encryption.DecryptData(&data); err != nil {
			return nil, fmt.Errorf("failed to decrypt element variable: %v", err)
		}

		variables[i] = engine.VariableData{
			Name: entity.Name,
			Data: &data,
		}
	}

	return variables, nil
}

func GetProcessVariables(ctx Context, cmd engine.GetProcessVariablesCmd) ([]engine.VariableData, error) {
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

	slices.SortFunc(entities, func(a *VariableEntity, b *VariableEntity) int {
		return strings.Compare(a.Name, b.Name)
	})

	variables := make([]engine.VariableData, len(entities))
	for i, entity := range entities {
		data := engine.Data{
			Encoding:    entity.Encoding,
			IsEncrypted: entity.IsEncrypted,
			Value:       entity.Value,
		}

		if err := ctx.Options().Encryption.DecryptData(&data); err != nil {
			return nil, fmt.Errorf("failed to decrypt process variable: %v", err)
		}

		variables[i] = engine.VariableData{
			Name: entity.Name,
			Data: &data,
		}
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

	variableNames := make(map[string]bool, len(cmd.Variables))
	for _, variable := range cmd.Variables {
		if _, ok := variableNames[variable.Name]; ok {
			continue // skip already processed variable
		}

		variableNames[variable.Name] = true

		data := variable.Data
		if data == nil {
			variable := VariableEntity{ // with fields, needed for deletion
				Partition:         processInstance.Partition,
				ElementInstanceId: pgtype.Int4{Int32: elementInstance.Id, Valid: true},
				Name:              variable.Name,
			}

			if err := ctx.Variables().Delete(&variable); err != nil {
				return err
			}

			continue
		}

		if err := ctx.Options().Encryption.EncryptData(data); err != nil {
			return fmt.Errorf("failed to encrypt element variable %s: %v", variable.Name, err)
		}

		variable := VariableEntity{
			Partition: processInstance.Partition,

			ElementId:         pgtype.Int4{Int32: elementInstance.ElementId, Valid: true},
			ElementInstanceId: pgtype.Int4{Int32: elementInstance.Id, Valid: true},
			ProcessId:         elementInstance.ProcessId,
			ProcessInstanceId: elementInstance.ProcessInstanceId,

			CreatedAt:   ctx.Time(),
			CreatedBy:   cmd.WorkerId,
			Encoding:    data.Encoding,
			IsEncrypted: data.IsEncrypted,
			Name:        variable.Name,
			UpdatedAt:   ctx.Time(),
			UpdatedBy:   cmd.WorkerId,
			Value:       data.Value,
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

	variableNames := make(map[string]bool, len(cmd.Variables))
	for _, variable := range cmd.Variables {
		if _, ok := variableNames[variable.Name]; ok {
			continue // skip already processed variable
		}

		variableNames[variable.Name] = true

		data := variable.Data
		if data == nil {
			variable := VariableEntity{ // with fields, needed for deletion
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
				Name:              variable.Name,
			}

			if err := ctx.Variables().Delete(&variable); err != nil {
				return err
			}

			continue
		}

		if err := ctx.Options().Encryption.EncryptData(data); err != nil {
			return fmt.Errorf("failed to encrypt process variable %s: %v", variable.Name, err)
		}

		variable := VariableEntity{
			Partition: processInstance.Partition,

			ProcessId:         processInstance.ProcessId,
			ProcessInstanceId: processInstance.Id,

			CreatedAt:   ctx.Time(),
			CreatedBy:   cmd.WorkerId,
			Encoding:    data.Encoding,
			IsEncrypted: data.IsEncrypted,
			Name:        variable.Name,
			UpdatedAt:   ctx.Time(),
			UpdatedBy:   cmd.WorkerId,
			Value:       data.Value,
		}

		if err := ctx.Variables().Upsert(&variable); err != nil {
			return err
		}
	}

	return nil
}
