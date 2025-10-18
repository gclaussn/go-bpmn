package internal

import (
	"errors"
	"fmt"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
)

// validateProcess validates if the process and its elements can be executed.
// If the process is invalid, causes are returned.
func validateProcess(bpmnElements []*model.Element) ([]engine.ErrorCause, error) {
	if len(bpmnElements) == 0 {
		return nil, engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to validate process",
			Detail: "expected BPMN elements not to be empty",
		}
	}

	process, ok := bpmnElements[0].Model.(model.Process)
	if !ok {
		return nil, engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to validate process",
			Detail: "expected process",
		}
	}

	var causes []engine.ErrorCause

	if !process.IsExecutable {
		causes = append(causes, engine.ErrorCause{
			Pointer: elementPointer(bpmnElements[0]),
			Type:    "process",
			Detail:  fmt.Sprintf("BPMN process %s is not executable", bpmnElements[0].Id),
		})
	}

	for _, bpmnElement := range bpmnElements {
		if bpmnElement.Id == "" {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(bpmnElement),
				Type:    "element",
				Detail:  fmt.Sprintf("BPMN element of type %s has no ID", bpmnElement.Type),
			})
		}

		switch bpmnElement.Type {
		case model.ElementExclusiveGateway:
			exclusivGateway := bpmnElement.Model.(model.ExclusiveGateway)
			if exclusivGateway.Default != "" {
				sequenceFlow := bpmnElement.OutgoingById(exclusivGateway.Default)
				if sequenceFlow == nil || sequenceFlow.Source != bpmnElement {
					causes = append(causes, engine.ErrorCause{
						Pointer: elementPointer(bpmnElement),
						Type:    "element",
						Detail:  fmt.Sprintf("exclusive gateway %s has no default sequence flow %s", bpmnElement.Id, exclusivGateway.Default),
					})
				}
			}
		case model.ElementInclusiveGateway:
			inclusiveGateway := bpmnElement.Model.(model.InclusiveGateway)
			if inclusiveGateway.Default != "" {
				sequenceFlow := bpmnElement.OutgoingById(inclusiveGateway.Default)
				if sequenceFlow == nil || sequenceFlow.Source != bpmnElement {
					causes = append(causes, engine.ErrorCause{
						Pointer: elementPointer(bpmnElement),
						Type:    "element",
						Detail:  fmt.Sprintf("inclusive gateway %s has no default sequence flow %s", bpmnElement.Id, inclusiveGateway.Default),
					})
				}
			}

			if len(bpmnElement.Incoming) > 1 {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "element",
					Detail:  fmt.Sprintf("BPMN element %s is not supported: joining inclusive gateway", bpmnElement.Id),
				})
			}
		case model.ElementErrorBoundaryEvent:
			boundaryEvent := bpmnElement.Model.(model.BoundaryEvent)
			if boundaryEvent.AttachedTo == nil {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "element",
					Detail:  fmt.Sprintf("boundary event %s is not attached", bpmnElement.Id),
				})
			}
		}

		for _, sequenceFlow := range bpmnElement.Incoming {
			if sequenceFlow.Source == nil {
				causes = append(causes, engine.ErrorCause{
					Pointer: fmt.Sprintf("%s/%s", elementPointer(bpmnElement.Parent), sequenceFlow.Id),
					Type:    "sequence_flow",
					Detail:  fmt.Sprintf("BPMN sequence flow %s has no source element", sequenceFlow.Id),
				})
			}
		}
		for _, sequenceFlow := range bpmnElement.Outgoing {
			if sequenceFlow.Target == nil {
				causes = append(causes, engine.ErrorCause{
					Pointer: fmt.Sprintf("%s/%s", elementPointer(bpmnElement.Parent), sequenceFlow.Id),
					Type:    "sequence_flow",
					Detail:  fmt.Sprintf("BPMN sequence flow %s has no target element", sequenceFlow.Id),
				})
			}
		}
	}

	return causes, nil
}

