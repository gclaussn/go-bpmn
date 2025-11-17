package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvalidXml(t *testing.T) {
	if _, err := New(strings.NewReader("")); err == nil {
		t.Fatal("expected error when XML is empty")
	}

	if _, err := New(strings.NewReader("#")); err == nil {
		t.Fatal("expected error when XML is invalid")
	}

	if _, err := New(strings.NewReader("<process></process>")); err == nil {
		t.Fatal("expected error when XML contains no definitions")
	}

	if _, err := New(strings.NewReader("<process></process1>")); err == nil {
		t.Fatal("expected error when XML is invalid")
	}
}

func TestUnknownElement(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// when
	model := mustCreateModel(t, "invalid/unknown-element.bpmn")

	// then
	assert.Equal("test", model.Definitions.Id)

	require.Len(model.Definitions.Processes, 1)

	processElement := model.Definitions.Processes[0]
	require.Len(processElement.Children, 2)
	require.Nil(processElement.ChildById("unknownElement"))

	startEvent := processElement.Children[0]
	require.Len(startEvent.Outgoing, 1)

	assert.NotNil(startEvent.Outgoing[0].Source)
	assert.Nil(startEvent.Outgoing[0].Target)

	endEvent := processElement.Children[1]
	require.Len(endEvent.Incoming, 1)

	assert.Nil(endEvent.Incoming[0].Source)
	assert.NotNil(endEvent.Incoming[0].Target)
}

func TestUnknownBoundaryEvent(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// when
	model := mustCreateModel(t, "invalid/unknown-boundary-event.bpmn")

	// then
	require.Len(model.Definitions.Processes, 1)

	processElement := model.Definitions.Processes[0]
	assert.Len(processElement.Children, 4)
	assert.Nil(processElement.ChildById("unknownBoundaryEvent"))
}

func TestUnknownCatchEvent(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// when
	model := mustCreateModel(t, "invalid/unknown-catch-event.bpmn")

	// then
	require.Len(model.Definitions.Processes, 1)

	processElement := model.Definitions.Processes[0]
	assert.Len(processElement.Children, 2)
	assert.Nil(processElement.ChildById("unknownCatchEvent"))
}

func TestServiceTask(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// when
	model := mustCreateModel(t, "task/service.bpmn")

	// then
	processElement := model.ProcessById("serviceTest")
	require.NotNil(processElement)

	assert.Len(processElement.Children, 3)
	assert.Equal("serviceTest", processElement.Id)
	assert.Equal("", processElement.Name)
	assert.Equal(ElementProcess, processElement.Type)

	assert.Nil(processElement.Parent)

	assert.Empty(processElement.Incoming)
	assert.Empty(processElement.Outgoing)

	process := processElement.Model.(Process)
	assert.True(process.IsExecutable)

	startEvent := model.ElementById("startEvent")
	require.NotNil(startEvent)

	assert.Equal("startEvent", startEvent.Id)
	assert.Equal("", startEvent.Name)
	assert.Equal(ElementNoneStartEvent, startEvent.Type)

	assert.Equal(processElement, startEvent.Parent)
	assert.Empty(startEvent.Children)

	assert.Len(startEvent.Incoming, 0)
	assert.Len(startEvent.Outgoing, 1)

	serviceTask := model.ElementById("serviceTask")
	require.NotNil(serviceTask)

	assert.Equal("serviceTask", serviceTask.Id)
	assert.Equal("", serviceTask.Name)
	assert.Equal(ElementServiceTask, serviceTask.Type)

	assert.Equal(processElement, serviceTask.Parent)
	assert.Empty(serviceTask.Children)

	assert.Len(serviceTask.Incoming, 1)
	assert.Len(serviceTask.Outgoing, 1)

	endEvent := model.ElementById("endEvent")
	require.NotNil(endEvent)

	assert.Equal("endEvent", endEvent.Id)
	assert.Equal("", endEvent.Name)
	assert.Equal(ElementNoneEndEvent, endEvent.Type)

	assert.Equal(processElement, endEvent.Parent)
	assert.Empty(endEvent.Children)

	assert.Len(endEvent.Incoming, 1)
	assert.Len(endEvent.Outgoing, 0)

	assert.Equal(startEvent.Outgoing[0], serviceTask.Incoming[0])
	assert.Equal(serviceTask.Outgoing[0], endEvent.Incoming[0])

	sequenceFlow1 := startEvent.Outgoing[0]
	assert.Equal(startEvent, sequenceFlow1.Source)
	assert.Equal(serviceTask, sequenceFlow1.Target)

	sequenceFlow2 := endEvent.Incoming[0]
	assert.Equal(serviceTask, sequenceFlow2.Source)
	assert.Equal(endEvent, sequenceFlow2.Target)

	noneStartEvents := processElement.ChildrenByType(ElementNoneStartEvent)
	require.Len(noneStartEvents, 1)

	assert.Equal(noneStartEvents[0], startEvent)
}

