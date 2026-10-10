package test

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type messageEventTest struct {
	e engine.Engine
}

func (x messageEventTest) boundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/message-boundary.bpmn", "messageBoundaryTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.HasState(engine.InstanceCreated)

	messageBoundaryEvent := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "ck",
			MessageName:           t.Name(),
		},
	})

	serviceTask.HasJob(engine.JobExecute)

	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	messageBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	messageBoundaryEvent.ExecuteTask()

	psAssert.HasPassed("messageBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	serviceTask.IsTerminated()
	messageBoundaryEvent.IsCompleted()

	// when job of terminated element instance is completed
	serviceTaskJob := serviceTask.CompleteJob()

	// then work is canceled
	assert.True(serviceTaskJob.IsCompleted())
	assert.Equal(engine.WorkCanceled, serviceTaskJob.State)
}

func (x messageEventTest) boundaryMessageSentBefore(t *testing.T) {
	require := require.New(t)

	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		ExpirationTimer: &engine.Timer{
			TimeDuration: engine.ISO8601Duration("PT1H"),
		},
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	process := mustCreateProcess(t, x.e, "event/message-boundary.bpmn", "messageBoundaryTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.HasState(engine.InstanceCreated)

	messageBoundaryEvent := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "ck",
			MessageName:           t.Name(),
		},
	})

	messageBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	messageBoundaryEvent.ExecuteTask()

	psAssert.HasPassed("messageBoundaryEvent")
	psAssert.HasPassed("endEventB")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	serviceTask.IsTerminated()
	messageBoundaryEvent.IsCompleted()

	require.Len(piAssert.Jobs(), 1)
}

func (x messageEventTest) boundaryNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process := mustCreateProcess(t, x.e, "event/message-boundary-non-interrupting.bpmn", "messageBoundaryNonInterruptingTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.HasState(engine.InstanceCreated)

	messageBoundaryEvent1 := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent1.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "ck",
			MessageName:           t.Name(),
		},
	})

	serviceTask.HasState(engine.InstanceStarted)
	serviceTask.HasJob(engine.JobExecute)

	message1, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	assert.True(message1.IsCorrelated)

	messageBoundaryEvent1.HasTask(engine.TaskTriggerEvent)
	messageBoundaryEvent1.ExecuteTask()

	message2, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	assert.True(message2.IsCorrelated)

	messageBoundaryEvent2 := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent2.HasTask(engine.TaskTriggerEvent)
	messageBoundaryEvent2.ExecuteTask()

	serviceTask.CompleteJob()

	psAssert.HasPassed("serviceTask")
	psAssert.HasPassed("endEventA")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 9)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)  // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // messageBoundaryEvent #1
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)  // messageBoundaryEvent #2
	assert.Equal(engine.InstanceTerminated, elementInstances[6].State) // messageBoundaryEvent #3

	require.Len(piAssert.Jobs(), 2)
}

func (x messageEventTest) boundaryNonInterruptingMessageSentBefore(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		ExpirationTimer: &engine.Timer{
			TimeDuration: engine.ISO8601Duration("PT1H"),
		},
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	process := mustCreateProcess(t, x.e, "event/message-boundary-non-interrupting.bpmn", "messageBoundaryNonInterruptingTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")
	serviceTask.HasState(engine.InstanceCreated)

	messageBoundaryEvent := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "ck",
			MessageName:           t.Name(),
		},
	})

	serviceTask.HasState(engine.InstanceStarted)
	serviceTask.HasJob(engine.JobExecute)

	messageBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	messageBoundaryEvent.ExecuteTask()

	serviceTask.CompleteJob()

	psAssert.HasPassed("serviceTask")
	psAssert.HasPassed("endEventA")

	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)  // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // messageBoundaryEvent #1
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State) // messageBoundaryEvent #2

	require.Len(piAssert.Jobs(), 2)
}