func newGraph(bpmnModel *model.Model, bpmnElements []*model.Element, elements []*ElementEntity) (graph, error) {
	if len(bpmnElements) == 0 {
		return graph{}, errors.New("expected BPMN elements not to be empty")
	}
	if len(bpmnElements) != len(elements) {
		return graph{}, fmt.Errorf("expected number of BPMN elements and entities to be equal: %d != %d", len(bpmnElements), len(elements))
	}

	ids := make(map[string]int32, len(elements))
	for i := range elements {
		ids[elements[i].BpmnElementId] = elements[i].Id
	}

	processElement := bpmnElements[0]
	if processElement.Type != model.ElementProcess {
		return graph{}, errors.New("expected process")
	}

	nodes := make(map[string]node, len(bpmnElements))
	for _, bpmnElement := range bpmnElements {
		id, ok := ids[bpmnElement.Id]
		if !ok {
			return graph{}, fmt.Errorf("BPMN element %s has no entity", bpmnElement.Id)
		}
		nodes[bpmnElement.Id] = node{id: id, bpmnElement: bpmnElement}
	}

	return graph{
		model:          bpmnModel,
		nodes:          nodes,
		processElement: processElement,
	}, nil
}

type graph struct {
	model          *model.Model
	nodes          map[string]node // mapping between BPMN element IDs and nodes
	processElement *model.Element  // root element
}

func (g graph) createExecution(scope *ElementInstanceEntity) (ElementInstanceEntity, error) {
	scopeNode, ok := g.nodes[scope.BpmnElementId]
	if !ok {
		return ElementInstanceEntity{}, fmt.Errorf("BPMN process %s has no element %s", g.processElement.Id, scope.BpmnElementId)
	}

	var noneStartEvents []*model.Element
	switch scopeNode.bpmnElement.Type {
	case model.ElementProcess:
		noneStartEvents = scopeNode.bpmnElement.ChildrenByType(model.ElementNoneStartEvent)
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

		parent: scope,
	}

	scope.ExecutionCount = 1

	return execution, nil
}

func (g graph) continueExecution(executions []*ElementInstanceEntity, execution *ElementInstanceEntity) ([]*ElementInstanceEntity, error) {
	node, ok := g.nodes[execution.BpmnElementId]
	if !ok {
		return executions, fmt.Errorf("BPMN process %s has no element %s", g.processElement.Id, execution.BpmnElementId)
	}

	if execution.State == engine.InstanceCompleted {
		return executions, nil // skip already completed execution
	}

	scope := execution.parent
	if scope.State == engine.InstanceQueued {
		execution.State = scope.State // end branch
		return executions, nil
	} else if execution.State == engine.InstanceStarted {
		execution.State = engine.InstanceCompleted // continue branch
	} else if scope.State == engine.InstanceSuspended {
		execution.State = engine.InstanceSuspended // end branch
		return executions, nil
	} else {
		// end branch, if BPMN element requires a job to be completed or a task to be executed
		switch execution.BpmnElementType {
		case
			model.ElementBusinessRuleTask,
			model.ElementErrorBoundaryEvent,
			model.ElementMessageCatchEvent,
			model.ElementScriptTask,
			model.ElementSendTask,
			model.ElementServiceTask,
			model.ElementSignalCatchEvent,
			model.ElementTimerCatchEvent:
			execution.State = engine.InstanceCreated
		case
			model.ElementExclusiveGateway,
			model.ElementInclusiveGateway:
			if len(node.bpmnElement.Outgoing) > 1 {
				execution.State = engine.InstanceCreated
			} else {
				execution.State = engine.InstanceCompleted
			}
		case model.ElementParallelGateway:
			if len(node.bpmnElement.Incoming) > 1 {
				execution.State = engine.InstanceCreated
			} else {
				execution.State = engine.InstanceCompleted
			}
		default:
			// continue branch, if element has no behavior (pass through element)
			execution.State = engine.InstanceCompleted
		}
	}

	if execution.State == engine.InstanceCompleted {
		for _, sequenceFlow := range node.bpmnElement.Outgoing {
			target := sequenceFlow.Target
			targetNode := g.nodes[target.Id]

			var prev *ElementInstanceEntity

			// determine if a previous execution should be set
			// if set, the next execution will be part of the "element_instance_prev_id_idx" index
			switch target.Type {
			case
				// required for a possible event based gateway
				model.ElementMessageCatchEvent,
				model.ElementSignalCatchEvent,
				model.ElementTimerCatchEvent,
				// required for a parallel gateway join
				model.ElementParallelGateway:
				prev = execution
			}

			// start branch
			next := ElementInstanceEntity{
				Partition: scope.Partition,

				ElementId:         targetNode.id,
				ProcessId:         scope.ProcessId,
				ProcessInstanceId: scope.ProcessInstanceId,

				BpmnElementId:   target.Id,
				BpmnElementType: target.Type,

				parent: scope,
				prev:   prev,
			}

			executions = append(executions, &next)

			scope.ExecutionCount = scope.ExecutionCount + 1
		}

		scope.ExecutionCount = scope.ExecutionCount - 1

		if scope.ExecutionCount == 0 && scope.ParentId.Valid {
			executions = append(executions, scope)
		}
	}

	if execution.State == engine.InstanceCreated {
		switch execution.BpmnElementType {
		case
			model.ElementBusinessRuleTask,
			model.ElementScriptTask,
			model.ElementSendTask,
			model.ElementServiceTask:
			attachments := g.model.AttachedTo(execution.BpmnElementId)
			for _, attachment := range attachments {
				attachedNode := g.nodes[attachment.Id]

				// start branch
				next := ElementInstanceEntity{
					Partition: scope.Partition,

					ElementId:         attachedNode.id,
					ProcessId:         scope.ProcessId,
					ProcessInstanceId: scope.ProcessInstanceId,

					BpmnElementId:   attachment.Id,
					BpmnElementType: attachment.Type,

					parent: scope,
					prev:   execution,
				}

				executions = append(executions, &next)
			}

			execution.ExecutionCount = len(attachments) * -1
		}
	}

	return executions, nil
}

