package model

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strconv"
)

func New(bpmnXmlReader io.Reader) (*Model, error) {
	var (
		definitions       Definitions
		definitionsParsed bool

		elements      []*Element
		sequenceFlows []*SequenceFlow

		element       *Element
		parentElement *Element

		isIncoming bool
		isOutgoing bool
	)

	addElement := func() {
		if parentElement != nil {
			element.Parent = parentElement
			parentElement.Children = append(parentElement.Children, element)
		}

		elements = append(elements, element)
	}

	addNewElement := func(elementType ElementType, attributes []xml.Attr) {
		element = newElement(elementType, attributes)
		addElement()
	}

	sequenceFlowById := func(id string) *SequenceFlow {
		for _, sequenceFlow := range sequenceFlows {
			if sequenceFlow.Id == id {
				return sequenceFlow
			}
		}

		sequenceFlow := &SequenceFlow{Id: id}
		sequenceFlows = append(sequenceFlows, sequenceFlow)
		return sequenceFlow
	}

	decoder := xml.NewDecoder(bpmnXmlReader)

	count := 0
	for {
		token, err := decoder.Token()
		if token == nil || err == io.EOF {
			if count == 0 {
				return nil, errors.New("XML is empty")
			}
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to decode XML: %v", err)
		}

		count++

		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "boundaryEvent":
				element = newElement(0, t.Attr) // unknown type
				element.Model = BoundaryEvent{AttachedTo: &Element{Id: getAttrValue(t.Attr, "attachedToRef")}}
			case "businessRuleTask":
				addNewElement(ElementBusinessRuleTask, t.Attr)
			case "definitions":
				definitions.Id = getAttrValue(t.Attr, "id")
				definitionsParsed = true
			case "endEvent":
				addNewElement(ElementNoneEndEvent, t.Attr)
			case "errorEventDefinition":
				if element.Type == 0 {
					element.Type = ElementErrorBoundaryEvent
				}
			case "exclusiveGateway":
				addNewElement(ElementExclusiveGateway, t.Attr)
				element.Model = ExclusiveGateway{Default: getAttrValue(t.Attr, "default")}
			case "inclusiveGateway":
				addNewElement(ElementInclusiveGateway, t.Attr)
				element.Model = InclusiveGateway{Default: getAttrValue(t.Attr, "default")}
			case "incoming":
				isIncoming = true
			case "intermediateCatchEvent":
				element = newElement(0, t.Attr) // unknown type
			case "intermediateThrowEvent":
				addNewElement(ElementNoneThrowEvent, t.Attr)
			case "manualTask":
				addNewElement(ElementManualTask, t.Attr)
			case "messageEventDefinition":
				switch element.Type {
				case ElementNoneStartEvent:
					element.Type = ElementMessageStartEvent
				default:
					element.Type = ElementMessageCatchEvent
				}
			case "outgoing":
				isOutgoing = true
			case "parallelGateway":
				addNewElement(ElementParallelGateway, t.Attr)
			case "process":
				isExecutable, _ := strconv.ParseBool(getAttrValue(t.Attr, "isExecutable"))

				addNewElement(ElementProcess, t.Attr)
				parentElement = element
				parentElement.Model = Process{IsExecutable: isExecutable}

				definitions.Processes = append(definitions.Processes, parentElement)
			case "scriptTask":
				addNewElement(ElementScriptTask, t.Attr)
			case "sendTask":
				addNewElement(ElementSendTask, t.Attr)
			case "serviceTask":
				addNewElement(ElementServiceTask, t.Attr)
			case "signalEventDefinition":
				switch element.Type {
				case ElementNoneStartEvent:
					element.Type = ElementSignalStartEvent
				default:
					element.Type = ElementSignalCatchEvent
				}
			case "startEvent":
				addNewElement(ElementNoneStartEvent, t.Attr)
			case "task":
				addNewElement(ElementTask, t.Attr)
			case "timerEventDefinition":
				switch element.Type {
				case ElementNoneStartEvent:
					element.Type = ElementTimerStartEvent
				default:
					element.Type = ElementTimerCatchEvent
				}
			default:
				element = nil
			}
		case xml.CharData:
			if element == nil {
				continue // skip unknown element
			} else if isIncoming {
				sequenceFlow := sequenceFlowById(string(t))
				sequenceFlow.Target = element
				element.Incoming = append(element.Incoming, sequenceFlow)
			} else if isOutgoing {
				sequenceFlow := sequenceFlowById(string(t))
				sequenceFlow.Source = element
				element.Outgoing = append(element.Outgoing, sequenceFlow)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "boundaryEvent":
				if element.Type != 0 { // add element, if type is known
					addElement()
				}
			case "incoming":
				isIncoming = false
			case "intermediateCatchEvent":
				if element.Type != 0 { // add element, if type is known
					addElement()
				}
			case "outgoing":
				isOutgoing = false
			}
		}
	}

	if !definitionsParsed {
		return nil, errors.New("no definitions found")
	}

	model := Model{
		Definitions: &definitions,

		Elements:      elements,
		SequenceFlows: sequenceFlows,
	}

	for _, element := range model.Elements {
		switch element.Type {
		case ElementErrorBoundaryEvent:
			// resolve "attached to" placeholder
			boundaryEvent := element.Model.(BoundaryEvent)

			attachedTo := model.ElementById(boundaryEvent.AttachedTo.Id)
			if attachedTo != nil {
				model.attachments = append(model.attachments, attachment{
					Id:      attachedTo.Id,
					Element: element,
				})
			}

			boundaryEvent.AttachedTo = attachedTo
			element.Model = boundaryEvent
		}
	}

	return &model, nil
}

