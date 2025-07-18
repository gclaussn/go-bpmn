package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
)

func newSignalEventTest(e engine.Engine) signalEventTest {
	return signalEventTest{
		e: e,
	}
}

type signalEventTest struct {
	e engine.Engine
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
