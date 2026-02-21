package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSignalEventTest(t *testing.T, e engine.Engine) signalEventTest {
	return signalEventTest{
		e: e,

		boundaryProcess:                mustCreateProcess(t, e, "event/signal-boundary.bpmn", "signalBoundaryTest"),
		boundaryNonInterruptingProcess: mustCreateProcess(t, e, "event/signal-boundary-non-interrupting.bpmn", "signalBoundaryNonInterruptingTest"),
		catchProcess:                   mustCreateProcess(t, e, "event/signal-catch.bpmn", "signalCatchTest"),
		catchDefinitionProcess:         mustCreateProcess(t, e, "event/signal-catch-definition.bpmn", "signalCatchDefinitionTest"),
		startDefinitionProcess:         mustCreateProcess(t, e, "event/signal-start-definition.bpmn", "signalStartDefinitionTest"),
	}
}

type signalEventTest struct {
	e engine.Engine

	boundaryProcess                engine.Process
	boundaryNonInterruptingProcess engine.Process
	catchProcess                   engine.Process
	catchDefinitionProcess         engine.Process
	startDefinitionProcess         engine.Process
}

func (x signalEventTest) boundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: "boundary-signal",
		},
	})

	piAssert.IsWaitingAt("serviceTask")

	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     "boundary-signal",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.ExecuteTask()
	piAssert.HasPassed("signalBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal(engine.InstanceTerminated, elementInstances[2].State) // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // signalBoundaryEvent

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSubscribeSignal, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
}

func (x signalEventTest) boundaryNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryNonInterruptingProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: "boundary-signal",
		},
	})

	piAssert.IsWaitingAt("serviceTask")

	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     "boundary-signal",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	piAssert.IsWaitingAt("signalBoundaryEvent")
	piAssert.ExecuteTask()

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob()

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)  // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // signalBoundaryEvent #1
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State) // signalBoundaryEvent #2

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSubscribeSignal, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
}

func (x signalEventTest) catch(t *testing.T) {
	assert := assert.New(t)

	processInstance, err := x.e.CreateProcessInstance(context.Background(), engine.CreateProcessInstanceCmd{
		BpmnProcessId: x.catchProcess.BpmnProcessId,
		Variables: []engine.VariableData{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: &engine.Data{Encoding: "encoding-b", Value: "value-b"}},
		},
		Version:  x.catchProcess.Version,
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := engine.Assert(t, x.e, processInstance)

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: "catch-signal",
		},
	})

	// when signal sent
	signal, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name: "catch-signal",
		Variables: []engine.VariableData{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: nil},
			{Name: "c", Data: nil},
		},
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// then
	assert.NotEmpty(signal.Id)

	assert.NotEmpty(signal.CreatedAt)
	assert.Equal(testWorkerId, signal.CreatedBy)
	assert.Equal("catch-signal", signal.Name)
	assert.Equal(1, signal.SubscriberCount)

	// when signal sent again
	signal, err = x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     "catch-signal",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// then
	assert.NotEmpty(signal.Id)

	assert.NotEmpty(signal.CreatedAt)
	assert.Equal(testWorkerId, signal.CreatedBy)
	assert.Equal("catch-signal", signal.Name)
	assert.Equal(0, signal.SubscriberCount)

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
	piAssert.HasProcessVariable("a")
	piAssert.HasNoProcessVariable("b")
	piAssert.HasNoProcessVariable("c")
}

func (x signalEventTest) catchDefinition(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.catchDefinitionProcess)

	// when signal sent
	signal, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     "testSignalName",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// then
	assert.NotEmpty(signal.Id)

	assert.NotEmpty(signal.CreatedAt)
	assert.Equal(testWorkerId, signal.CreatedBy)
	assert.Equal("testSignalName", signal.Name)
	assert.Equal(2, signal.SubscriberCount) // +1 for signal-start-definition.bpmn

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) end(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-end.bpmn", "signalEndTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("signalEndEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	piAssert.IsWaitingAt("signalEndEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) endDefinition(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-end-definition.bpmn", "signalEndDefinitionTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("signalEndEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) start(t *testing.T) {
	bpmnXml := mustReadBpmnFile(t, "event/signal-start.bpmn")

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "signalStartTest",
		BpmnXml:       bpmnXml,
		Signals: []engine.SignalDefinition{
			{BpmnElementId: "signalStartEvent", SignalName: "start-signal"},
		},
		Version:  "1",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	piAssert1 := engine.AssertSignalStart(t, x.e, process.Id, engine.SendSignalCmd{
		Name: "start-signal",
		Variables: []engine.VariableData{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: &engine.Data{Encoding: "encoding-b", Value: "value-b"}},
			{Name: "c", Data: nil},
		},
		WorkerId: testWorkerId,
	})

	piAssert1.IsCompleted()
	piAssert1.HasProcessVariable("a")
	piAssert1.HasProcessVariable("b")
	piAssert1.HasNoProcessVariable("c")

	piAssert2 := engine.AssertSignalStart(t, x.e, process.Id, engine.SendSignalCmd{
		Name:     "start-signal",
		WorkerId: testWorkerId,
	})
	piAssert2.IsCompleted()

	assert := assert.New(t)
	assert.NotEqual(piAssert1.ProcessInstance().String(), piAssert2.ProcessInstance().String())
}

func (x signalEventTest) startEventDefinition(t *testing.T) {
	piAssert1 := engine.AssertSignalStart(t, x.e, x.startDefinitionProcess.Id, engine.SendSignalCmd{
		Name:     "testSignalName",
		WorkerId: testWorkerId,
	})

	piAssert1.IsCompleted()
}

func (x signalEventTest) throw(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-throw.bpmn", "signalThrowTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("signalThrowEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	piAssert.IsWaitingAt("signalThrowEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}

func (x signalEventTest) throwDefinition(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/signal-throw-definition.bpmn", "signalThrowDefinitionTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("signalThrowEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}
