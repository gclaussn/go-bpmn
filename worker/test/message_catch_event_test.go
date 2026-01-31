package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type messageCatchEvent struct {
}

func (h messageCatchEvent) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnXml, err := readBpmnFile("event/message-catch.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "messageCatchTest",
		BpmnXml:       bpmnXml,
		Version:       "1",
	}, nil
}

func (h messageCatchEvent) Handle(mux worker.JobMux) error {
	mux.SubscribeMessage("messageCatchEvent", h.subscribeMessage)
	return nil
}

func (h messageCatchEvent) subscribeMessage(jc worker.JobContext) (string, string, error) {
	return "catch-message", "catch-message-ck", nil
}

func TestMessageCatchEventProcess(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	w := mustCreateWorker(t, e)

	messageCatchEventProcess, err := w.Register(messageCatchEvent{})
	if err != nil {
		t.Fatalf("failed to register handler: %v", err)
	}

	processInstance, err := messageCatchEventProcess.CreateProcessInstance(context.Background(), worker.Variables{})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := worker.Assert(t, w, processInstance)

	piAssert.IsWaitingAt("messageCatchEvent")
	piAssert.ExecuteJob()

	if _, err := e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "catch-message-ck",
		Name:           "catch-message",
		WorkerId:       worker.DefaultWorkerId,
	}); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	piAssert.IsWaitingAt("messageCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}
