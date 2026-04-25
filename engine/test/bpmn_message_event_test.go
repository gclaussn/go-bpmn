package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMessageEventTest(t *testing.T, e engine.Engine) messageEventTest {
	return messageEventTest{
		e: e,

		boundaryProcess:                mustCreateProcess(t, e, "event/message-boundary.bpmn", "messageBoundaryTest"),
		boundaryNonInterruptingProcess: mustCreateProcess(t, e, "event/message-boundary-non-interrupting.bpmn", "messageBoundaryNonInterruptingTest"),
		catchProcess:                   mustCreateProcess(t, e, "event/message-catch.bpmn", "messageCatchTest"),
		catchDefinitionProcess:         mustCreateProcess(t, e, "event/message-catch-definition.bpmn", "messageCatchDefinitionTest"),
		startDefinitionProcess:         mustCreateProcess(t, e, "event/message-start-definition.bpmn", "messageStartDefinitionTest"),
	}
}

type messageEventTest struct {
	e engine.Engine

	boundaryProcess                engine.Process
	boundaryNonInterruptingProcess engine.Process
	catchProcess                   engine.Process
	catchDefinitionProcess         engine.Process
	startDefinitionProcess         engine.Process
}

func (x messageEventTest) boundary(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "boundary-message-ck",
			MessageName:           "boundary-message",
		},
	})

	piAssert.IsWaitingAt("serviceTask")
	executeJob := piAssert.Job()

	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "boundary-message-ck",
		Name:           "boundary-message",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.ExecuteTask()
	piAssert.HasPassed("messageBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal(engine.InstanceTerminated, elementInstances[2].State) // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // messageBoundaryEvent

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSubscribeMessage, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)

	// when complete job of terminated element instance
	x.e.LockJobs(context.Background(), engine.LockJobsCmd{
		Partition: executeJob.Partition,
		Id:        executeJob.Id,
	})

	canceledExecuteJob, err := x.e.CompleteJob(context.Background(), engine.CompleteJobCmd{
		Partition: executeJob.Partition,
		Id:        executeJob.Id,
	})
	if err != nil {
		t.Fatalf("failed to complete job: %v", err)
	}

	// then work is canceled
	assert.True(canceledExecuteJob.IsCompleted())
	assert.Equal(engine.WorkCanceled, canceledExecuteJob.State)
}

func (x messageEventTest) boundaryMessageSentBefore(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "boundary-message-ck",
		ExpirationTimer: &engine.Timer{
			TimeDuration: engine.ISO8601Duration("PT1H"),
		},
		Name:     "boundary-message",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "boundary-message-ck",
			MessageName:           "boundary-message",
		},
	})

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.ExecuteTask()
	piAssert.HasPassed("messageBoundaryEvent")
	piAssert.HasPassed("endEventB")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 5)

	assert.Equal(engine.InstanceTerminated, elementInstances[2].State) // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // messageBoundaryEvent

	jobs := piAssert.Jobs()
	require.Len(jobs, 1)

	assert.Equal(engine.JobSubscribeMessage, jobs[0].Type)
}

func (x messageEventTest) boundaryNonInterrupting(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryNonInterruptingProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: t.Name(),
			MessageName:           "boundary-message",
		},
	})

	piAssert.IsWaitingAt("serviceTask")

	message1, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: t.Name(),
		Name:           "boundary-message",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	assert.True(message1.IsCorrelated)

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.ExecuteTask()

	message2, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: t.Name(),
		Name:           "boundary-message",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	assert.True(message2.IsCorrelated)

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.ExecuteTask()

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob()

	piAssert.HasPassed("serviceTask")
	piAssert.HasPassed("endEventA")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 9)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)  // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // messageBoundaryEvent #1
	assert.Equal(engine.InstanceCompleted, elementInstances[4].State)  // messageBoundaryEvent #2
	assert.Equal(engine.InstanceTerminated, elementInstances[6].State) // messageBoundaryEvent #3

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSubscribeMessage, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
}

func (x messageEventTest) boundaryNonInterruptingMessageSentBefore(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	_, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: t.Name(),
		ExpirationTimer: &engine.Timer{
			TimeDuration: engine.ISO8601Duration("PT1H"),
		},
		Name:     "boundary-message",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	piAssert := mustCreateProcessInstance(t, x.e, x.boundaryNonInterruptingProcess)

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: t.Name(),
			MessageName:           "boundary-message",
		},
	})

	piAssert.IsWaitingAt("serviceTask")

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.ExecuteTask()

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob()

	piAssert.HasPassed("serviceTask")
	piAssert.HasPassed("endEventA")
	piAssert.IsCompleted()

	elementInstances := piAssert.ElementInstances()
	require.Len(elementInstances, 7)

	assert.Equal(engine.InstanceCompleted, elementInstances[2].State)  // serviceTask
	assert.Equal(engine.InstanceCompleted, elementInstances[3].State)  // messageBoundaryEvent #1
	assert.Equal(engine.InstanceTerminated, elementInstances[4].State) // messageBoundaryEvent #2

	jobs := piAssert.Jobs()
	require.Len(jobs, 2)

	assert.Equal(engine.JobSubscribeMessage, jobs[0].Type)
	assert.Equal(engine.JobExecute, jobs[1].Type)
}

