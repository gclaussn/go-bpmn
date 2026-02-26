package internal

import (
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
)

type ElementInstanceEntity struct {
	Partition time.Time
	Id        int32

	ParentId      pgtype.Int4
	PrevElementId pgtype.Int4 // internal field
	PrevId        pgtype.Int4 // internal field

	ElementId         int32
	ProcessId         int32
	ProcessInstanceId int32

	BpmnElementId   string
	BpmnElementType model.ElementType
	Context         pgtype.Text // internal field, used for error code and escalation code
	CreatedAt       time.Time
	CreatedBy       string
	EndedAt         pgtype.Timestamp
	ExecutionCount  int // internal field
	IsMultiInstance bool
	StartedAt       pgtype.Timestamp
	State           engine.InstanceState

	parent *ElementInstanceEntity
	prev   *ElementInstanceEntity
}

func (e ElementInstanceEntity) ElementInstance() engine.ElementInstance {
	return engine.ElementInstance{
		Partition: engine.Partition(e.Partition),
		Id:        e.Id,

		ParentId: e.ParentId.Int32,

		ElementId:         e.ElementId,
		ProcessId:         e.ProcessId,
		ProcessInstanceId: e.ProcessInstanceId,

		BpmnElementId:   e.BpmnElementId,
		BpmnElementType: e.BpmnElementType,
		CreatedAt:       e.CreatedAt,
		CreatedBy:       e.CreatedBy,
		EndedAt:         timeOrNil(e.EndedAt),
		IsMultiInstance: e.IsMultiInstance,
		StartedAt:       timeOrNil(e.StartedAt),
		State:           e.State,
	}
}

type ElementInstanceRepository interface {
	Insert(*ElementInstanceEntity) error
	Select(partition time.Time, id int32) (*ElementInstanceEntity, error)

	// SelectActiveChildren selects children of the given parent element instance
	// that are not in state COMPLETED, TERMINATED and CANCELED.
	SelectActiveChildren(parent *ElementInstanceEntity) ([]*ElementInstanceEntity, error)

	// SelectByProcessInstanceAndState selects element instances related to a process instance.
	// Only element instances with the same state as the process instance are selected.
	SelectByProcessInstanceAndState(*ProcessInstanceEntity) ([]*ElementInstanceEntity, error)

	// SelectBoundaryEvents selects element instances, which are attached to the given element instance.
	// Only element instances with state CREATED are selected.
	SelectBoundaryEvents(*ElementInstanceEntity) ([]*ElementInstanceEntity, error)

	// SelectParallelGateways selects element instances, which have the same
	//  - parent element instance
	//  - element
	//  - state
	SelectParallelGateways(*ElementInstanceEntity) ([]*ElementInstanceEntity, error)

	Update(*ElementInstanceEntity) error

	Query(engine.ElementInstanceCriteria, engine.QueryOptions) ([]engine.ElementInstance, error)
}
