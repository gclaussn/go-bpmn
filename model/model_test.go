package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestServiceTask(t *testing.T) {
	assert := assert.New(t)

	// when
	model := mustCreateModel(t, "task/service.bpmn")

	// then
	assert.Equal("test", model.Definitions.Id)
	assert.Len(model.Definitions.Processes, 1)

	processElement, err := model.ProcessById("serviceTest")
	assert.NotNil(processElement)
	assert.Nil(err)

	assert.Len(processElement.Elements, 3)
	assert.Equal("serviceTest", processElement.Id)
	assert.Empty(processElement.Incoming)
	assert.Equal("", processElement.Name)
	assert.Empty(processElement.Outgoing)
	assert.Nil(processElement.Parent)
	assert.Len(processElement.SequenceFlows, 2)
	assert.Equal(ElementProcess, processElement.Type)

	process := processElement.Model.(*Process)
	assert.True(process.IsExecutable)

	startEvent := processElement.FindElementById("startEvent")
	assert.NotNil(startEvent)
	assert.Empty(startEvent.Incoming)
	assert.Equal("", startEvent.Name)
	assert.Len(startEvent.Outgoing, 1)
	assert.Equal(processElement, startEvent.Parent)
	assert.Equal(ElementNoneStartEvent, startEvent.Type)

	serviceTask := processElement.FindElementById("serviceTask")
	assert.NotNil(serviceTask)
	assert.Len(serviceTask.Incoming, 1)
	assert.Equal("", serviceTask.Name)
	assert.Len(serviceTask.Outgoing, 1)
	assert.Equal(processElement, serviceTask.Parent)
	assert.Equal(ElementServiceTask, serviceTask.Type)

	endEvent := processElement.FindElementById("endEvent")
	assert.NotNil(endEvent)
	assert.Len(endEvent.Incoming, 1)
	assert.Equal("", endEvent.Name)
	assert.Empty(endEvent.Outgoing)
	assert.Equal(processElement, endEvent.Parent)
	assert.Equal(ElementNoneEndEvent, endEvent.Type)

	assert.Equal(startEvent.Outgoing[0], serviceTask.Incoming[0])
	assert.Equal(serviceTask.Outgoing[0], endEvent.Incoming[0])

	sequenceFlow1 := startEvent.Outgoing[0]
	assert.Equal(startEvent, sequenceFlow1.Source)
	assert.Equal(serviceTask, sequenceFlow1.Target)

	sequenceFlow2 := endEvent.Incoming[0]
	assert.Equal(serviceTask, sequenceFlow2.Source)
	assert.Equal(endEvent, sequenceFlow2.Target)

	noneStartEvents := processElement.FindElementsByType(ElementNoneStartEvent)
	assert.Len(noneStartEvents, 1)
	assert.Equal(noneStartEvents[0], startEvent)

	assert.Nil(processElement.FindElementById("not-existing"))
}

func TestUnknownElement(t *testing.T) {
	assert := assert.New(t)

	// when
	model := mustCreateModel(t, "invalid/element-unknown.bpmn")

	// then
	assert.Equal("test", model.Definitions.Id)
	assert.Len(model.Definitions.Processes, 1)

	processElement, err := model.ProcessById("elementUnknownTest")
	assert.NotNil(processElement)
	assert.Nil(err)

	assert.Len(processElement.Elements, 2)

	assert.NotNil(processElement.SequenceFlows[0].Source)
	assert.Nil(processElement.SequenceFlows[0].Target)
	assert.Nil(processElement.SequenceFlows[1].Source)
	assert.NotNil(processElement.SequenceFlows[1].Target)
}
