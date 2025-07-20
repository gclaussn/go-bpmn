package test

import (
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

	processInstance, err := x.e.CreateProcessInstance(engine.CreateProcessInstanceCmd{
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
	signalEvent, err := x.e.SendSignal(engine.SendSignalCmd{
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
	assert.False(signalEvent.Partition.IsZero())
	assert.NotEmpty(signalEvent.Id)

	assert.NotEmpty(signalEvent.CreatedAt)
	assert.Equal(testWorkerId, signalEvent.CreatedBy)
	assert.Equal("catch-signal", signalEvent.Name)
	assert.Equal(1, signalEvent.SubscriberCount)

	// when signal sent again
	signalEvent, err = x.e.SendSignal(engine.SendSignalCmd{
		Name:     "catch-signal",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	// then
	assert.False(signalEvent.Partition.IsZero())
	assert.NotEmpty(signalEvent.Id)

	assert.NotEmpty(signalEvent.CreatedAt)
	assert.Equal(testWorkerId, signalEvent.CreatedBy)
	assert.Equal("catch-signal", signalEvent.Name)
	assert.Equal(0, signalEvent.SubscriberCount)

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
	piAssert.HasProcessVariable("a")
	piAssert.HasNoProcessVariable("b")
	piAssert.HasNoProcessVariable("c")
}

func (x signalEventTest) start(t *testing.T) {
	bpmnXml := mustReadBpmnFile(t, "event/signal-start.bpmn")

	process, err := x.e.CreateProcess(engine.CreateProcessCmd{
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

	piAssert := engine.AssertSignalStart(t, x.e, process.Id, "signalStartEvent", map[string]*engine.Data{
		"a": {Encoding: "encoding-a", Value: "value-a"},
		"b": {Encoding: "encoding-b", Value: "value-b"},
		"c": nil,
	})

	piAssert.IsCompleted()
	piAssert.HasProcessVariable("a")
	piAssert.HasProcessVariable("b")
	piAssert.HasNoProcessVariable("c")
}
