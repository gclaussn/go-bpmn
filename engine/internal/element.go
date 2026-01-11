package internal

import (
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

type ElementEntity struct {
	Id int32

	ProcessId int32

	BpmnElementId       string
	BpmnElementName     string
	BpmnElementType     model.ElementType
	IsMultiInstance     bool
	ParentBpmnElementId pgtype.Text
}

func (e ElementEntity) Element() engine.Element {
	return engine.Element{
		Id: e.Id,

		ProcessId: e.ProcessId,

		BpmnElementId:       e.BpmnElementId,
		BpmnElementName:     e.BpmnElementName,
		BpmnElementType:     e.BpmnElementType,
		IsMultiInstance:     e.IsMultiInstance,
		ParentBpmnElementId: e.ParentBpmnElementId.String,
	}
}

type ElementRepository interface {
	InsertBatch([]*ElementEntity) error
	SelectByProcessId(processId int32) ([]*ElementEntity, error)

	Query(engine.ElementCriteria, engine.QueryOptions) ([]engine.Element, error)
}
