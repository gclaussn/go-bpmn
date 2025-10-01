package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func newSignalEventTest(t *testing.T, e engine.Engine) signalEventTest {
	return signalEventTest{
		e: e,

		catchTest: mustCreateProcess(t, e, "event/signal-catch.bpmn", "signalCatchTest"),
	}
}

type signalEventTest struct {
	e engine.Engine

	catchTest engine.Process
}

func (x signalEventTest) catch(t *testing.T) {
	assert := assert.New(t)

	processInstance, err := x.e.CreateProcessInstance(context.Background(), engine.CreateProcessInstanceCmd{
		BpmnProcessId: x.catchTest.BpmnProcessId,
		Variables: map[string]*engine.Data{
			"a": {Encoding: "encoding-a", Value: "value-a"},
			"b": {Encoding: "encoding-b", Value: "value-b"},
		},
		Version:  x.catchTest.Version,
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
		Variables: map[string]*engine.Data{
			"a": {Encoding: "encoding-a", Value: "value-a"},
			"b": nil,
			"c": nil,
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

func (x signalEventTest) start(t *testing.T) {
	bpmnXml := mustReadBpmnFile(t, "event/signal-start.bpmn")

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "signalStartTest",
		BpmnXml:       bpmnXml,
		SignalNames: map[string]string{
			"signalStartEvent": "start-signal",
		},
		Version:  "1",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	piAssert1 := engine.AssertSignalStart(t, x.e, process.Id, engine.SendSignalCmd{
		Name: "start-signal",
		Variables: map[string]*engine.Data{
			"a": {Encoding: "encoding-a", Value: "value-a"},
			"b": {Encoding: "encoding-b", Value: "value-b"},
			"c": nil,
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