func (x messageEventTest) catch(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "event/message-catch.bpmn", "messageCatchTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process, engine.CreateProcessInstanceCmd{
		Variables: []engine.ProcessVariable{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: &engine.Data{Encoding: "encoding-b", Value: "value-b"}},
		},
	})

	messageCatchEvent := psAssert.IsWaitingAt("messageCatchEvent")
	messageCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "ck",
			MessageName:           t.Name(),
		},
	})

	// when message sent
	message, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
		Variables: []engine.ProcessVariable{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: nil},
			{Name: "c", Data: nil},
		},
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// then
	assert.Equal(engine.Message{
		Id: message.Id,

		CorrelationKey: "ck",
		CreatedAt:      message.CreatedAt,
		CreatedBy:      testWorkerId,
		ExpiresAt:      time.Time{},
		IsCorrelated:   true,
		Name:           t.Name(),
		UniqueKey:      "",
	}, message)

	// when
	messageCatchEvent.HasTask(engine.TaskTriggerEvent)
	messageCatchEvent.ExecuteTask()

	// then
	piAssert.HasVariable("a")
	piAssert.HasNoVariable("b")
	piAssert.HasNoVariable("c")

	piAssert.IsCompleted()

	messages, err := x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Id: message.Id})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Len(messages, 1)
	assert.NotZero(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
}

func (x messageEventTest) catchDefinition(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "event/message-catch-definition.bpmn", "messageCatchDefinitionTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	messageCatchEvent := psAssert.IsWaitingAt("messageCatchEvent")
	messageCatchEvent.HasJob(engine.JobSetMessageCorrelationKey)
	messageCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "ck",
		},
	})

	// when message sent
	message, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           "catchMessageName",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// then
	assert.Equal(engine.Message{
		Id: message.Id,

		CorrelationKey: "ck",
		CreatedAt:      message.CreatedAt,
		CreatedBy:      testWorkerId,
		ExpiresAt:      time.Time{},
		IsCorrelated:   true,
		Name:           "catchMessageName",
		UniqueKey:      "",
	}, message)

	// when
	messageCatchEvent.HasTask(engine.TaskTriggerEvent)
	messageCatchEvent.ExecuteTask()

	// then
	piAssert.IsCompleted()

	messages, err := x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Id: message.Id})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Len(messages, 1)
	assert.NotZero(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
}

func (x messageEventTest) catchMessageSentBefore(t *testing.T) {
	assert := assert.New(t)

	// given
	message1, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		ExpirationTimer: &engine.Timer{
			TimeDuration: engine.ISO8601Duration("PT1H"),
		},
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	_, err = x.e.SendMessage(context.Background(), engine.SendMessageCmd{ // same as message 1, but expired
		CorrelationKey: "ck",
		Name:           t.Name(),
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	_, err = x.e.SendMessage(context.Background(), engine.SendMessageCmd{ // same as message 1
		CorrelationKey: "ck",
		ExpirationTimer: &engine.Timer{
			TimeDuration: engine.ISO8601Duration("PT1H"),
		},
		Name:     t.Name(),
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	process := mustCreateProcess(t, x.e, "event/message-catch.bpmn", "messageCatchTest")

	_, psAssert1 := mustCreateProcessInstance(t, x.e, process)
	_, psAssert2 := mustCreateProcessInstance(t, x.e, process)
	_, psAssert3 := mustCreateProcessInstance(t, x.e, process)

	// when correlated
	messageCatchEvent1 := psAssert1.IsWaitingAt("messageCatchEvent")
	messageCatchEvent1.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: message1.CorrelationKey,
			MessageName:           message1.Name,
		},
	})

	// then
	messageCatchEvent1.HasTask(engine.TaskTriggerEvent)

	messages, err := x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Name: message1.Name})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Len(messages, 3)
	assert.Zero(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
	assert.NotZero(messages[1].ExpiresAt)
	assert.False(messages[1].IsCorrelated)
	assert.NotZero(messages[2].ExpiresAt)
	assert.False(messages[2].IsCorrelated)

	// when not correlated
	messageCatchEvent2 := psAssert2.IsWaitingAt("messageCatchEvent")
	messageCatchEvent2.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: message1.CorrelationKey + "*",
			MessageName:           message1.Name,
		},
	})

	// then
	messages, err = x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Name: message1.Name})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Len(messages, 3)
	assert.Zero(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
	assert.NotZero(messages[1].ExpiresAt)
	assert.False(messages[1].IsCorrelated)
	assert.NotZero(messages[2].ExpiresAt)
	assert.False(messages[2].IsCorrelated)

	// when correlated
	messageCatchEvent3 := psAssert3.IsWaitingAt("messageCatchEvent")
	messageCatchEvent3.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: message1.CorrelationKey,
			MessageName:           message1.Name,
		},
	})

	// then
	messageCatchEvent3.HasTask(engine.TaskTriggerEvent)

	messages, err = x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Name: message1.Name})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Len(messages, 3)
	assert.Zero(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
	assert.NotZero(messages[1].ExpiresAt)
	assert.False(messages[1].IsCorrelated)
	assert.Zero(messages[2].ExpiresAt)
	assert.True(messages[2].IsCorrelated)
}

