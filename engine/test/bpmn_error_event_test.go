package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newErrorEventTest(t *testing.T, e engine.Engine) errorEventTest {
	return errorEventTest{
		e: e,

		boundaryEventDefinitionProcess: mustCreateProcess(t, e, "event/error-boundary-event-definition.bpmn", "errorBoundaryEventDefinitionTest"),
		boundaryProcess:                mustCreateProcess(t, e, "event/error-boundary-event.bpmn", "errorBoundaryEventTest"),
		boundaryMultipleProcess:        mustCreateProcess(t, e, "event/error-boundary-multiple-event.bpmn", "errorBoundaryMultipleEventTest"),
	}
}

type errorEventTest struct {
	e engine.Engine

	boundaryEventDefinitionProcess engine.Process
	boundaryProcess                engine.Process
	boundaryMultipleProcess        engine.Process
}

func (x errorEventTest) boundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("errorBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	piAssert.HasPassed("errorBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal(engine.InstanceTerminated, elementInstances[2].State) // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // errorBoundaryEvent

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSetErrorCode, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
}

// boundaryWithCode tests that for an error boundary event with error code, no SET_ERROR_CODE job is created.
func (x errorEventTest) boundaryWithCode(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	bpmnXml := mustReadBpmnFile(t, "event/error-boundary-event.bpmn")

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "errorBoundaryEventTest",
		BpmnXml:       bpmnXml,
		ErrorCodes: map[string]string{
			"errorBoundaryEvent": "TEST_CODE",
		},
		Version:  t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	piAssert.HasPassed("errorBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()

	jobs := piAssert.Jobs()
	require.Len(jobs, 1)

	assert.Equal(engine.JobExecute, jobs[0].Type)
}

// boundaryWithoutCode tests that an error boundary event without error code is found and executed.
func (x errorEventTest) boundaryWithoutCode(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("errorBoundaryEvent")
	piAssert.CompleteJob()

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	piAssert.HasPassed("errorBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()
}

// boundaryTerminated tests that an error boundary event is terminated, if it is not executed.
func (x errorEventTest) boundaryTerminated(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("errorBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob()

	piAssert.HasPassed("serviceTask")
	piAssert.HasPassed("endEventA")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)  // serviceTask
	assert.Equal(engine.InstanceTerminated, elementInstances[3].State) // errorBoundaryEvent
}

func (x errorEventTest) boundaryNotFound(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("errorBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "not-existing",
		},
	})
}

// boundaryMultiple tests if the error boundary event with the concrete error code is executed,
// when there is also an error boundary event with an empty error code.
func (x errorEventTest) boundaryMultiple(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryMultipleProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("errorBoundaryEventA")
	piAssert.CompleteJob()

	piAssert.IsWaitingAt("errorBoundaryEventB")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "TEST_CODE",
		},
	})

	piAssert.HasPassed("errorBoundaryEventB")
	piAssert.HasPassed("endEventC")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 6)

	assert.Equal(engine.InstanceTerminated, elementInstances[2].State) // serviceTask
	assert.Equal(engine.InstanceTerminated, elementInstances[3].State) // errorBoundaryEventA
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)  // errorBoundaryEventB

	jobs := piAssert.Jobs()
	require.Len(jobs, 3)

	assert.Equal(engine.JobSetErrorCode, jobs[0].Type)
	assert.Equal(engine.JobSetErrorCode, jobs[1].Type)
	assert.Equal(engine.JobExecute, jobs[2].Type)
}

// boundaryWithEventDefinition tests that for an error boundary event with event definition, no SET_ERROR_CODE job is created.
func (x errorEventTest) boundaryWithEventDefinition(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryEventDefinitionProcess)

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "testErrorCode",
		},
	})

	piAssert.HasPassed("errorBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()
}
