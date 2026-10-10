package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type escalationEventTest struct {
	e engine.Engine
}

func (x escalationEventTest) boundary(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "event/escalation-boundary.bpmn", "escalationBoundaryTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.HasState(engine.InstanceCreated)

	escalationBoundaryEvent := psAssert.IsWaitingAt("escalationBoundaryEvent")
	escalationBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "TEST_CODE",
		},
	})

	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "TEST_CODE",
		},
	})

	psAssert.HasPassed("escalationBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	assert.Len(elementInstances, 5)

	serviceTask.IsTerminated()
	escalationBoundaryEvent.IsCompleted()
}

// boundaryEventDefinition tests that for an escalation boundary event with event definition, no SET_ESCALATION_CODE job is created.
func (x escalationEventTest) boundaryEventDefinition(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "event/escalation-boundary-definition.bpmn", "escalationBoundaryDefinitionTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "testEscalationCode",
		},
	})

	psAssert.HasPassed("escalationBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()

	assert.Len(piAssert.Jobs(), 1)
}

func (x escalationEventTest) boundaryNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/escalation-boundary-non-interrupting.bpmn", "escalationBoundaryNonInterruptingTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	escalationBoundaryEvent := psAssert.IsWaitingAt("escalationBoundaryEvent")
	escalationBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "TEST_CODE",
		},
	})

	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "TEST_CODE",
		},
	})

	psAssert.HasPassed("endEventB")
	piAssert.HasState(engine.InstanceStarted)

	serviceTask.HasJob(engine.JobExecute)
	serviceTask.CompleteJob()

	psAssert.HasPassed("endEventA")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)  // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // esclationBoundaryEvent #1
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State) // esclationBoundaryEvent #2

	jobs := piAssert.Jobs()
	require.Len(jobs, 3)

	assert.Equal(engine.JobSetEscalationCode, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
	assert.Equal(engine.JobExecute, jobs[2].Type)
}

// end tests that
//   - an escalation throw event never triggers an interrupting boundary event
//   - an escalation end event never triggers a non-interrupting boundary event
//   - an escalation end event terminates the scope
func (x escalationEventTest) end(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/escalation-throw-end.bpmn", "escalationThrowEndTest", engine.CreateProcessCmd{
		Escalations: []engine.EscalationDefinition{
			{BpmnElementId: "escalationThrowEvent", EscalationCode: "throw"},
			{BpmnElementId: "escalationEndEvent", EscalationCode: "end"},
			{BpmnElementId: "escalationBoundaryEventNonInterrupting", EscalationCode: "end"},
			{BpmnElementId: "escalationBoundaryEvent", EscalationCode: ""},
		},
	})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	escalationThrowEvent := psAssert.IsWaitingAt("subProcess").IsWaitingAt("escalationThrowEvent")
	escalationThrowEvent.HasTask(engine.TaskTriggerEvent)
	escalationThrowEvent.ExecuteTask()

	escalationEndEvent := psAssert.IsWaitingAt("subProcess").IsWaitingAt("escalationEndEvent")
	escalationEndEvent.HasTask(engine.TaskTriggerEvent)
	escalationEndEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 9)

	assert.Equal("subProcess", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[3].State)
	assert.Equal("escalationBoundaryEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)
	assert.Equal("escalationEndEvent", elementInstances[7].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[7].State)
	assert.Equal("escalationEnd", elementInstances[8].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[8].State)
}

// throw tests that
//   - an escalation throw event never triggers an interrupting boundary event
//   - an escalation end event never triggers a non-interrupting boundary event
//   - an escalation end event behaves like a none end event, if no boundary event is found
func (x escalationEventTest) throw(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/escalation-throw-end.bpmn", "escalationThrowEndTest", engine.CreateProcessCmd{
		Escalations: []engine.EscalationDefinition{
			{BpmnElementId: "escalationThrowEvent", EscalationCode: "throw"},
			{BpmnElementId: "escalationEndEvent", EscalationCode: "end"},
			{BpmnElementId: "escalationBoundaryEventNonInterrupting", EscalationCode: ""},
			{BpmnElementId: "escalationBoundaryEvent", EscalationCode: "throw"},
		},
	})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	escalationThrowEvent := psAssert.IsWaitingAt("subProcess").IsWaitingAt("escalationThrowEvent")
	escalationThrowEvent.HasTask(engine.TaskTriggerEvent)
	escalationThrowEvent.ExecuteTask()

	escalationEndEvent := psAssert.IsWaitingAt("subProcess").IsWaitingAt("escalationEndEvent")
	escalationEndEvent.HasTask(engine.TaskTriggerEvent)
	escalationEndEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 11)

	assert.Equal("subProcess", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("escalationBoundaryEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State)
	assert.Equal("escalationThrowEvent", elementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[6].State)
	assert.Equal("escalationBoundaryEventNonInterrupting", elementInstances[7].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[7].State)
	assert.Equal("escalationEndNonInterrupting", elementInstances[8].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[8].State)
	assert.Equal("escalationEndEvent", elementInstances[9].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[9].State)
	assert.Equal("endEvent", elementInstances[10].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[10].State)
}
