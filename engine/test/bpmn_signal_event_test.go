package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
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
	assert := assert.New(t)

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

	signalEvent, err := x.e.SendSignal(engine.SendSignalCmd{
		Name: "start-signal",
		Variables: map[string]*engine.Data{
			"a": {Encoding: "encoding-a", Value: "value-a"},
			"b": {Encoding: "encoding-b", Value: "value-b"},
			"c": nil,
		},
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	assert.Equal(engine.SignalEvent{
		Partition: signalEvent.Partition,
		Id:        signalEvent.Id,

		CreatedAt:       signalEvent.CreatedAt,
		CreatedBy:       testWorkerId,
		Name:            "start-signal",
		SubscriberCount: 1,
	}, signalEvent)

	results, err := x.e.Query(engine.ElementCriteria{ProcessId: process.Id})
	if err != nil {
		t.Fatalf("failed to query elements: %v", err)
	}

	var element engine.Element
	for _, result := range results {
		if result.(engine.Element).BpmnElementName == "signalStartEvent" {
			element = result.(engine.Element)
			break
		}
	}

	completedTasks, failedTasks, err := x.e.ExecuteTasks(engine.ExecuteTasksCmd{
		ElementId: element.Id,
		ProcessId: process.Id,

		Limit: 1,
	})
	if err != nil {
		t.Fatalf("failed to execute trigger event task: %v", err)
	}

	if len(failedTasks) != 0 || len(completedTasks) == 0 {
		t.Fatal("trigger event task failed")
	}

	results, err = x.e.Query(engine.ProcessInstanceCriteria{ProcessId: process.Id})
	if err != nil {
		t.Fatalf("failed to query process instances: %v", err)
	}

	var processInstance engine.ProcessInstance
	for _, result := range results {
		if result.(engine.ProcessInstance).CreatedAt == *completedTasks[0].CompletedAt {
			processInstance = result.(engine.ProcessInstance)
			break
		}
	}

	piAssert := engine.Assert(t, x.e, processInstance)
	piAssert.IsCompleted()
	piAssert.HasProcessVariable("a")
	piAssert.HasProcessVariable("b")
	piAssert.HasNoProcessVariable("c")
}