type Model struct {
	Definitions *Definitions

	Elements      []*Element
	SequenceFlows []*SequenceFlow

	attachments []attachment
}

// AttachedTo returns all boundary events that are attached to a specific task, sub process or call activity.
func (m *Model) AttachedTo(id string) []*Element {
	var elements []*Element
	for _, attachment := range m.attachments {
		if attachment.Id == id {
			elements = append(elements, attachment.Element)
		}
	}
	return elements
}

// ElementById returns the element with the given id, or nil, if no such element exists.
func (m *Model) ElementById(id string) *Element {
	for _, element := range m.Elements {
		if element.Id == id {
			return element
		}
	}
	return nil
}

// ElementsByProcessId returns all elements of a process, including the process element itself.
// If the process does not exist, nil is returned.
func (m *Model) ElementsByProcessId(processId string) []*Element {
	processElement := m.ProcessById(processId)
	if processElement == nil {
		return nil
	}

	bpmnElements := make([]*Element, 1, len(processElement.Children)+1)
	bpmnElements[0] = processElement

	i := 0
	for i < len(bpmnElements) {
		bpmnElements = append(bpmnElements, bpmnElements[i].Children...)
		i++
	}

	return bpmnElements
}

// ElementsByType returns all elements of the given type.
func (m *Model) ElementsByType(elementType ElementType) []*Element {
	var elements []*Element
	for _, element := range m.Elements {
		if element.Type == elementType {
			elements = append(elements, element)
		}
	}
	return elements
}

// ProcessById returns the process with the given id, or nil, if no such process exists.
func (m *Model) ProcessById(id string) *Element {
	for i := range m.Definitions.Processes {
		if m.Definitions.Processes[i].Id == id {
			return m.Definitions.Processes[i]
		}
	}
	return nil
}

type Definitions struct {
	Id string

	Processes []*Element
}

type SequenceFlow struct {
	Id     string
	Source *Element
	Target *Element
}

// attachment represent an attached to relation between a boundary event and a task, sub process or call activity.
type attachment struct {
	Id      string   // ID of a task or sub process.
	Element *Element // The attached element.
}

func getAttrValue(attributes []xml.Attr, name string) string {
	for i := range attributes {
		if attributes[i].Name.Local == name {
			return attributes[i].Value
		}
	}
	return ""
}

func newElement(elementType ElementType, attributes []xml.Attr) *Element {
	return &Element{
		Id:   getAttrValue(attributes, "id"),
		Name: getAttrValue(attributes, "name"),
		Type: elementType,
	}
}
