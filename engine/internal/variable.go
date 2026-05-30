package internal

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
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
	SelectByElementInstance(elementInstance *ElementInstanceEntity, names []string) ([]*VariableEntity, error)
	SelectByProcessInstance(engine.GetProcessVariablesCmd) ([]*VariableEntity, error)
	Upsert(*VariableEntity) error

	Query(engine.VariableCriteria, engine.QueryOptions) ([]engine.Variable, error)
}

func GetElementVariables(ctx Context, cmd engine.GetElementVariablesCmd) ([]engine.ElementVariable, error) {
	processInstance, err := ctx.ProcessInstances().SelectByElementInstance(time.Time(cmd.Partition), cmd.ElementInstanceId)
	if err == pgx.ErrNoRows {
		return nil, engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to get element variables",
			Detail: fmt.Sprintf("element instance %s/%d could not be found", cmd.Partition, cmd.ElementInstanceId),
		}
	}

	variables := make([]engine.ElementVariable, 0, 1)

	elementInstanceId := cmd.ElementInstanceId
	for {
		elementInstance, err := ctx.ElementInstances().Select(processInstance.Partition, elementInstanceId)
		if err != nil {
			return nil, err
		}

		entities, err := ctx.Variables().SelectByElementInstance(elementInstance, cmd.Names)
		if err != nil {
			return nil, err
		}

		for _, entity := range entities {
			data := engine.Data{
				Encoding:    entity.Encoding,
				IsEncrypted: entity.IsEncrypted,
				Value:       entity.Value,
			}

			if err := ctx.Options().Encryption.DecryptData(&data); err != nil {
				return nil, fmt.Errorf("failed to decrypt element variable: %v", err)
			}

			variables = append(variables, engine.ElementVariable{
				BpmnElementId: elementInstance.BpmnElementId,
				Name:          entity.Name,
				Data:          &data,
			})
		}

		if !elementInstance.ParentId.Valid || elementInstance.BpmnElementType == model.ElementProcess {
			break
		}
		if cmd.ExcludeParentVariables {
			break
		}

		elementInstanceId = elementInstance.ParentId.Int32
	}

	slices.SortFunc(variables, func(a engine.ElementVariable, b engine.ElementVariable) int {
		if a.Name != b.Name {
			return strings.Compare(a.Name, b.Name)
		} else {
			return strings.Compare(a.BpmnElementId, b.BpmnElementId)
		}
	})

	return variables, nil
}

func GetProcessVariables(ctx Context, cmd engine.GetProcessVariablesCmd) ([]engine.ProcessVariable, error) {
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

	variables := make([]engine.ProcessVariable, len(entities))
	for i, entity := range entities {
		data := engine.Data{
			Encoding:    entity.Encoding,
			IsEncrypted: entity.IsEncrypted,
			Value:       entity.Value,
		}

		if err := ctx.Options().Encryption.DecryptData(&data); err != nil {
			return nil, fmt.Errorf("failed to decrypt process variable: %v", err)
		}

		variables[i] = engine.ProcessVariable{
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

	// create mapping between BPMN element ID and element instance
	elementInstances := make(map[string]*ElementInstanceEntity)
	elementInstances[""] = elementInstance

	var counter int
	for _, variable := range cmd.Variables {
		if variable.BpmnElementId == "" {
			continue
		}

		if _, ok := elementInstances[variable.BpmnElementId]; ok {
			continue
		}

		elementInstances[variable.BpmnElementId] = nil
		counter++
	}

	for {
		if _, ok := elementInstances[elementInstance.BpmnElementId]; ok {
			counter--
		}

		elementInstances[elementInstance.BpmnElementId] = elementInstance

		if elementInstance.BpmnElementType == model.ElementProcess {
			break
		}

		elementInstance, err = ctx.ElementInstances().Select(elementInstance.Partition, elementInstance.ParentId.Int32)
		if err != nil {
			return err
		}
	}

	if counter != 0 {
		// collect scope IDs
		scopeIds, invalidScopeIds :=
			make([]string, 0, len(elementInstances)-counter),
			make([]string, 0, counter)

		for bpmnElementId, elementInstance := range elementInstances {
			if bpmnElementId == "" {
				continue
			}

			if elementInstance != nil {
				scopeIds = append(scopeIds, bpmnElementId)
			} else {
				invalidScopeIds = append(invalidScopeIds, bpmnElementId)
			}
		}

		return engine.Error{
			Type:  engine.ErrorValidation,
			Title: "failed to set element variables",
			Detail: fmt.Sprintf(
				"element instance %s/%d has no such scopes [%s], but [%s]",
				cmd.Partition,
				cmd.ElementInstanceId,
				strings.Join(invalidScopeIds, ", "),
				strings.Join(scopeIds, ", "),
			),
		}
	}

	encryption := ctx.Options().Encryption

	entities := make([]*VariableEntity, 0, len(cmd.Variables))
	for _, variable := range cmd.Variables {
		elementInstance := elementInstances[variable.BpmnElementId]

		// deduplication
		if containsElementVariable(entities, elementInstance.ElementId, variable.Name) {
			continue
		}

		data := variable.Data
		if data == nil {
			entities = append(entities, &VariableEntity{ // with fields, needed for deletion
				Partition:         processInstance.Partition,
				ElementInstanceId: pgtype.Int4{Int32: elementInstance.Id, Valid: true},
				Name:              variable.Name,
			})
			continue
		}

		if err := encryption.EncryptData(data); err != nil {
			return fmt.Errorf("failed to encrypt element variable %s: %v", variable.Name, err)
		}

		entities = append(entities, &VariableEntity{
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
		})
	}

	for _, entity := range entities {
		if entity.ElementId.Valid {
			if err := ctx.Variables().Upsert(entity); err != nil {
				return err
			}
		} else {
			if err := ctx.Variables().Delete(entity); err != nil {
				return err
			}
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

	encryption := ctx.Options().Encryption

	entities := make([]*VariableEntity, 0, len(cmd.Variables))
	for _, variable := range cmd.Variables {
		// deduplication
		if containsProcessVariable(entities, variable.Name) {
			continue
		}

		data := variable.Data
		if data == nil {
			entities = append(entities, &VariableEntity{ // with fields, needed for deletion
				Partition:         processInstance.Partition,
				ProcessInstanceId: processInstance.Id,
				Name:              variable.Name,
			})
			continue
		}

		if err := encryption.EncryptData(data); err != nil {
			return fmt.Errorf("failed to encrypt element variable %s: %v", variable.Name, err)
		}

		entities = append(entities, &VariableEntity{
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
		})
	}

	for _, entity := range entities {
		if entity.ProcessId != 0 {
			if err := ctx.Variables().Upsert(entity); err != nil {
				return err
			}
		} else {
			if err := ctx.Variables().Delete(entity); err != nil {
				return err
			}
		}
	}

	return nil
}

func containsElementVariable(entities []*VariableEntity, elementId int32, name string) bool {
	for _, entity := range entities {
		if entity.ElementId.Int32 == elementId && entity.Name == name {
			return true
		}
	}
	return false
}

func containsProcessVariable(entities []*VariableEntity, name string) bool {
	for _, entity := range entities {
		if entity.Name == name {
			return true
		}
	}
	return false
}
