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
		elements      []*Element
		sequenceFlows []*SequenceFlow

		definitions *Definitions

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
			case "businessRuleTask":
				addNewElement(ElementBusinessRuleTask, t.Attr)
			case "definitions":
				definitions = &Definitions{}
				definitions.Id = getAttrValue(t.Attr, "id")
			case "endEvent":
				addNewElement(ElementNoneEndEvent, t.Attr)
			case "exclusiveGateway":
				addNewElement(ElementExclusiveGateway, t.Attr)
				element.Model = ExclusiveGateway{
					Default: getAttrValue(t.Attr, "default"),
				}
			case "inclusiveGateway":
				addNewElement(ElementInclusiveGateway, t.Attr)
				element.Model = InclusiveGateway{
					Default: getAttrValue(t.Attr, "default"),
				}
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
				if definitions == nil {
					if err := decoder.Skip(); err != nil {
						return nil, errors.New("XML is invalid")
					}
					break
				}

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
			if definitions == nil {
				if err := decoder.Skip(); err != nil {
					return nil, errors.New("XML is invalid")
				}
				break
			}
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

	if definitions == nil {
		return nil, errors.New("no definitions found")
	}

	return &Model{
		Definitions: definitions,

		Elements:      elements,
		SequenceFlows: sequenceFlows,
	}, nil
}

type Definitions struct {
	Id        string
	Processes []*Element
}

type Model struct {
	Definitions *Definitions

	Elements      []*Element
	SequenceFlows []*SequenceFlow
}

func (m *Model) ElementById(id string) *Element {
	for _, element := range m.Elements {
		if element.Id == id {
			return element
		}
	}
	return nil
}

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

func (m *Model) ElementsByType(elementType ElementType) []*Element {
	var elements []*Element
	for _, element := range m.Elements {
		if element.Type == elementType {
			elements = append(elements, element)
		}
	}
	return elements
}

func (m *Model) ProcessById(id string) *Element {
	for i := range m.Definitions.Processes {
		if m.Definitions.Processes[i].Id == id {
			return m.Definitions.Processes[i]
		}
	}
	return nil
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
