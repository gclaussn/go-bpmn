package model

type Element struct {
	Id   string
	Name string
	Type ElementType

	Parent   *Element
	Incoming []*SequenceFlow
	Outgoing []*SequenceFlow

	Elements      []*Element
	SequenceFlows []*SequenceFlow

	Model any
}

func (e *Element) AllElements() []*Element {
	all := []*Element{e}

	i := 0
	for i < len(all) {
		all = append(all, all[i].Elements...)
		i++
	}

	return all
}

func (e *Element) ElementById(id string) *Element {
	for i := 0; i < len(e.Elements); i++ {
		if e.Elements[i].Id == id {
			return e.Elements[i]
		}
	}
	return nil
}

func (e *Element) ElementsByType(elementType ElementType) []*Element {
	var elements []*Element
	for i := 0; i < len(e.Elements); i++ {
		if e.Elements[i].Type == elementType {
			elements = append(elements, e.Elements[i])
		}
	}
	return elements
}

func (e *Element) OutgoingById(targetId string) *Element {
	for i := 0; i < len(e.Outgoing); i++ {
		target := e.Outgoing[i].Target
		if target != nil && target.Id == targetId {
			return target
		}
	}
	return nil
}

func (e *Element) getSequenceFlow(id string) *SequenceFlow {
	for i := 0; i < len(e.SequenceFlows); i++ {
		if e.SequenceFlows[i].Id == id {
			return e.SequenceFlows[i]
		}
	}

	sequenceFlow := &SequenceFlow{Id: id}
	e.SequenceFlows = append(e.SequenceFlows, sequenceFlow)
	return sequenceFlow
}

type SequenceFlow struct {
	Id     string
	Source *Element
	Target *Element
}

// element specific models

type Process struct {
	IsExecutable bool
}
