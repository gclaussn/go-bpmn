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
			Detail:  fmt.Sprintf("process %s is not executable", bpmnElements[0].Id),
		})
	}

	for _, bpmnElement := range bpmnElements {
		if bpmnElement.Id == "" {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(bpmnElement),
				Type:    "element",
				Detail:  fmt.Sprintf("element of type %s has no ID", bpmnElement.Type),
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
					Detail:  fmt.Sprintf("element %s is not supported: joining inclusive gateway", bpmnElement.Id),
				})
			}
		case model.ElementSubProcess:
			subProcess := bpmnElement.Model.(model.SubProcess)
			if subProcess.TriggeredByEvent {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "element",
					Detail:  fmt.Sprintf("element %s is not supported: event sub-process", bpmnElement.Id),
				})
				break
			}

			for _, child := range bpmnElement.Children {
				if model.IsStartEvent(child.Type) {
					if child.Type == model.ElementNoneStartEvent {
						continue
					}

					causes = append(causes, engine.ErrorCause{
						Pointer: elementPointer(child),
						Type:    "element",
						Detail:  fmt.Sprintf("sub-process %s cannot be started by event %s: not a none start event", bpmnElement.Id, child.Id),
					})
				}
			}

			noneStartEvents := bpmnElement.ChildrenByType(model.ElementNoneStartEvent)
			if len(noneStartEvents) != 1 {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "element",
					Detail:  fmt.Sprintf("sub-process %s has no or multiple none start events", bpmnElement.Id),
				})
			}
		}

		if model.IsBoundaryEvent(bpmnElement.Type) {
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
					Detail:  fmt.Sprintf("sequence flow %s has no source element", sequenceFlow.Id),
				})
			}
		}
		for _, sequenceFlow := range bpmnElement.Outgoing {
			if sequenceFlow.Target == nil {
				causes = append(causes, engine.ErrorCause{
					Pointer: fmt.Sprintf("%s/%s", elementPointer(bpmnElement.Parent), sequenceFlow.Id),
					Type:    "sequence_flow",
					Detail:  fmt.Sprintf("sequence flow %s has no target element", sequenceFlow.Id),
				})
			}
		}
	}

	noneStartEvents := bpmnElements[0].ChildrenByType(model.ElementNoneStartEvent)
	if len(noneStartEvents) > 1 {
		causes = append(causes, engine.ErrorCause{
			Pointer: elementPointer(bpmnElements[0]),
			Type:    "process",
			Detail:  fmt.Sprintf("process %s has multiple none start events", bpmnElements[0].Id),
		})
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
	for _, element := range elements {
		ids[element.BpmnElementId] = element.Id
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

		var boundaryEvents []*model.Element
		if isTaskOrScope(bpmnElement.Type) {
			boundaryEvents = bpmnModel.AttachedTo(bpmnElement.Id)
		}

		nodes[bpmnElement.Id] = node{
			id:             id,
			bpmnElement:    bpmnElement,
			boundaryEvents: boundaryEvents,
		}
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

func (g graph) continueExecution(executions []*ElementInstanceEntity, execution *ElementInstanceEntity) ([]*ElementInstanceEntity, error) {
	node, err := g.node(execution.BpmnElementId)
	if err != nil {
		return executions, err
	}

	scope := execution.parent

	if execution.State == engine.InstanceCompleted || execution.State == engine.InstanceTerminated {
		scope.ExecutionCount--
		return executions, nil // skip already completed or terminated execution
	}

	if scope.State == engine.InstanceQueued {
		// end branch
		execution.State = engine.InstanceQueued
	} else if execution.State == engine.InstanceStarted {
		// continue branch
		execution.State = engine.InstanceCompleted
	} else if scope.State == engine.InstanceSuspended {
		// end branch
		execution.State = engine.InstanceSuspended
	} else if execution.State == engine.InstanceCreated {
		// end branch
		if execution.ExecutionCount < 0 { // task, sub process or call activity, but not all boundary events defined
			execution.State = engine.InstanceCreated
		} else if execution.Context.Valid {
			if model.IsBoundaryEvent(execution.BpmnElementType) {
				// defined boundary event
				execution.State = engine.InstanceCreated
			} else {
				execution.State = engine.InstanceStarted
			}
		} else {
			execution.State = engine.InstanceStarted
		}
	} else if model.IsBoundaryEvent(execution.BpmnElementType) {
		execution.State = engine.InstanceCreated
	} else if isTaskOrScope(execution.BpmnElementType) {
		if execution.State != 0 && execution.ExecutionCount == 0 {
			execution.State = engine.InstanceStarted
		} else {
			execution.State = engine.InstanceCreated
		}
	} else {
		switch execution.BpmnElementType {
		// gateway
		case
			model.ElementExclusiveGateway,
			model.ElementInclusiveGateway:
			if len(node.bpmnElement.Outgoing) > 1 {
				execution.State = engine.InstanceStarted
			} else {
				execution.State = engine.InstanceCompleted
			}
		case model.ElementParallelGateway:
			if len(node.bpmnElement.Incoming) > 1 {
				execution.State = engine.InstanceStarted
			} else {
				execution.State = engine.InstanceCompleted
			}
		// event
		case model.ElementMessageCatchEvent:
			execution.State = engine.InstanceCreated
		case
			model.ElementMessageEndEvent,
			model.ElementMessageThrowEvent:
			execution.State = engine.InstanceStarted
		case
			model.ElementErrorEndEvent,
			model.ElementSignalCatchEvent,
			model.ElementSignalEndEvent,
			model.ElementSignalThrowEvent,
			model.ElementTimerCatchEvent:
			if node.eventDefinition == nil {
				execution.State = engine.InstanceCreated
			} else {
				execution.State = engine.InstanceStarted
			}
		default:
			// continue branch, if element has no behavior (pass through element)
			execution.State = engine.InstanceCompleted
		}
	}

	switch execution.State {
	case engine.InstanceCompleted:
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
			scope.ExecutionCount++
		}

		scope.ExecutionCount--
	case engine.InstanceCreated, engine.InstanceSuspended:
		if isTaskOrScope(execution.BpmnElementType) {
			if execution.ExecutionCount < 0 { // not all boundary events defined
				return executions, nil
			}

			executionCount := 0

			for _, boundaryEvent := range node.boundaryEvents {
				boundaryEventNode := g.nodes[boundaryEvent.Id]

				// start branch
				attached := ElementInstanceEntity{
					Partition: scope.Partition,

					ElementId:         boundaryEventNode.id,
					ProcessId:         scope.ProcessId,
					ProcessInstanceId: scope.ProcessInstanceId,

					BpmnElementId:   boundaryEvent.Id,
					BpmnElementType: boundaryEvent.Type,

					parent: scope,
					prev:   execution,
				}

				if boundaryEventNode.eventDefinition == nil || boundaryEvent.Type == model.ElementMessageBoundaryEvent {
					executionCount--
				}

				executions = append(executions, &attached)
				scope.ExecutionCount++
			}

			if executionCount == 0 {
				execution.State = engine.InstanceStarted
			} else {
				execution.ExecutionCount = executionCount
			}
		}
	}

	if execution.State == engine.InstanceStarted {
		if execution.BpmnElementType == model.ElementSubProcess {
			// start branch
			next, err := g.createExecution(execution)
			if err != nil {
				return executions, err
			}

			executions = append(executions, &next)
		}
	}

	return executions, nil
}

func (g graph) createExecution(scope *ElementInstanceEntity) (ElementInstanceEntity, error) {
	scopeNode, err := g.node(scope.BpmnElementId)
	if err != nil {
		return ElementInstanceEntity{}, err
	}

	var noneStartEvents []*model.Element
	switch scopeNode.bpmnElement.Type {
	case
		model.ElementProcess,
		model.ElementSubProcess:
		noneStartEvents = scopeNode.bpmnElement.ChildrenByType(model.ElementNoneStartEvent)
	}

	if len(noneStartEvents) == 0 {
		return ElementInstanceEntity{}, fmt.Errorf("scope %s has no none start event", scope.BpmnElementId)
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

	scope.ExecutionCount++

	return execution, nil
}

func (g graph) createExecutionAt(scope *ElementInstanceEntity, bpmnElementId string) (ElementInstanceEntity, error) {
	scopeNode, err := g.node(scope.BpmnElementId)
	if err != nil {
		return ElementInstanceEntity{}, err
	}

	node, err := g.node(bpmnElementId)
	if err != nil {
		return ElementInstanceEntity{}, err
	}

	if node.bpmnElement.Parent != scopeNode.bpmnElement {
		return ElementInstanceEntity{}, fmt.Errorf("scope %s has no element %s", scope.BpmnElementId, bpmnElementId)
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

	scope.ExecutionCount++

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
	node, err := g.node(sourceId)
	if err != nil {
		return err
	}
	if node.bpmnElement.TargetById(targetId) == nil {
		return fmt.Errorf("element %s has no outgoing sequence flow to %s", sourceId, targetId)
	}
	return nil
}

func (g graph) joinParallelGateway(waiting []*ElementInstanceEntity) ([]*ElementInstanceEntity, error) {
	if len(waiting) < 2 {
		return nil, nil
	}

	node, err := g.node(waiting[0].BpmnElementId)
	if err != nil {
		return nil, err
	}

	incoming := node.bpmnElement.Incoming
	if node.bpmnElement.Type != model.ElementParallelGateway || len(incoming) < 2 {
		return nil, fmt.Errorf("element %s is not a joining parallel gateway", node.bpmnElement.Id)
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

		if i != 0 {
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

func (g graph) node(bpmnElementId string) (node, error) {
	if node, ok := g.nodes[bpmnElementId]; ok {
		return node, nil
	} else {
		return node, fmt.Errorf("process %s has no element %s", g.processElement.Id, bpmnElementId)
	}
}

// setEventDefinitions enriches graph nodes by adding event definitions.
func (g graph) setEventDefinitions(eventDefinitions []*EventDefinitionEntity) {
	for _, eventeventDefinition := range eventDefinitions {
		node, ok := g.nodes[eventeventDefinition.BpmnElementId]
		if !ok {
			continue
		}

		node.eventDefinition = eventeventDefinition
		g.nodes[eventeventDefinition.BpmnElementId] = node
	}
}

type node struct {
	id              int32 // ID of the related ElementEntity
	bpmnElement     *model.Element
	boundaryEvents  []*model.Element
	eventDefinition *EventDefinitionEntity
}