func (x messageEventTest) end(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/message-end.bpmn", "messageEndTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	messageEndEvent := psAssert.IsWaitingAt("messageEndEvent")
	messageEndEvent.HasJob(engine.JobExecute)
	messageEndEvent.CompleteJob()

	piAssert.IsCompleted()
}

func (x messageEventTest) start(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "event/message-start.bpmn", "messageStartTest", engine.CreateProcessCmd{
		Messages: []engine.MessageDefinition{
			{BpmnElementId: "messageStartEvent", MessageName: t.Name()},
		},
	})

	piAssert1, _ := engine.AssertMessageStart(t, x.e, process, engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
		Variables: []engine.ProcessVariable{
			{Name: "a", Data: &engine.Data{Encoding: "encoding-a", Value: "value-a"}},
			{Name: "b", Data: &engine.Data{Encoding: "encoding-b", Value: "value-b"}},
			{Name: "c", Data: nil},
		},
	})

	piAssert1.HasVariable("a")
	piAssert1.HasVariable("b")
	piAssert1.HasNoVariable("c")

	piAssert1.IsCompleted()

	piAssert2, _ := engine.AssertMessageStart(t, x.e, process, engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
	})

	piAssert2.IsCompleted()

	assert.NotEqual(piAssert1.ProcessInstance().String(), piAssert2.ProcessInstance().String())

	messages, err := x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Name: t.Name()})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Len(messages, 2)
	assert.NotZero(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
	assert.NotZero(messages[1].ExpiresAt)
	assert.True(messages[1].IsCorrelated)
}

func (x messageEventTest) startSingleton(t *testing.T) {
	assert := assert.New(t)

	// given
	process := mustCreateProcess(t, x.e, "event/message-start.v2.bpmn", "messageStartTest", engine.CreateProcessCmd{
		Messages: []engine.MessageDefinition{
			{BpmnElementId: "messageStartEvent", MessageName: t.Name()},
		},
	})

	// when
	message, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           t.Name(),
		UniqueKey:      "uk",
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// then
	assert.Zero(message.ExpiresAt)
	assert.True(message.IsCorrelated)

	// when
	completedTasks, failedTasks, err := x.e.ExecuteTasks(context.Background(), engine.ExecuteTasksCmd{
		ProcessId: process.Id,
		Type:      engine.TaskTriggerEvent,
	})
	if err != nil {
		t.Fatalf("failed to execute task: %v", err)
	}
	if len(completedTasks) == 0 || len(failedTasks) != 0 {
		t.Fatal("trigger event task failed")
	}

	// then
	messages, err := x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Id: message.Id})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Zero(messages[0].ExpiresAt)

	// when
	processInstances, err := x.e.CreateQuery().QueryProcessInstances(context.Background(), engine.ProcessInstanceCriteria{
		Partition: completedTasks[0].Partition,
		Id:        completedTasks[0].ProcessInstanceId,
	})
	if err != nil {
		t.Fatalf("failed to query process instances: %v", err)
	}

	assert.Equal("ck", processInstances[0].CorrelationKey)

	piAssert, psAssert := engine.Assert(t, x.e, processInstances[0])
	psAssert.IsWaitingAt("serviceTask").CompleteJob()

	// then
	piAssert.IsCompleted()

	messages, err = x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Id: message.Id})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.NotZero(messages[0].ExpiresAt)
}