func (g graph) createExecutionAt(scope *ElementInstanceEntity, bpmnElementId string) (ElementInstanceEntity, error) {
	scopeNode, ok := g.nodes[scope.BpmnElementId]
	if !ok {
		return ElementInstanceEntity{}, fmt.Errorf("BPMN process %s has no element %s", g.processElement.Id, scope.BpmnElementId)
	}

	node, ok := g.nodes[bpmnElementId]
	if !ok {
		return ElementInstanceEntity{}, fmt.Errorf("BPMN process %s has no element %s", g.processElement.Id, bpmnElementId)
	}

	if node.bpmnElement.Parent != scopeNode.bpmnElement {
		return ElementInstanceEntity{}, fmt.Errorf("BPMN scope %s has no element %s", scope.BpmnElementId, bpmnElementId)
	}

	execution := ElementInstanceEntity{
		Partition: scope.Partition,

		ElementId:         node.id,
		ProcessId:         scope.ProcessId,
		ProcessInstanceId: scope.ProcessInstanceId,

		BpmnElementId:   node.bpmnElement.Id,
		BpmnElementType: node.bpmnElement.Type,

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
	}
}

func (g graph) ensureSequenceFlow(sourceId string, targetId string) error {
	node, ok := g.nodes[sourceId]
	if !ok {
		return fmt.Errorf("BPMN process %s has no element %s", g.processElement.Id, sourceId)
	}
	if node.bpmnElement.TargetById(targetId) == nil {
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
		return nil, fmt.Errorf("BPMN process %s has no element %s", g.processElement.Id, waiting[0].BpmnElementId)
	}

	incoming := node.bpmnElement.Incoming
	if node.bpmnElement.Type != model.ElementParallelGateway || len(incoming) < 2 {
		return nil, fmt.Errorf("BPMN element %s is not a joining parallel gateway", node.bpmnElement.Id)
	}

	for i := range waiting {
		if waiting[i].ElementId != node.id {
			return nil, fmt.Errorf("expected all waiting executions to have element ID %d", node.id)
		}
	}

	incomingIds := make(map[int32]int, len(incoming))
	for i := range incoming {
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
			execution.State = engine.InstanceCompleted
		}

		incomingIds[execution.PrevElementId.Int32] = count - 1
		joined = append(joined, execution)

		if len(joined) == len(incoming) {
			return joined, nil
		}
	}

	return nil, nil
}

func (g graph) elementByElementId(elementId int32) *model.Element {
	for _, node := range g.nodes {
		if node.id == elementId {
			return node.bpmnElement
		}
	}
	return nil
}

type node struct {
	id          int32 // ID of the related ElementEntity
	bpmnElement *model.Element
}
