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

	ParentId pgtype.Int4

	ElementId             int32
	PrevElementId         pgtype.Int4 // internal field
	PrevElementInstanceId pgtype.Int4 // internal field
	ProcessId             int32
	ProcessInstanceId     int32

	BpmnElementId   string
	BpmnElementType model.ElementType
	CreatedAt       time.Time
	CreatedBy       string
	EndedAt         pgtype.Timestamp
	ExecutionCount  int // internal field
	IsMultiInstance bool
	StartedAt       pgtype.Timestamp
	State           engine.InstanceState
	StateChangedBy  string

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
		StateChangedBy:  e.StateChangedBy,
	}
}

type ElementInstanceRepository interface {
	Insert(*ElementInstanceEntity) error
	Select(partition time.Time, id int32) (*ElementInstanceEntity, error)

	// SelectByProcessInstanceAndState selects all element instances related to a process instance and are in the same state.
	SelectByProcessInstanceAndState(*ProcessInstanceEntity) ([]*ElementInstanceEntity, error)
	// SelectParallelGateways selects all element instances that have a specific element ID, parent element instance ID and state.
	SelectParallelGateways(*ElementInstanceEntity) ([]*ElementInstanceEntity, error)

	Update(*ElementInstanceEntity) error

	Query(engine.ElementInstanceCriteria, engine.QueryOptions) ([]engine.ElementInstance, error)
}
