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

		element *Element

		isBoundaryEvent bool

		isIncoming bool
		isOutgoing bool
	)

	startElement := func(elementType ElementType, attributes []xml.Attr) {
		newElement := &Element{
			Id:   getAttrValue(attributes, "id"),
			Name: getAttrValue(attributes, "name"),
			Type: elementType,
		}

		if element != nil {
			newElement.Parent = element
			element.Children = append(element.Children, newElement)
		}

		element = newElement
		elements = append(elements, element)
	}

	endElement := func() {
		element = element.Parent
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
				cancelActivity, _ := strconv.ParseBool(getAttrValueWithDefault(t.Attr, "cancelActivity", "true"))

				startElement(0, t.Attr) // unknown type
				element.Model = BoundaryEvent{
					AttachedTo:     &Element{Id: getAttrValue(t.Attr, "attachedToRef")},
					CancelActivity: cancelActivity,
				}

				isBoundaryEvent = true
			case "businessRuleTask":
				startElement(ElementBusinessRuleTask, t.Attr)
			case "definitions":
				definitions.Id = getAttrValue(t.Attr, "id")
				definitionsParsed = true
			case "endEvent":
				startElement(ElementNoneEndEvent, t.Attr)
			case "error":
				bpmnErrorId := getAttrValue(t.Attr, "id")
				bpmnError := definitions.errorById(bpmnErrorId)
				bpmnError.Name = getAttrValue(t.Attr, "name")
				bpmnError.Code = getAttrValue(t.Attr, "errorCode")
			case "errorEventDefinition":
				if element.Type == 0 {
					element.Type = ElementErrorBoundaryEvent
				}

				eventDefinition := EventDefinition{Id: getAttrValue(t.Attr, "id")}

				if bpmnErrorId := getAttrValue(t.Attr, "errorRef"); bpmnErrorId != "" {
					eventDefinition.Error = definitions.errorById(bpmnErrorId)
				}

				switch model := element.Model.(type) {
				case BoundaryEvent:
					model.EventDefinition = eventDefinition
					element.Model = model
				}
			case "escalation":
				escalationId := getAttrValue(t.Attr, "id")
				escalation := definitions.escalationById(escalationId)
				escalation.Name = getAttrValue(t.Attr, "name")
				escalation.Code = getAttrValue(t.Attr, "escalationCode")
			case "escalationEventDefinition":
				if element.Type == 0 {
					element.Type = ElementEscalationBoundaryEvent
				}

				eventDefinition := EventDefinition{Id: getAttrValue(t.Attr, "id")}

				if escalationId := getAttrValue(t.Attr, "escalationRef"); escalationId != "" {
					eventDefinition.Escalation = definitions.escalationById(escalationId)
				}

				switch model := element.Model.(type) {
				case BoundaryEvent:
					model.EventDefinition = eventDefinition
					element.Model = model
				}
			case "exclusiveGateway":
				startElement(ElementExclusiveGateway, t.Attr)
				element.Model = ExclusiveGateway{Default: getAttrValue(t.Attr, "default")}
			case "inclusiveGateway":
				startElement(ElementInclusiveGateway, t.Attr)
				element.Model = InclusiveGateway{Default: getAttrValue(t.Attr, "default")}
			case "incoming":
				isIncoming = true
			case "intermediateCatchEvent":
				startElement(0, t.Attr) // unknown type
				element.Model = IntermediateCatchEvent{}
			case "intermediateThrowEvent":
				startElement(ElementNoneThrowEvent, t.Attr)
			case "manualTask":
				startElement(ElementManualTask, t.Attr)
			case "message":
				messageId := getAttrValue(t.Attr, "id")
				message := definitions.messageById(messageId)
				message.Name = getAttrValue(t.Attr, "name")
			case "messageEventDefinition":
				if isBoundaryEvent {
					element.Type = ElementMessageBoundaryEvent
				} else if element.Type == ElementNoneStartEvent {
					element.Type = ElementMessageStartEvent
				} else if element.Type == ElementNoneEndEvent {
					element.Type = ElementMessageEndEvent
				} else if element.Type == ElementNoneThrowEvent {
					element.Type = ElementMessageThrowEvent
				} else {
					element.Type = ElementMessageCatchEvent
				}

				eventDefinition := EventDefinition{Id: getAttrValue(t.Attr, "id")}

				if messageId := getAttrValue(t.Attr, "messageRef"); messageId != "" {
					eventDefinition.Message = definitions.messageById(messageId)
				}

				switch model := element.Model.(type) {
				case BoundaryEvent:
					model.EventDefinition = eventDefinition
					element.Model = model
				case IntermediateCatchEvent:
					model.EventDefinition = eventDefinition
					element.Model = model
				case StartEvent:
					model.EventDefinition = eventDefinition
					element.Model = model
				}
			case "outgoing":
				isOutgoing = true
			case "parallelGateway":
				startElement(ElementParallelGateway, t.Attr)
			case "process":
				isExecutable, _ := strconv.ParseBool(getAttrValue(t.Attr, "isExecutable"))

				startElement(ElementProcess, t.Attr)
				element.Model = Process{IsExecutable: isExecutable}

				definitions.Processes = append(definitions.Processes, element)
			case "scriptTask":
				startElement(ElementScriptTask, t.Attr)
			case "sendTask":
				startElement(ElementSendTask, t.Attr)
			case "serviceTask":
				startElement(ElementServiceTask, t.Attr)
			case "signal":
				signalId := getAttrValue(t.Attr, "id")
				signal := definitions.signalById(signalId)
				signal.Name = getAttrValue(t.Attr, "name")
			case "signalEventDefinition":
				if isBoundaryEvent {
					element.Type = ElementSignalBoundaryEvent
				} else if element.Type == ElementNoneStartEvent {
					element.Type = ElementSignalStartEvent
				} else {
					element.Type = ElementSignalCatchEvent
				}

				eventDefinition := EventDefinition{Id: getAttrValue(t.Attr, "id")}

				if signalId := getAttrValue(t.Attr, "signalRef"); signalId != "" {
					eventDefinition.Signal = definitions.signalById(signalId)
				}

				switch model := element.Model.(type) {
				case BoundaryEvent:
					model.EventDefinition = eventDefinition
					element.Model = model
				case IntermediateCatchEvent:
					model.EventDefinition = eventDefinition
					element.Model = model
				case StartEvent:
					model.EventDefinition = eventDefinition
					element.Model = model
				}
			case "startEvent":
				startElement(ElementNoneStartEvent, t.Attr)
				element.Model = StartEvent{}
			case "subProcess":
				triggeredByEvent, _ := strconv.ParseBool(getAttrValue(t.Attr, "triggeredByEvent"))

				startElement(ElementSubProcess, t.Attr)
				element.Model = SubProcess{TriggeredByEvent: triggeredByEvent}
			case "task":
				startElement(ElementTask, t.Attr)
			case "timerEventDefinition":
				if isBoundaryEvent {
					element.Type = ElementTimerBoundaryEvent
				} else if element.Type == ElementNoneStartEvent {
					element.Type = ElementTimerStartEvent
				} else {
					element.Type = ElementTimerCatchEvent
				}
			default:
				if err := decoder.Skip(); err != nil {
					return nil, fmt.Errorf("failed to skip XML element: %v", err)
				}
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
				if element.Type == 0 {
					return nil, fmt.Errorf("failed to determine type of boundary event %s", element.Id)
				}

				isBoundaryEvent = false
				endElement()
			case "incoming":
				isIncoming = false
			case "intermediateCatchEvent":
				if element.Type == 0 {
					return nil, fmt.Errorf("failed to determine type of intermediate catch event %s", element.Id)
				}

				endElement()
			case "outgoing":
				isOutgoing = false
			case
				"businessRuleTask",
				"endEvent",
				"exclusiveGateway",
				"inclusiveGateway",
				"intermediateThrowEvent",
				"manualTask",
				"parallelGateway",
				"process",
				"scriptTask",
				"sendTask",
				"serviceTask",
				"startEvent",
				"subProcess",
				"task":
				endElement()
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
		if IsBoundaryEvent(element.Type) {
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

	elements := make([]*Element, 1, len(processElement.Children)+1)
	elements[0] = processElement

	i := 0
	for i < len(elements) {
		elements = append(elements, elements[i].Children...)
		i++
	}

	return elements
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

	Errors      []*Error
	Escalations []*Escalation
	Messages    []*Message
	Processes   []*Element
	Signals     []*Signal
}

func (d *Definitions) errorById(id string) *Error {
	for _, bpmnError := range d.Errors {
		if bpmnError.Id == id {
			return bpmnError
		}
	}

	bpmnError := &Error{Id: id}
	d.Errors = append(d.Errors, bpmnError)
	return bpmnError
}

func (d *Definitions) escalationById(id string) *Escalation {
	for _, escalation := range d.Escalations {
		if escalation.Id == id {
			return escalation
		}
	}

	escalation := &Escalation{Id: id}
	d.Escalations = append(d.Escalations, escalation)
	return escalation
}

func (d *Definitions) messageById(id string) *Message {
	for _, message := range d.Messages {
		if message.Id == id {
			return message
		}
	}

	message := &Message{Id: id}
	d.Messages = append(d.Messages, message)
	return message
}

func (d *Definitions) signalById(id string) *Signal {
	for _, signal := range d.Signals {
		if signal.Id == id {
			return signal
		}
	}

	signal := &Signal{Id: id}
	d.Signals = append(d.Signals, signal)
	return signal
}

type Error struct {
	Id   string
	Name string
	Code string
}

type Escalation struct {
	Id   string
	Name string
	Code string
}

type Message struct {
	Id   string
	Name string
}

type Signal struct {
	Id   string
	Name string
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

func getAttrValueWithDefault(attributes []xml.Attr, name string, defaultValue string) string {
	if value := getAttrValue(attributes, name); value != "" {
		return value
	} else {
		return defaultValue
	}
}
