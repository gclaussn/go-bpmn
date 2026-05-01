package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

type collaborationTest struct {
	e engine.Engine
}

func (x collaborationTest) startEnd(t *testing.T) {
	assert := assert.New(t)

	processA := mustCreateProcess(t, x.e, "collaboration.bpmn", "collaborationATest")
	processC := mustCreateProcess(t, x.e, "collaboration.bpmn", "collaborationCTest")

	assert.Equal("collaboration", processA.BpmnCollaborationId)
	assert.Equal("a", processA.BpmnParticipantId)
	assert.Equal("participant a", processA.BpmnParticipantName)

	assert.Equal("collaboration", processC.BpmnCollaborationId)
	assert.Equal("c", processC.BpmnParticipantId)
	assert.Equal("participant c", processC.BpmnParticipantName)

	piAssertA := mustCreateProcessInstance(t, x.e, processA)
	piAssertA.IsCompleted()

	elementInstancesA := piAssertA.ElementInstances()
	assert.Equal("collaborationATest", elementInstancesA[0].BpmnElementId)
	assert.Equal("startEventA", elementInstancesA[1].BpmnElementId)
	assert.Equal("endEventA", elementInstancesA[2].BpmnElementId)

	piAssertC := mustCreateProcessInstance(t, x.e, processC)
	piAssertC.IsCompleted()

	elementInstancesC := piAssertC.ElementInstances()
	assert.Equal("collaborationCTest", elementInstancesC[0].BpmnElementId)
	assert.Equal("startEventC", elementInstancesC[1].BpmnElementId)
	assert.Equal("endEventC", elementInstancesC[2].BpmnElementId)
}