func (x messageEventTest) startDefinition(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/message-start-definition.bpmn", "messageStartDefinitionTest")

	piAssert, _ := engine.AssertMessageStart(t, x.e, process, engine.SendMessageCmd{
		CorrelationKey: "ck",
		Name:           "startMessageName",
	})

	piAssert.IsCompleted()
}

func (x messageEventTest) throw(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/message-throw.bpmn", "messageThrowTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	messageThrowEvent := psAssert.IsWaitingAt("messageThrowEvent")
	messageThrowEvent.HasJob(engine.JobExecute)
	messageThrowEvent.CompleteJob()

	piAssert.IsCompleted()
}

func (x messageEventTest) subscriptionCancelation(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "event/message-subscription-cancelation.bpmn", "messageSubscriptionCancelationTest", engine.CreateProcessCmd{
			Messages: []engine.MessageDefinition{
				{BpmnElementId: "messageBoundaryEvent", MessageName: t.Name() + "1"},
				{BpmnElementId: "messageCatchEvent", MessageName: t.Name() + "2"},
				{BpmnElementId: "subProcessMessageCatchEvent", MessageName: t.Name() + "3"},
			},
		}),
		mustCreateProcess(t, x.e, "event/message-catch.bpmn", "messageCatchTest", engine.CreateProcessCmd{
			Messages: []engine.MessageDefinition{
				{BpmnElementId: "messageCatchEvent", MessageName: t.Name() + "4"},
			},
		})

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	messageBoundaryEvent := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "1",
		},
	})

	messageCatchEvent := psAssert.IsWaitingAt("messageCatchEvent")
	messageCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "2",
		},
	})

	subProcessMessageCatchEvent := psAssert.IsWaitingAt("subProcess").IsWaitingAt("subProcessMessageCatchEvent")
	subProcessMessageCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "3",
		},
	})

	callActivity := psAssert.IsWaitingAt("subProcess").IsWaitingAt("callActivity")
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	child, childScope, ok := piAssert.Child()
	require.True(ok)

	subMessageCatchEvent := childScope.IsWaitingAt("messageCatchEvent")
	subMessageCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "4",
		},
	})

	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "1",
		Name:           t.Name() + "1",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// when messageBoundaryEvent is triggered
	messageBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	messageBoundaryEvent.ExecuteTask()

	// then subProcess is terminated
	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 11)
	assert.Equal("subProcess", elementInstances[4].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State)
	assert.Equal("messageBoundaryEvent", elementInstances[5].BpmnElementId)
	assert.Equal(engine.InstanceCompleted, elementInstances[5].State)
	assert.Equal("callActivity", elementInstances[8].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[8].State)
	assert.Equal("subProcessMessageCatchEvent", elementInstances[9].BpmnElementId)
	assert.Equal(engine.InstanceTerminated, elementInstances[9].State)

	// then message subscription of subProcessMessageCatchEvent is canceled
	processInstance := piAssert.ProcessInstance()

	messageSubscriptions, err := x.e.CreateQuery().QueryMessageSubscriptions(context.Background(), engine.MessageSubscriptionCriteria{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
	})
	if err != nil {
		t.Fatalf("failed to query message subscriptions: %v", err)
	}

	require.Len(messageSubscriptions, 1)
	assert.Equal(elementInstances[3].Id, messageSubscriptions[0].ElementInstanceId)
	assert.Equal("messageCatchEvent", messageSubscriptions[0].BpmnElementId)
	assert.Equal(t.Name()+"2", messageSubscriptions[0].Name)

	// when sub process instance is terminated
	subTasks := child.ExecuteTasks()

	// then
	require.Len(subTasks, 1)
	assert.Equal(engine.TaskTerminateProcessInstance, subTasks[0].Type)

	// then message subscription of messageCatchEvent is canceled
	subProcessInstance := child.ProcessInstance()

	messageSubscriptions, err = x.e.CreateQuery().QueryMessageSubscriptions(context.Background(), engine.MessageSubscriptionCriteria{
		Partition:         subProcessInstance.Partition,
		ProcessInstanceId: subProcessInstance.Id,
	})
	if err != nil {
		t.Fatalf("failed to query message subscriptions: %v", err)
	}

	require.Empty(messageSubscriptions)
}