func (x messageEventTest) catch(t *testing.T) {
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
		Variables: []engine.VariableData{
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

func (x messageEventTest) catchDefinition(t *testing.T) {
	assert := assert.New(t)

	processInstance, err := x.e.CreateProcessInstance(context.Background(), engine.CreateProcessInstanceCmd{
		BpmnProcessId: x.catchDefinitionProcess.BpmnProcessId,
		Version:       x.catchProcess.Version,
		WorkerId:      testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	piAssert := engine.Assert(t, x.e, processInstance)

	piAssert.IsWaitingAt("messageCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "catchMessageCk",
		},
	})

	// when message sent
	message, err := x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "catchMessageCk",
		Name:           "catchMessageName",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// then
	assert.Equal(engine.Message{
		Id: message.Id,

		CorrelationKey: "catchMessageCk",
		CreatedAt:      message.CreatedAt,
		CreatedBy:      testWorkerId,
		ExpiresAt:      nil,
		IsCorrelated:   true,
		Name:           "catchMessageName",
		UniqueKey:      "",
	}, message)

	// when
	piAssert.IsWaitingAt("messageCatchEvent")
	piAssert.ExecuteTask()

	// then
	piAssert.IsCompleted()

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
		BpmnProcessId: x.catchProcess.BpmnProcessId,
		Version:       x.catchProcess.Version,
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

func (x messageEventTest) end(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/message-end.bpmn", "messageEndTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)
	piAssert.IsWaitingAt("messageEndEvent")
	piAssert.CompleteJob()
	piAssert.IsCompleted()
}

func (x messageEventTest) start(t *testing.T) {
	assert := assert.New(t)

	bpmnXml := mustReadBpmnFile(t, "event/message-start.bpmn")

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "messageStartTest",
		BpmnXml:       bpmnXml,
		Messages: []engine.MessageDefinition{
			{BpmnElementId: "messageStartEvent", MessageName: "start-message"},
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
		Messages: []engine.MessageDefinition{
			{BpmnElementId: "messageStartEvent", MessageName: "start-message-singleton"},
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

func (x messageEventTest) startDefinition(t *testing.T) {
	piAssert := engine.AssertMessageStart(t, x.e, x.startDefinitionProcess.Id, engine.SendMessageCmd{
		CorrelationKey: "startMessageCk",
		Name:           "startMessageName",
		WorkerId:       testWorkerId,
	})

	piAssert.IsCompleted()
}

func (x messageEventTest) throw(t *testing.T) {
	process := mustCreateProcess(t, x.e, "event/message-throw.bpmn", "messageThrowTest")

	piAssert := mustCreateProcessInstance(t, x.e, process)
	piAssert.IsWaitingAt("messageThrowEvent")
	piAssert.CompleteJob()
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

	piAssert := mustCreateProcessInstance(t, x.e, process)

	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "1",
		},
	})

	piAssert.IsWaitingAt("messageCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "2",
		},
	})

	piAssert.IsWaitingAt("subProcessMessageCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "3",
		},
	})

	piAssert.IsWaitingAt("callActivity")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId: subProcess.BpmnProcessId,
				Version:       subProcess.Version,
			},
		},
	})

	query := x.e.CreateQuery()

	pi := piAssert.ProcessInstance()

	subProcessInstances, err := query.QueryProcessInstances(context.Background(), engine.ProcessInstanceCriteria{
		Partition: pi.Partition,
		ParentId:  pi.Id,
	})
	if err != nil {
		t.Fatalf("failed to query sub-process instance: %v", err)
	}

	if len(subProcessInstances) == 0 {
		t.Fatal("no sub-process instance found")
	}

	subPiAssert := engine.Assert(t, x.e, subProcessInstances[0])

	subPiAssert.IsWaitingAt("messageCatchEvent")
	subPiAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			MessageCorrelationKey: "4",
		},
	})

	_, err = x.e.SendMessage(context.Background(), engine.SendMessageCmd{
		CorrelationKey: "1",
		Name:           t.Name() + "1",
		WorkerId:       testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// when messageBoundaryEvent is triggered
	piAssert.IsWaitingAt("messageBoundaryEvent")
	piAssert.ExecuteTask()

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
	messageSubscriptions, err := query.QueryMessageSubscriptions(context.Background(), engine.MessageSubscriptionCriteria{
		Partition:         pi.Partition,
		ProcessInstanceId: pi.Id,
	})
	if err != nil {
		t.Fatalf("failed to query message subscriptions: %v", err)
	}

	require.Len(messageSubscriptions, 1)
	assert.Equal(elementInstances[3].Id, messageSubscriptions[0].ElementInstanceId)
	assert.Equal("messageCatchEvent", messageSubscriptions[0].BpmnElementId)
	assert.Equal(t.Name()+"2", messageSubscriptions[0].Name)

	// when sub process instance is terminated
	subTasks := subPiAssert.ExecuteTasks()

	// then
	require.Len(subTasks, 1)
	assert.Equal(engine.TaskTerminateProcessInstance, subTasks[0].Type)

	// then message subscription of messageCatchEvent is canceled
	messageSubscriptions, err = query.QueryMessageSubscriptions(context.Background(), engine.MessageSubscriptionCriteria{
		Partition:         subProcessInstances[0].Partition,
		ProcessInstanceId: subProcessInstances[0].Id,
	})
	if err != nil {
		t.Fatalf("failed to query message subscriptions: %v", err)
	}

	require.Empty(messageSubscriptions)
}
