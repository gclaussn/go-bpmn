package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type eventBasedGatewayTest struct {
	e engine.Engine
}

func (x eventBasedGatewayTest) gateway(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "gateway/event-based.bpmn", "eventBasedTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 6)

	assert.Equal("eventBasedGateway", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[2].State)
	assert.Equal("messageCatchEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[3].State)
	assert.Equal("signalCatchEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[4].State)
	assert.Equal("timerCatchEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[5].State)

	messageCatchEvent := psAssert.IsWaitingAt("messageCatchEvent")
	messageCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageName:           t.Name(),
			MessageCorrelationKey: "ck",
		},
	})

	signalCatchEvent := psAssert.IsWaitingAt("signalCatchEvent")
	signalCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	signalSubscriptions, err := x.e.CreateQuery().QuerySignalSubscriptions(context.Background(), engine.SignalSubscriptionCriteria{})
	if err != nil {
		t.Fatalf("failed to query signal subscriptions: %v", err)
	}

	assert.Len(signalSubscriptions, 1)

	timerCatchEvent := psAssert.IsWaitingAt("timerCatchEvent")
	timerCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				TimeDuration: "PT1H",
			},
		},
	})

	elementInstances = piAssert.ElementInstances()
	require.Len(elementInstances, 6)

	assert.Equal("eventBasedGateway", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceStarted, elementInstances[2].State)
	assert.Equal("messageCatchEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[3].State)
	assert.Equal("signalCatchEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[4].State)
	assert.Equal("timerCatchEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[5].State)

	_, err = x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	messageCatchEvent.HasTask(engine.TaskTriggerEvent)
	messageCatchEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances = piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal("eventBasedGateway", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)
	assert.Equal("messageCatchEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("signalCatchEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State)
	assert.Equal("timerCatchEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[5].State)
	assert.Equal("messageEnd", elementInstances[6].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[6].State)

	// ensure signal subscription is canceled
	signalSubscriptions, err = x.e.CreateQuery().QuerySignalSubscriptions(context.Background(), engine.SignalSubscriptionCriteria{})
	if err != nil {
		t.Fatalf("failed to query signal subscriptions: %v", err)
	}

	assert.Len(signalSubscriptions, 0)
}

func (x eventBasedGatewayTest) gatewayDefinition(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "gateway/event-based-definition.bpmn", "eventBasedDefinitionTest", engine.CreateProcessCmd{
		Signals: []engine.SignalDefinition{
			{BpmnElementId: "signalCatchEvent", SignalName: t.Name()},
		},
		Timers: []engine.TimerDefinition{
			{BpmnElementId: "timerCatchEvent", Timer: &engine.Timer{TimeDuration: "PT1H"}},
		},
	})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal("eventBasedGateway", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceStarted, elementInstances[2].State)
	assert.Equal("signalCatchEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[3].State)
	assert.Equal("timerCatchEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceCreated, elementInstances[4].State)

	_, err := x.e.SendSignal(context.Background(), engine.SendSignalCmd{
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	signalCatchEvent := psAssert.IsWaitingAt("signalCatchEvent")
	signalCatchEvent.HasTask(engine.TaskTriggerEvent)
	signalCatchEvent.ExecuteTask()

	piAssert.IsCompleted()

	elementInstances = piAssert.ElementInstances()
	require.Len(elementInstances, 6)

	assert.Equal("eventBasedGateway", elementInstances[2].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)
	assert.Equal("signalCatchEvent", elementInstances[3].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)
	assert.Equal("timerCatchEvent", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State)
	assert.Equal("signalEnd", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
}