func (x messageEventTest) triggerEventTaskCancelation(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	process, subProcess :=
		mustCreateProcess(t, x.e, "call-activity/message-boundary.bpmn", "callActivityMessageBoundaryTest"),
		mustCreateProcess(t, x.e, "event/message-catch.bpmn", "messageCatchTest")

	piAssert, psAssert := mustCreateProcessInstance(t, x.e, process)

	messageBoundaryEvent := psAssert.IsWaitingAt("messageBoundaryEvent")
	messageBoundaryEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: t.Name() + "1",
			MessageName:           t.Name() + "1",
		},
	})

	callActivity := psAssert.IsWaitingAt("callActivity")
	callActivity.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	child, childScope, ok := piAssert.Child()
	require.True(ok)

	subMessageCatchEvent := childScope.IsWaitingAt("messageCatchEvent")
	subMessageCatchEvent.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: t.Name() + "2",
			MessageName:           t.Name() + "2",
		},
	})

	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: t.Name() + "1",
		Name:           t.Name() + "1",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	_, err = x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: t.Name() + "2",
		Name:           t.Name() + "2",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// when messageBoundaryEvent is triggered
	messageBoundaryEvent.HasTask(engine.TaskTriggerEvent)
	messageBoundaryEvent.ExecuteTask()

	tasks := child.Tasks()
	require.Len(tasks, 2)

	assert.Equal(engine.TaskTriggerEvent, tasks[0].Type)
	assert.Equal(engine.TaskTerminateProcessInstance, tasks[1].Type)

	// when sub-process instance is terminated
	completedTasks, failedTasks, err := x.e.ExecuteTasks(context.Background(), engine.ExecuteTasksCmd{
		Partition: tasks[1].Partition,
		Id:        tasks[1].Id,
	})
	if err != nil {
		t.Fatalf("failed to execute task: %v", err)
	}

	// then
	require.Len(completedTasks, 1)
	require.Len(failedTasks, 0)

	assert.Equal(engine.WorkDone, completedTasks[0].State)

	// when messageCatchEvent is triggered
	completedTasks, failedTasks, err = x.e.ExecuteTasks(context.Background(), engine.ExecuteTasksCmd{
		Partition: tasks[0].Partition,
		Id:        tasks[0].Id,
	})
	if err != nil {
		t.Fatalf("failed to execute task: %v", err)
	}

	// then
	require.Len(completedTasks, 1)
	require.Len(failedTasks, 0)

	assert.Equal(engine.WorkCanceled, completedTasks[0].State)

	messages, err := x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{
		Name: t.Name() + "2",
	})
	if err != nil {
		t.Fatalf("failed to query message: %v", err)
	}

	require.Len(messages, 1)
	assert.NotZero(messages[0].ExpiresAt)
}
