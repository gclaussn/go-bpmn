package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func newMessageEventTest(t *testing.T, e engine.Engine) messageEventTest {
	return messageEventTest{
		e: e,

		catchTest: mustCreateProcess(t, e, "event/message-catch.bpmn", "messageCatchTest"),
	}
}

type messageEventTest struct {
	e engine.Engine

	catchTest engine.Process
}

func (x messageEventTest) catch(t *testing.T) {
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

	piAssert.IsWaitingAt("messageCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "catch-message-ck",
			MessageName:           "catch-message",
		},
	})

	// when message sent
	message, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "catch-message-ck",
		Name:           "catch-message",
		Variables: map[string]*engine.Data{
			"a": {Encoding: "encoding-a", Value: "value-a"},
			"b": nil,
			"c": nil,
		},
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// then
	assert.Equal(engine.Message{
		Id: message.Id,

		CorrelationKey: "catch-message-ck",
		CreatedAt:      message.CreatedAt,
		CreatedBy:      testWorkerId,
		ExpiresAt:      nil,
		IsCorrelated:   true,
		Name:           "catch-message",
		UniqueKey:      "",
	}, message)

	// when
	piAssert.IsWaitingAt("messageCatchEvent")
	piAssert.ExecuteTask()

	// then
	piAssert.IsCompleted()
	piAssert.HasProcessVariable("a")
	piAssert.HasNoProcessVariable("b")
	piAssert.HasNoProcessVariable("c")

	messages, err := x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Id: message.Id})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Len(messages, 1)
	assert.NotNil(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
}

func (x messageEventTest) catchMessageSentBefore(t *testing.T) {
	assert := assert.New(t)

	// given
	message1, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "catch-message-sent-before-ck",
		ExpirationTimer: &engine.Timer{
			TimeDuration: engine.ISO8601Duration("PT1H"),
		},
		Name:     "catch-message-sent-before",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	_, err = x.e.SendMessage(context.Background(), engine.SendMessageCmd{ // same as message 1, but expired
		CorrelationKey: "catch-message-sent-before-ck",
		Name:           "catch-message-sent-before",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	_, err = x.e.SendMessage(context.Background(), engine.SendMessageCmd{ // same as message 1
		CorrelationKey: "catch-message-sent-before-ck",
		ExpirationTimer: &engine.Timer{
			TimeDuration: engine.ISO8601Duration("PT1H"),
		},
		Name:     "catch-message-sent-before",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	createProcessInstanceCmd := engine.CreateProcessInstanceCmd{
		BpmnProcessId: x.catchTest.BpmnProcessId,
		Version:       x.catchTest.Version,
		WorkerId:      testWorkerId,
	}

	processInstance1, err := x.e.CreateProcessInstance(context.Background(), createProcessInstanceCmd)
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	processInstance2, err := x.e.CreateProcessInstance(context.Background(), createProcessInstanceCmd)
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	processInstance3, err := x.e.CreateProcessInstance(context.Background(), createProcessInstanceCmd)
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert1 := engine.Assert(t, x.e, processInstance1)
	piAssert2 := engine.Assert(t, x.e, processInstance2)
	piAssert3 := engine.Assert(t, x.e, processInstance3)

	// when correlated
	piAssert1.IsWaitingAt("messageCatchEvent")
	piAssert1.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: message1.CorrelationKey,
			MessageName:           message1.Name,
		},
	})

	// then
	piAssert1.IsWaitingAt("messageCatchEvent")

	messages, err := x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Name: message1.Name})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Len(messages, 3)
	assert.Nil(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
	assert.NotNil(messages[1].ExpiresAt)
	assert.False(messages[1].IsCorrelated)
	assert.NotNil(messages[2].ExpiresAt)
	assert.False(messages[2].IsCorrelated)

	// when not correlated
	piAssert2.IsWaitingAt("messageCatchEvent")
	piAssert2.CompleteJob(engine.CompleteJobCmd{
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
	assert.Nil(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
	assert.NotNil(messages[1].ExpiresAt)
	assert.False(messages[1].IsCorrelated)
	assert.NotNil(messages[2].ExpiresAt)
	assert.False(messages[2].IsCorrelated)

	// when correlated
	piAssert3.IsWaitingAt("messageCatchEvent")
	piAssert3.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: message1.CorrelationKey,
			MessageName:           message1.Name,
		},
	})

	// then
	messages, err = x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Name: message1.Name})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Len(messages, 3)
	assert.Nil(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
	assert.NotNil(messages[1].ExpiresAt)
	assert.False(messages[1].IsCorrelated)
	assert.Nil(messages[2].ExpiresAt)
	assert.True(messages[2].IsCorrelated)
}

func (x messageEventTest) start(t *testing.T) {
	assert := assert.New(t)

	bpmnXml := mustReadBpmnFile(t, "event/message-start.bpmn")

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "messageStartTest",
		BpmnXml:       bpmnXml,
		MessageNames: map[string]string{
			"messageStartEvent": "start-message",
		},
		Version:  "1",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	piAssert1 := engine.AssertMessageStart(t, x.e, process.Id, engine.SendMessageCmd{
		CorrelationKey: "start-message-ck",
		Name:           "start-message",
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

	piAssert2 := engine.AssertMessageStart(t, x.e, process.Id, engine.SendMessageCmd{
		CorrelationKey: "start-message-ck",
		Name:           "start-message",
		WorkerId:       testWorkerId,
	})
	piAssert2.IsCompleted()

	assert.NotEqual(piAssert1.ProcessInstance().String(), piAssert2.ProcessInstance().String())

	messages, err := x.e.CreateQuery().QueryMessages(context.Background(), engine.MessageCriteria{Name: "start-message"})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Len(messages, 2)
	assert.NotNil(messages[0].ExpiresAt)
	assert.True(messages[0].IsCorrelated)
	assert.NotNil(messages[1].ExpiresAt)
	assert.True(messages[1].IsCorrelated)
}

func (x messageEventTest) startSingleton(t *testing.T) {
	assert := assert.New(t)

	q := x.e.CreateQuery()

	// given
	bpmnXml := mustReadBpmnFile(t, "event/message-start.v2.bpmn")

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "messageStartTest",
		BpmnXml:       bpmnXml,
		MessageNames: map[string]string{
			"messageStartEvent": "start-message-singleton",
		},
		Version:  "2",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	// when
	message, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "start-message-singleton-ck",
		Name:           "start-message-singleton",
		UniqueKey:      "start-message-singleton-uk",
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// then
	assert.Nil(message.ExpiresAt)
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
	messages, err := q.QueryMessages(context.Background(), engine.MessageCriteria{Id: message.Id})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.Nil(messages[0].ExpiresAt)

	// when
	processInstances, err := x.e.CreateQuery().QueryProcessInstances(context.Background(), engine.ProcessInstanceCriteria{
		Partition: completedTasks[0].Partition,
		Id:        completedTasks[0].ProcessInstanceId,
	})
	if err != nil {
		t.Fatalf("failed to query process instances: %v", err)
	}

	piAssert := engine.Assert(t, x.e, processInstances[0])
	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob()

	// then
	piAssert.IsCompleted()

	messages, err = q.QueryMessages(context.Background(), engine.MessageCriteria{Id: message.Id})
	if err != nil {
		t.Fatalf("failed to query messages: %v", err)
	}

	assert.NotNil(messages[0].ExpiresAt)
}
