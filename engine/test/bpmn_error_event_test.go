package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type errorEventTest struct {
	e engine.Engine
}

func (x errorEventTest) boundary(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "event/error-boundary.bpmn", "errorBoundaryTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.HasState(engine.InstanceCreated)

	errorBoundaryEvent := psAssert.IsWaitingAt("errorBoundaryEvent")
	errorBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	psAssert.HasPassed("errorBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	serviceTask.IsTerminated()
	errorBoundaryEvent.IsCompleted()
}

// boundaryWithCode tests that for an error boundary event with error code, no SET_ERROR_CODE job is created.
func (x errorEventTest) boundaryWithCode(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "event/error-boundary.bpmn", "errorBoundaryTest", engine.CreateProcessCmd{
		Errors: []engine.ErrorDefinition{
			{BpmnElementId: "errorBoundaryEvent", ErrorCode: "TEST_CODE"},
		},
	})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	psAssert.HasPassed("errorBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()

	require.Len(piAssert.Jobs(), 1)
}

// boundaryWithoutCode tests that an error boundary event without error code is found and executed.
func (x errorEventTest) boundaryWithoutCode(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/error-boundary.bpmn", "errorBoundaryTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	errorBoundaryEvent := psAssert.IsWaitingAt("errorBoundaryEvent")
	errorBoundaryEvent.HasJob(engine.JobSetErrorCode)
	errorBoundaryEvent.CompleteJob()

	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	psAssert.HasPassed("errorBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()
}

// boundaryTerminated tests that an error boundary event is terminated, if it is not executed.
func (x errorEventTest) boundaryTerminated(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "event/error-boundary.bpmn", "errorBoundaryTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	errorBoundaryEvent := psAssert.IsWaitingAt("errorBoundaryEvent")
	errorBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	serviceTask.CompleteJob()

	psAssert.HasPassed("serviceTask")
	psAssert.HasPassed("endEventA")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	serviceTask.IsCompleted()
	errorBoundaryEvent.IsTerminated()
}

func (x errorEventTest) boundaryNotFound(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/error-boundary.bpmn", "errorBoundaryTest")

	_, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	errorBoundaryEvent := psAssert.IsWaitingAt("errorBoundaryEvent")
	errorBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	serviceTask.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "not-existing",
		},
	})
}

// boundaryMultiple tests if the error boundary event with the concrete error code is executed,
// when there is also an error boundary event with an empty error code.
func (x errorEventTest) boundaryMultiple(t *testing.T) {
	require := require.New(t)

	process := mustCreateProcess(t, x.e, "event/error-boundary-multiple.bpmn", "errorBoundaryMultipleTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	errorBoundaryEventA := psAssert.IsWaitingAt("errorBoundaryEventA")
	errorBoundaryEventA.HasJob(engine.JobSetErrorCode)
	errorBoundaryEventA.CompleteJob()

	errorBoundaryEventB := psAssert.IsWaitingAt("errorBoundaryEventB")
	errorBoundaryEventB.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	psAssert.HasPassed("errorBoundaryEventB")
	psAssert.HasPassed("endEventC")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 6)

	serviceTask.IsTerminated()
	errorBoundaryEventA.IsTerminated()
	errorBoundaryEventB.IsCompleted()
}

// boundaryWithEventDefinition tests that for an error boundary event with event definition, no SET_ERROR_CODE job is created.
func (x errorEventTest) boundaryWithEventDefinition(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/error-boundary-definition.bpmn", "errorBoundaryDefinitionTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "testErrorCode",
		},
	})

	psAssert.HasPassed("errorBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()
}

// end tests that an error end event terminates its scope
func (x errorEventTest) end(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/error-end.bpmn", "errorEndTest", engine.CreateProcessCmd{
		Errors: []engine.ErrorDefinition{
			{BpmnElementId: "errorEndEvent", ErrorCode: "end"},
			{BpmnElementId: "errorBoundaryEvent", ErrorCode: ""},
		},
	})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	errorEndEvent := psAssert.IsWaitingAt("subProcess").IsWaitingAt("errorEndEvent")

	errorEndEvent.HasTask(engine.TaskTriggerEvent)
	errorEndEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal("subProcess", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[2].State)
	assert.Equal("errorBoundaryEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("errorEndEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
	assert.Equal("errorEnd", elementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[6].State)
}

// endNone tests that an error end event behaves like a none end event, if no error boundary event is found
func (x errorEventTest) endNone(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/error-end.bpmn", "errorEndTest", engine.CreateProcessCmd{
		Errors: []engine.ErrorDefinition{
			{BpmnElementId: "errorEndEvent", ErrorCode: "end"},
			{BpmnElementId: "errorBoundaryEvent", ErrorCode: "x"},
		},
	})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	errorEndEvent := psAssert.IsWaitingAt("subProcess").IsWaitingAt("errorEndEvent")
	errorEndEvent.HasTask(engine.TaskTriggerEvent)
	errorEndEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal("subProcess", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)
	assert.Equal("errorBoundaryEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[3].State)
	assert.Equal("errorEndEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
	assert.Equal("endEvent", elementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[6].State)
}
