package model

type Element struct {
	Id   string
	Name string
	Type ElementType

	Parent   *Element
	Children []*Element

	Incoming []*SequenceFlow
	Outgoing []*SequenceFlow

	Model any
}

// ChildById returns the child element with the given id, or nil, if no such element exists.
func (e *Element) ChildById(id string) *Element {
	for _, child := range e.Children {
		if child.Id == id {
			return child
		}
	}
	return nil
}

// ChildrenByType returns all child elements of the given type.
func (e *Element) ChildrenByType(elementType ElementType) []*Element {
	var children []*Element
	for _, child := range e.Children {
		if child.Type == elementType {
			children = append(children, child)
		}
	}
	return children
}

// OutgoingById returns the outgoing sequence flow with the given id or nil, if no such sequence flow exists.
func (e *Element) OutgoingById(id string) *SequenceFlow {
	for i := range e.Outgoing {
		if e.Outgoing[i].Id == id {
			return e.Outgoing[i]
		}
	}
	return nil
}

// TargetById returns the target element with the given id, connected with an outgoing sequence flow, or nil, if no such element exists.
func (e *Element) TargetById(targetId string) *Element {
	for i := range e.Outgoing {
		target := e.Outgoing[i].Target
		if target != nil && target.Id == targetId {
			return target
		}
	}
	return nil
}

type SequenceFlow struct {
	Id     string
	Source *Element
	Target *Element
}

// element specific models

type ExclusiveGateway struct {
	Default string // Optional ID of a default sequence flow.
}

type InclusiveGateway struct {
	Default string // Optional ID of a default sequence flow.
}

type Process struct {
	IsExecutable bool
}
