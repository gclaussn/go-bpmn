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
		definitions   *Definitions
		element       *Element
		parentElement *Element

		isIncoming bool
		isOutgoing bool
	)

	getAttrValue := func(attributes []xml.Attr, name string) string {
		for i := range attributes {
			if attributes[i].Name.Local == name {
				return attributes[i].Value
			}
		}
		return ""
	}

	newElement := func(elementType ElementType, attributes []xml.Attr) *Element {
		element := Element{
			Id:   getAttrValue(attributes, "id"),
			Name: getAttrValue(attributes, "name"),
			Type: elementType,

			Parent: parentElement,
		}

		if parentElement != nil {
			parentElement.Elements = append(parentElement.Elements, &element)
		}

		return &element
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
			case "businessRuleTask":
				element = newElement(ElementBusinessRuleTask, t.Attr)
			case "definitions":
				definitions = &Definitions{}
				definitions.Id = getAttrValue(t.Attr, "id")
			case "endEvent":
				element = newElement(ElementNoneEndEvent, t.Attr)
			case "exclusiveGateway":
				element = newElement(ElementExclusiveGateway, t.Attr)
			case "inclusiveGateway":
				element = newElement(ElementInclusiveGateway, t.Attr)
			case "incoming":
				isIncoming = true
			case "intermediateCatchEvent":
				element = newElement(0, t.Attr) // unknown type
			case "intermediateThrowEvent":
				element = newElement(ElementNoneThrowEvent, t.Attr)
			case "manualTask":
				element = newElement(ElementManualTask, t.Attr)
			case "outgoing":
				isOutgoing = true
			case "parallelGateway":
				element = newElement(ElementParallelGateway, t.Attr)
			case "process":
				if definitions == nil {
					if err := decoder.Skip(); err != nil {
						return nil, errors.New("XML is invalid")
					}
					break
				}

				isExecutable, _ := strconv.ParseBool(getAttrValue(t.Attr, "isExecutable"))

				parentElement = newElement(ElementProcess, t.Attr)
				parentElement.Model = &Process{IsExecutable: isExecutable}

				definitions.Processes = append(definitions.Processes, parentElement)
			case "scriptTask":
				element = newElement(ElementScriptTask, t.Attr)
			case "sendTask":
				element = newElement(ElementSendTask, t.Attr)
			case "serviceTask":
				element = newElement(ElementServiceTask, t.Attr)
			case "startEvent":
				element = newElement(ElementNoneStartEvent, t.Attr)
			case "task":
				element = newElement(ElementTask, t.Attr)
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
				continue // skip processing for unknown elements
			} else if isIncoming {
				sequenceFlow := parentElement.getSequenceFlow(string(t))
				sequenceFlow.Target = element
				element.Incoming = append(element.Incoming, sequenceFlow)
			} else if isOutgoing {
				sequenceFlow := parentElement.getSequenceFlow(string(t))
				sequenceFlow.Source = element
				element.Outgoing = append(element.Outgoing, sequenceFlow)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "incoming":
				isIncoming = false
			case "intermediateCatchEvent":
				if element.Type == 0 { // if unknown, handle as pass through element
					element.Type = ElementNoneThrowEvent
				}
			case "outgoing":
				isOutgoing = false
			}
		}
	}

	if definitions == nil {
		return nil, errors.New("no definitions found")
	}

	return &Model{Definitions: definitions}, nil
}

type Definitions struct {
	Id        string
	Processes []*Element
}

type Model struct {
	Definitions *Definitions
}

func (m *Model) ProcessById(id string) (*Element, error) {
	for i := range m.Definitions.Processes {
		if m.Definitions.Processes[i].Id == id {
			return m.Definitions.Processes[i], nil
		}
	}
	return nil, fmt.Errorf("failed to find BPMN process by ID %s", id)
}
