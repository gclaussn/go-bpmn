package internal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
)

func newGraph(processElements []*model.Element, elements []*ElementEntity) (graph, error) {
	ids := make(map[string]int32, len(elements))
	for i := 0; i < len(elements); i++ {
		ids[elements[i].BpmnElementId] = elements[i].Id
	}

	var e []string

	processElement := processElements[0]
	process := processElement.Model.(*model.Process)
	if !process.IsExecutable {
		e = append(e, "BPMN process is not executable")
	}

	nodes := make(map[string]node, len(processElements))
	for _, element := range processElements {
		if element.Id == "" {
			e = append(e, fmt.Sprintf("BPMN element of type %s has no ID", element.Type))
			continue
		}

		id, ok := ids[element.Id]
		if !ok {
			e = append(e, fmt.Sprintf("BPMN element %s has no entity", element.Id)) // indicates a bug
			continue
		}

		switch element.Type {
		case model.ElementInclusiveGateway:
			if len(element.Incoming) > 1 {
				e = append(e, fmt.Sprintf("BPMN element %s is not supported: joining inclusive gateway", element.Id))
			}
		}

		for _, sequenceFlow := range element.SequenceFlows {
			if sequenceFlow.Source == nil {
				e = append(e, fmt.Sprintf("BPMN sequence flow %s has no source element", sequenceFlow.Id))
			}
			if sequenceFlow.Target == nil {
				e = append(e, fmt.Sprintf("BPMN sequence flow %s has no target element", sequenceFlow.Id))
			}
		}

		nodes[element.Id] = node{id: id, element: element}
	}

	if len(e) != 0 {
		return graph{}, errors.New(strings.Join(e, "; "))
	}

	return graph{
		nodes:          nodes,
		processElement: processElement,
	}, nil
}

type graph struct {
	nodes          map[string]node // mapping between BPMN element IDs and nodes
	processElement *model.Element  // root element
}

func (g graph) createExecution(scope *ElementInstanceEntity) (ElementInstanceEntity, error) {
	scopeNode, ok := g.nodes[scope.BpmnElementId]
	if !ok {
		return ElementInstanceEntity{}, fmt.Errorf("BPMN process has no element %s", scope.BpmnElementId)
	}

	var noneStartEvents []*model.Element
	switch scopeNode.element.Type {
	case model.ElementProcess:
		noneStartEvents = scopeNode.element.ElementsByType(model.ElementNoneStartEvent)
	}

	if len(noneStartEvents) == 0 {
		return ElementInstanceEntity{}, fmt.Errorf("BPMN scope %s has no none start event element", scope.BpmnElementId)
	}

	node := g.nodes[noneStartEvents[0].Id]

	execution := ElementInstanceEntity{
		Partition: scope.Partition,

		ElementId:         node.id,
		ProcessId:         scope.ProcessId,
		ProcessInstanceId: scope.ProcessInstanceId,

		BpmnElementId:   noneStartEvents[0].Id,
		BpmnElementType: noneStartEvents[0].Type,
		State:           engine.InstanceCreated,

		parent: scope,
	}

	scope.ExecutionCount = 1

	return execution, nil
}

func (g graph) createExecutionAt(scope *ElementInstanceEntity, bpmnElementId string) (ElementInstanceEntity, error) {
	scopeNode, ok := g.nodes[scope.BpmnElementId]
	if !ok {
		return ElementInstanceEntity{}, fmt.Errorf("BPMN process has no element %s", scope.BpmnElementId)
	}

	node, ok := g.nodes[bpmnElementId]
	if !ok {
		return ElementInstanceEntity{}, fmt.Errorf("BPMN process has no element %s", bpmnElementId)
	}

	if node.element.Parent != scopeNode.element {
		return ElementInstanceEntity{}, fmt.Errorf("BPMN scope %s has no element %s", scope.BpmnElementId, bpmnElementId)
	}

	execution := ElementInstanceEntity{
		Partition: scope.Partition,

		ElementId:         node.id,
		ProcessId:         scope.ProcessId,
		ProcessInstanceId: scope.ProcessInstanceId,

		BpmnElementId:   node.element.Id,
		BpmnElementType: node.element.Type,
		State:           engine.InstanceCreated,

		parent: scope,
	}

	scope.ExecutionCount = scope.ExecutionCount + 1

	return execution, nil
}

func (g graph) createProcessScope(processInstance *ProcessInstanceEntity) ElementInstanceEntity {
	node := g.nodes[g.processElement.Id]

	return ElementInstanceEntity{
		Partition: processInstance.Partition,

		ElementId:         node.id,
		ProcessId:         processInstance.ProcessId,
		ProcessInstanceId: processInstance.Id,

		BpmnElementId:   g.processElement.Id,
		BpmnElementType: g.processElement.Type,
		CreatedAt:       processInstance.CreatedAt,
		CreatedBy:       processInstance.CreatedBy,
		StartedAt:       processInstance.StartedAt,
		State:           processInstance.State,
		StateChangedBy:  processInstance.StateChangedBy,
	}
}

func (g graph) ensureSequenceFlow(sourceId string, targetId string) error {
	node, ok := g.nodes[sourceId]
	if !ok {
		return fmt.Errorf("BPMN process has no element %s", sourceId)
	}
	if node.element.OutgoingById(targetId) == nil {
		return fmt.Errorf("BPMN element %s has no outgoing sequence flow to %s", sourceId, targetId)
	}
	return nil
}

func (g graph) joinParallelGateway(waiting []*ElementInstanceEntity) ([]*ElementInstanceEntity, error) {
	if len(waiting) < 2 {
		return nil, nil
	}

	node, ok := g.nodes[waiting[0].BpmnElementId]
	if !ok {
		return nil, fmt.Errorf("BPMN process has no element %s", waiting[0].BpmnElementId)
	}

	incoming := node.element.Incoming
	if node.element.Type != model.ElementParallelGateway || len(incoming) < 2 {
		return nil, fmt.Errorf("BPMN element %s is not a joining parallel gateway", node.element.Id)
	}

	for i := 0; i < len(waiting); i++ {
		if waiting[i].ElementId != node.id {
			return nil, fmt.Errorf("expected all waiting executions to have element ID %d", node.id)
		}
	}

	incomingIds := make(map[int32]int, len(incoming))
	for i := 0; i < len(incoming); i++ {
		sourceId := incoming[i].Source.Id
		sourceNode := g.nodes[sourceId]

		if count, ok := incomingIds[sourceNode.id]; ok {
			incomingIds[sourceNode.id] = count + 1
		} else {
			incomingIds[sourceNode.id] = 1
		}
	}

	var joined []*ElementInstanceEntity
	for i, execution := range waiting {
		count, ok := incomingIds[execution.PrevElementId.Int32]
		if !ok || count == 0 {
			continue
		}

		if i == 0 {
			execution.State = engine.InstanceStarted
		} else {
			execution.State = engine.InstanceEnded
		}

		incomingIds[execution.PrevElementId.Int32] = count - 1
		joined = append(joined, execution)

		if len(joined) == len(incoming) {
			return joined, nil
		}
	}

	return nil, nil
}

type node struct {
	id      int32 // ID of the related ElementEntity
	element *model.Element
}