func TestErrorBoundaryEvent(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// when
	model := mustCreateModel(t, "event/error-boundary-event.bpmn")

	// then
	processElement := model.ProcessById("errorBoundaryEventTest")
	require.NotNil(processElement)
	assert.Len(processElement.Children, 5)

	errorBoundaryEvent := processElement.ChildById("errorBoundaryEvent")
	require.NotNil(errorBoundaryEvent)

	assert.Equal(ElementErrorBoundaryEvent, errorBoundaryEvent.Type)

	assert.Len(errorBoundaryEvent.Incoming, 0)
	assert.Len(errorBoundaryEvent.Outgoing, 1)

	boundaryEvent := errorBoundaryEvent.Model.(BoundaryEvent)
	require.NotNil(boundaryEvent.AttachedTo)
	assert.Equal("serviceTask", boundaryEvent.AttachedTo.Id)
	assert.Equal(ElementServiceTask, boundaryEvent.AttachedTo.Type)
	assert.True(boundaryEvent.CancelActivity)

	assert.Equal("errorBoundaryEventDefinition", boundaryEvent.EventDefinition.Id)
	assert.Nil(boundaryEvent.EventDefinition.Error)
}

func TestErrorBoundaryEventDefinition(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// when
	model := mustCreateModel(t, "event/error-boundary-event-definition.bpmn")

	// then
	processElement := model.ProcessById("errorBoundaryEventDefinitionTest")
	require.NotNil(processElement)
	assert.Len(processElement.Children, 5)

	errorBoundaryEvent := processElement.ChildById("errorBoundaryEvent")
	require.NotNil(errorBoundaryEvent)

	assert.Equal(ElementErrorBoundaryEvent, errorBoundaryEvent.Type)

	assert.Len(errorBoundaryEvent.Incoming, 0)
	assert.Len(errorBoundaryEvent.Outgoing, 1)

	boundaryEvent := errorBoundaryEvent.Model.(BoundaryEvent)
	require.NotNil(boundaryEvent.AttachedTo)
	assert.Equal("serviceTask", boundaryEvent.AttachedTo.Id)
	assert.Equal(ElementServiceTask, boundaryEvent.AttachedTo.Type)

	assert.Equal("errorBoundaryEventDefinition", boundaryEvent.EventDefinition.Id)
	require.NotNil(boundaryEvent.EventDefinition.Error)

	bpmnError := boundaryEvent.EventDefinition.Error
	assert.Equal("testError", bpmnError.Id)
	assert.Equal("testErrorName", bpmnError.Name)
	assert.Equal("testErrorCode", bpmnError.Code)
}

func TestEscalationBoundaryEventDefinition(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// when
	model := mustCreateModel(t, "event/escalation-boundary-definition.bpmn")

	// then
	processElement := model.ProcessById("escalationBoundaryDefinitionTest")
	require.NotNil(processElement)

	escalationBoundaryEvent := processElement.ChildById("escalationBoundaryEvent")
	require.NotNil(escalationBoundaryEvent)

	boundaryEvent := escalationBoundaryEvent.Model.(BoundaryEvent)

	assert.Equal("escalationBoundaryEventDefinition", boundaryEvent.EventDefinition.Id)
	require.NotNil(boundaryEvent.EventDefinition.Escalation)

	escalation := boundaryEvent.EventDefinition.Escalation
	assert.Equal("testEscalation", escalation.Id)
	assert.Equal("testEscalationName", escalation.Name)
	assert.Equal("testEscalationCode", escalation.Code)
}

func TestEscalationBoundaryEventNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// when
	model := mustCreateModel(t, "event/escalation-boundary-non-interrupting.bpmn")

	// then
	processElement := model.ProcessById("escalationBoundaryNonInterruptingTest")
	require.NotNil(processElement)

	escalationBoundaryEvent := processElement.ChildById("escalationBoundaryEvent")
	require.NotNil(escalationBoundaryEvent)

	boundaryEvent := escalationBoundaryEvent.Model.(BoundaryEvent)
	assert.False(boundaryEvent.CancelActivity)
}

func TestTimerCatchEvent(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// when
	model := mustCreateModel(t, "event/timer-catch.bpmn")

	// then
	processElement := model.ProcessById("timerCatchTest")
	require.NotNil(processElement)

	assert.Len(processElement.Children, 3)

	timerCatchEvent := model.ElementById("timerCatchEvent")
	require.NotNil(timerCatchEvent)

	assert.Equal(ElementTimerCatchEvent, timerCatchEvent.Type)

	assert.Equal(processElement, timerCatchEvent.Parent)

	assert.Len(timerCatchEvent.Incoming, 1)
	assert.Len(timerCatchEvent.Outgoing, 1)
}

func TestTimerStartEvent(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	// when
	model := mustCreateModel(t, "event/timer-start.bpmn")

	// then
	processElement := model.ProcessById("timerStartTest")
	require.NotNil(processElement)

	assert.Len(processElement.Children, 2)

	timerStartEvent := model.ElementById("timerStartEvent")
	require.NotNil(timerStartEvent)

	assert.Equal(ElementTimerStartEvent, timerStartEvent.Type)

	assert.Equal(processElement, timerStartEvent.Parent)

	assert.Len(timerStartEvent.Incoming, 0)
	assert.Len(timerStartEvent.Outgoing, 1)
}
