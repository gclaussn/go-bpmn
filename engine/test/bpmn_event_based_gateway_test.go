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

	query := x.e.CreateQuery()

	process := mustCreateProcess(t, x.e, "gateway/event-based.bpmn", "eventBasedTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)

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

	piAssert.IsWaitingAt("messageCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageName:           t.Name(),
			MessageCorrelationKey: "ck",
		},
	})

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			SignalName: t.Name(),
		},
	})

	signalSubscriptions, err := query.QuerySignalSubscriptions(context.Background(), engine.SignalSubscriptionCriteria{})
	if err != nil {
		t.Fatalf("failed to query signal subscriptions: %v", err)
	}

	assert.Len(signalSubscriptions, 1)

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
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

	piAssert.IsWaitingAt("messageCatchEvent")
	piAssert.ExecuteTask()

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
	signalSubscriptions, err = query.QuerySignalSubscriptions(context.Background(), engine.SignalSubscriptionCriteria{})
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

	piAssert := mustCreateProcessInstance(t, x.e, process)

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

	piAssert.IsWaitingAt("signalCatchEvent")
	piAssert.ExecuteTask()

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
