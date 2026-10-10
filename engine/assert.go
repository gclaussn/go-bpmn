package engine

import (
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/gclaussn/go-bpmn/model"
)

const testWorkerId = "test-worker"

// Assert asserts a process instance.
// Asserts for the process instance and it's process (root) scope are returned.
func Assert(t *testing.T, e Engine, processInstance ProcessInstance) (ProcessInstanceAssert, ElementInstanceAssert) {
	processInstanceAssert := ProcessInstanceAssert{
		t: t,
		e: e,

		partition:     processInstance.Partition,
		id:            processInstance.Id,
		parentId:      processInstance.ParentId,
		bpmnProcessId: processInstance.BpmnProcessId,
	}

	return processInstanceAssert, processInstanceAssert.ProcessScope()
}

// AssertMessageStart asserts a process instance started by a message start event.
// Asserts for the created process instance and it's process (root) scope are returned.
//
//  1. Sends a message, using cmd.
//  2. Ensures that the sent message is correlated.
//  2. Finds the related message start event.
//  3. Triggers the message start event, which creates the process instance.
func AssertMessageStart(t *testing.T, e Engine, process Process, cmd SendMessageCmd) (ProcessInstanceAssert, ElementInstanceAssert) {
	if cmd.WorkerId == "" {
		cmd.WorkerId = testWorkerId
	}

	message, err := e.SendMessage(context.Background(), cmd)
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	if !message.IsCorrelated {
		t.Fatalf("expected message %s to be correlated", message)
	}

	elements, err := e.CreateQuery().QueryElements(context.Background(), ElementCriteria{
		ProcessId: process.Id,
	})
	if err != nil {
		t.Fatalf("failed to query elements: %v", err)
	}

	var messageEvent Element
	for _, element := range elements {
		if element.EventDefinition == nil {
			continue
		}
		if element.EventDefinition.MessageName == message.Name && !element.EventDefinition.IsSuspended {
			messageEvent = element
			break
		}
	}

	if messageEvent.Id == 0 {
		t.Fatalf("expected to find message event")
	}
	if messageEvent.BpmnElementType != model.ElementMessageStartEvent {
		t.Fatalf("expected message event to be a start event: %+v", messageEvent)
	}

	tasks, err := e.CreateQuery().QueryTasks(context.Background(), TaskCriteria{
		ProcessId: process.Id,
		Type:      TaskTriggerEvent,
	})
	if err != nil {
		t.Fatalf("failed to query tasks: %v", err)
	}

	var triggerEventTask Task
	for _, task := range tasks {
		if !task.IsCompleted() && task.CreatedAt.Equal(message.CreatedAt) {
			triggerEventTask = task
			break
		}
	}
	if triggerEventTask.Id == 0 {
		t.Fatal("expected to find trigger event task")
	}

	completedTasks, _, err := e.ExecuteTasks(context.Background(), ExecuteTasksCmd{
		Partition: triggerEventTask.Partition,
		Id:        triggerEventTask.Id,
	})
	if err != nil {
		t.Fatalf("failed to execute task: %v", err)
	}
	if len(completedTasks) == 0 {
		t.Fatal("expected trigger event task to complete")
	}

	return Assert(t, e, ProcessInstance{
		Partition: completedTasks[0].Partition,
		Id:        completedTasks[0].ProcessInstanceId,

		BpmnProcessId: messageEvent.ParentBpmnElementId,
	})
}

// AssertSignalStart asserts a process instance started by a signal start event.
// Asserts for the created process instance and it's process (root) scope are returned.
//
//  1. Sends a signal, using cmd.
//  2. Ensures that the sent signal has at least one subscriber.
//  2. Finds the related signal start event.
//  3. Triggers the signal start event, which creates the process instance.
func AssertSignalStart(t *testing.T, e Engine, process Process, cmd SendSignalCmd) (ProcessInstanceAssert, ElementInstanceAssert) {
	if cmd.WorkerId == "" {
		cmd.WorkerId = testWorkerId
	}

	signal, err := e.SendSignal(context.Background(), cmd)
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	if signal.SubscriberCount == 0 {
		t.Fatalf("expected signal %s to have at least one subscriber", signal)
	}

	elements, err := e.CreateQuery().QueryElements(context.Background(), ElementCriteria{
		ProcessId: process.Id,
	})
	if err != nil {
		t.Fatalf("failed to query elements: %v", err)
	}

	var signalEvent Element
	for _, element := range elements {
		if element.EventDefinition == nil {
			continue
		}
		if element.EventDefinition.SignalName == cmd.Name && !element.EventDefinition.IsSuspended {
			signalEvent = element
			break
		}
	}

	if signalEvent.Id == 0 {
		t.Fatalf("expected to find signal event")
	}
	if signalEvent.BpmnElementType != model.ElementSignalStartEvent {
		t.Fatalf("expected signal event to be a start event: %+v", signalEvent)
	}

	tasks, err := e.CreateQuery().QueryTasks(context.Background(), TaskCriteria{
		ProcessId: process.Id,
		Type:      TaskTriggerEvent,
	})
	if err != nil {
		t.Fatalf("failed to query tasks: %v", err)
	}

	var triggerEventTask Task
	for _, task := range tasks {
		if !task.IsCompleted() && task.CreatedAt.Equal(signal.CreatedAt) {
			triggerEventTask = task
			break
		}
	}
	if triggerEventTask.Id == 0 {
		t.Fatal("expected to find trigger event task")
	}

	completedTasks, _, err := e.ExecuteTasks(context.Background(), ExecuteTasksCmd{
		Partition: triggerEventTask.Partition,
		Id:        triggerEventTask.Id,
	})
	if err != nil {
		t.Fatalf("failed to execute task: %v", err)
	}
	if len(completedTasks) == 0 {
		t.Fatal("expected trigger event task to complete")
	}

	return Assert(t, e, ProcessInstance{
		Partition: completedTasks[0].Partition,
		Id:        completedTasks[0].ProcessInstanceId,

		BpmnProcessId: signalEvent.ParentBpmnElementId,
	})
}

// AsserTimerStart asserts a process instance started by a timer start event.
// Asserts for the created process instance and it's process (root) scope are returned.
//
// Since a process can have multiple timer start events, the ID of the BPMN start element must be provided.
//
//  1. Finds the related timer start event.
//  2. Increases the engine's time to make trigger event task due.
//  3. Triggers the timer start event, which creates the process instance.
func AsserTimerStart(t *testing.T, e Engine, process Process, startEventId string) (ProcessInstanceAssert, ElementInstanceAssert) {
	elements, err := e.CreateQuery().QueryElements(context.Background(), ElementCriteria{
		ProcessId:     process.Id,
		BpmnElementId: startEventId,
	})
	if err != nil {
		t.Fatalf("failed to query elements: %v", err)
	}

	if len(elements) == 0 {
		t.Fatalf("expected to find timer event")
	}
	if elements[0].BpmnElementType != model.ElementTimerStartEvent {
		t.Fatalf("expected timer event to be a start event: %+v", elements[0])
	}

	tasks, err := e.CreateQuery().QueryTasks(context.Background(), TaskCriteria{
		ProcessId: process.Id,
		ElementId: elements[0].Id,
		Type:      TaskTriggerEvent,
	})
	if err != nil {
		t.Fatalf("failed to query tasks: %v", err)
	}

	var triggerEventTask Task
	for _, task := range tasks {
		if !task.IsCompleted() {
			triggerEventTask = task
			break
		}
	}
	if triggerEventTask.Id == 0 {
		t.Fatal("failed to find trigger event task")
	}

	if _, _, err := e.SetTime(context.Background(), SetTimeCmd{
		Time: triggerEventTask.DueAt,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	completedTasks, _, err := e.ExecuteTasks(context.Background(), ExecuteTasksCmd{
		Partition: triggerEventTask.Partition,
		Id:        triggerEventTask.Id,
	})
	if err != nil {
		t.Fatalf("failed to execute task: %v", err)
	}
	if len(completedTasks) == 0 {
		t.Fatal("expected trigger event task to complete")
	}

	return Assert(t, e, ProcessInstance{
		Partition: completedTasks[0].Partition,
		Id:        completedTasks[0].ProcessInstanceId,

		BpmnProcessId: elements[0].ParentBpmnElementId,
	})
}

// ProcessInstanceAssert is used to assert state and variables of a process instance.
type ProcessInstanceAssert struct {
	t *testing.T
	e Engine

	partition     Partition
	id            int32
	parentId      int32
	bpmnProcessId string
}

// Child asserts the next, not ended child of a process instance.
// Asserts for the child process instance and it's process (root) scope are returned.
//
// If no child process instances exist or all child process instances are ended, false is returned.
func (a ProcessInstanceAssert) Child() (ProcessInstanceAssert, ElementInstanceAssert, bool) {
	processInstances, err := a.e.CreateQuery().QueryProcessInstances(context.Background(), ProcessInstanceCriteria{
		Partition: a.partition,
		ParentId:  a.id,
	})
	if err != nil {
		a.Fatalf("failed to query process instances: %v", err)
	}

	for _, processInstance := range processInstances {
		if !processInstance.IsEnded() {
			childInstanceAssert := ProcessInstanceAssert{
				t: a.t,
				e: a.e,

				partition:     processInstance.Partition,
				id:            processInstance.Id,
				parentId:      processInstance.ParentId,
				bpmnProcessId: processInstance.BpmnProcessId,
			}

			return childInstanceAssert, childInstanceAssert.ProcessScope(), true
		}
	}

	return ProcessInstanceAssert{}, ElementInstanceAssert{}, false
}

// Children returns asserts for all child process instances.
func (a ProcessInstanceAssert) Children() []ProcessInstanceAssert {
	processInstances, err := a.e.CreateQuery().QueryProcessInstances(context.Background(), ProcessInstanceCriteria{
		Partition: a.partition,
		ParentId:  a.id,
	})
	if err != nil {
		a.Fatalf("failed to query process instances: %v", err)
	}

	children := make([]ProcessInstanceAssert, len(processInstances))
	for i, processInstance := range processInstances {
		children[i] = ProcessInstanceAssert{
			t: a.t,
			e: a.e,

			partition:     processInstance.Partition,
			id:            processInstance.Id,
			parentId:      processInstance.ParentId,
			bpmnProcessId: processInstance.BpmnProcessId,
		}
	}

	return children
}

// ElementInstances is used to query element instances of a process instance.
// Optionally, additional criteria can be provided.
func (a ProcessInstanceAssert) ElementInstances(criteria ...ElementInstanceCriteria) []ElementInstance {
	if len(criteria) > 1 {
		a.Fatalf("expected zero or one criteria")
	}

	var c ElementInstanceCriteria
	if len(criteria) != 0 {
		c = criteria[0]
	} else {
		c = ElementInstanceCriteria{}
	}

	c.Partition = a.partition
	c.ProcessInstanceId = a.id

	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), c)
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	return elementInstances
}

// ExecuteTasks executes all due tasks of a process instance.
// Completed tasks are returned.
//
// Fails if at least one task completed with an error, or if there are failed tasks.
func (a ProcessInstanceAssert) ExecuteTasks() []Task {
	completedTasks, failedTasks, err := a.e.ExecuteTasks(context.Background(), ExecuteTasksCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.id,
		Limit:             100,
	})
	if err != nil {
		a.Fatalf("failed to execute tasks: %v", err)
	}

	for i := range completedTasks {
		if completedTasks[i].HasError() {
			a.Fatalf("completed task %s has error: %s", completedTasks[i], completedTasks[i].Error)
		}
	}

	if len(failedTasks) != 0 {
		a.Fatalf("expected no failed tasks, but got %d: %+v", len(failedTasks), failedTasks)
	}

	return completedTasks
}

func (a ProcessInstanceAssert) Fatalf(format string, args ...any) {
	fatalf(a.t, format, args...)
}

// HasNoVariable asserts that no process variable with name exists.
func (a ProcessInstanceAssert) HasNoVariable(name string) {
	variables, err := a.e.GetProcessVariables(context.Background(), GetProcessVariablesCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.id,
		Names:             []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get process variable %s: %v", name, err)
	}

	for _, variable := range variables {
		if variable.Name == name {
			a.Fatalf("expected process instance %s to have no variable %s", a, name)
		}
	}
}

// HasState asserts that a process instance is in state.
func (a ProcessInstanceAssert) HasState(state InstanceState) {
	processInstance := a.ProcessInstance()
	if processInstance.State != state {
		a.Fatalf("expected process instance %s to be %s, but is %s", a, state, processInstance.State)
	}
}

// HasVariable asserts that a process variable with name exists.
func (a ProcessInstanceAssert) HasVariable(name string) {
	variables, err := a.e.GetProcessVariables(context.Background(), GetProcessVariablesCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.id,
		Names:             []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get process variable %s: %v", name, err)
	}

	for _, variable := range variables {
		if variable.Name == name {
			return
		}
	}

	a.Fatalf("expected process instance %s to have variable %s", a, name)
}

// IsCompleted asserts that a process instance is in state [InstanceCompleted].
func (a ProcessInstanceAssert) IsCompleted() {
	a.HasState(InstanceCompleted)
}

// IsTerminated asserts that a process instance is in state [InstanceTerminated].
func (a ProcessInstanceAssert) IsTerminated() {
	a.HasState(InstanceTerminated)
}

// Jobs is used to query jobs of a process instance.
// Optionally, additional criteria can be provided.
func (a ProcessInstanceAssert) Jobs(criteria ...JobCriteria) []Job {
	if len(criteria) > 1 {
		a.Fatalf("expected zero or one criteria")
	}

	var c JobCriteria
	if len(criteria) != 0 {
		c = criteria[0]
	} else {
		c = JobCriteria{}
	}

	c.Partition = a.partition
	c.ProcessInstanceId = a.id

	jobs, err := a.e.CreateQuery().QueryJobs(context.Background(), c)
	if err != nil {
		a.Fatalf("failed to query jobs: %v", err)
	}

	return jobs
}

// Parent asserts the parent of a process instance.
//
// Fails if no parent process instance exists.
func (a ProcessInstanceAssert) Parent() ProcessInstanceAssert {
	if a.parentId == 0 {
		a.Fatalf("expected process instance %s to have a parent", a)
	}

	processInstances, err := a.e.CreateQuery().QueryProcessInstances(context.Background(), ProcessInstanceCriteria{
		Partition: a.partition,
		Id:        a.parentId,
	})
	if err != nil {
		a.Fatalf("failed to query process instance: %v", err)
	}
	if len(processInstances) == 0 {
		a.Fatalf("expected one process instance, but got none")
	}

	return ProcessInstanceAssert{
		t: a.t,
		e: a.e,

		partition:     processInstances[0].Partition,
		id:            processInstances[0].Id,
		parentId:      processInstances[0].ParentId,
		bpmnProcessId: processInstances[0].BpmnProcessId,
	}
}

// ProcessInstance returns the related process instance.
func (a ProcessInstanceAssert) ProcessInstance() ProcessInstance {
	processInstances, err := a.e.CreateQuery().QueryProcessInstances(context.Background(), ProcessInstanceCriteria{
		Partition: a.partition,
		Id:        a.id,
	})
	if err != nil {
		a.Fatalf("failed to query process instance: %v", err)
	}
	if len(processInstances) != 1 {
		a.Fatalf("expected one process instance, but got %d: %+v", len(processInstances), processInstances)
	}
	return processInstances[0]
}

// ProcessScope asserts a process instance's process (root) scope.
func (a ProcessInstanceAssert) ProcessScope() ElementInstanceAssert {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.id,
		BpmnElementId:     a.bpmnProcessId,
	})
	if err != nil {
		a.Fatalf("failed to query element instance: %v", err)
	}
	if len(elementInstances) == 0 {
		a.Fatalf("expected process scope, but got none")
	}

	return ElementInstanceAssert{
		t: a.t,
		e: a.e,

		partition:         elementInstances[0].Partition,
		id:                elementInstances[0].Id,
		parentId:          elementInstances[0].ParentId,
		processInstanceId: elementInstances[0].ProcessInstanceId,
		bpmnElementId:     elementInstances[0].BpmnElementId,
	}
}

func (a ProcessInstanceAssert) String() string {
	return fmt.Sprintf("%s (%s/%d)", a.bpmnProcessId, a.partition, a.id)
}

// Tasks is used to query tasks of a process instance.
// Optionally, additional criteria can be provided.
func (a ProcessInstanceAssert) Tasks(criteria ...TaskCriteria) []Task {
	if len(criteria) > 1 {
		a.Fatalf("expected zero or one criteria")
	}

	var c TaskCriteria
	if len(criteria) != 0 {
		c = criteria[0]
	} else {
		c = TaskCriteria{}
	}

	c.Partition = a.partition
	c.ProcessInstanceId = a.id

	tasks, err := a.e.CreateQuery().QueryTasks(context.Background(), c)
	if err != nil {
		a.Fatalf("failed to query tasks: %v", err)
	}

	return tasks
}

// UserTasks is used to query user tasks of a process instance.
// Optionally, additional criteria can be provided.
func (a ProcessInstanceAssert) UserTasks(criteria ...UserTaskCriteria) []UserTask {
	if len(criteria) > 1 {
		a.Fatalf("expected zero or one criteria")
	}

	var c UserTaskCriteria
	if len(criteria) != 0 {
		c = criteria[0]
	} else {
		c = UserTaskCriteria{}
	}

	c.Partition = a.partition
	c.ProcessInstanceId = a.id

	userTasks, err := a.e.CreateQuery().QueryUserTasks(context.Background(), c)
	if err != nil {
		a.Fatalf("failed to query user tasks: %v", err)
	}

	return userTasks
}

// ElementInstanceAssert is used to assert state and variables of an element instance.
type ElementInstanceAssert struct {
	t *testing.T
	e Engine

	partition         Partition
	id                int32
	parentId          int32
	processInstanceId int32
	bpmnElementId     string
}

// Child asserts the next, not ended child of an element instance.
//
// If no child element instances exist or all child element instances are ended, false is returned.
func (a ElementInstanceAssert) Child() (ElementInstanceAssert, bool) {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.partition,
		ParentId:  a.id,
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	for _, elementInstance := range elementInstances {
		if !elementInstance.IsEnded() {
			return ElementInstanceAssert{
				t: a.t,
				e: a.e,

				partition:         elementInstance.Partition,
				id:                elementInstance.Id,
				parentId:          elementInstance.ParentId,
				processInstanceId: elementInstance.ProcessInstanceId,
				bpmnElementId:     elementInstance.BpmnElementId,
			}, true
		}
	}

	return ElementInstanceAssert{}, false
}

// ChildAt asserts an element instance's next, not ended child instance at a certain BPMN element.
//
// If no such child element instance exists or all child element instances at the BPMN element are ended, false is returned.
func (a ElementInstanceAssert) ChildAt(bpmnElementId string) (ElementInstanceAssert, bool) {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition:     a.partition,
		ParentId:      a.id,
		BpmnElementId: bpmnElementId,
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	for _, elementInstance := range elementInstances {
		if !elementInstance.IsEnded() {
			return ElementInstanceAssert{
				t: a.t,
				e: a.e,

				partition:         elementInstance.Partition,
				id:                elementInstance.Id,
				parentId:          elementInstance.ParentId,
				processInstanceId: elementInstance.ProcessInstanceId,
				bpmnElementId:     elementInstance.BpmnElementId,
			}, true
		}
	}

	return ElementInstanceAssert{}, false
}

// Children returns asserts for all child element instances.
func (a ElementInstanceAssert) Children() []ElementInstanceAssert {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.partition,
		ParentId:  a.id,
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	children := make([]ElementInstanceAssert, len(elementInstances))
	for i, elementInstance := range elementInstances {
		children[i] = ElementInstanceAssert{
			t: a.t,
			e: a.e,

			partition:         elementInstance.Partition,
			id:                elementInstance.Id,
			parentId:          elementInstance.ParentId,
			processInstanceId: elementInstance.ProcessInstanceId,
			bpmnElementId:     elementInstance.BpmnElementId,
		}
	}

	return children
}

// ChildrenAt returns asserts for all child element instances at a certain BPMN element.
func (a ElementInstanceAssert) ChildrenAt(bpmnElementId string) []ElementInstanceAssert {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition:     a.partition,
		ParentId:      a.id,
		BpmnElementId: bpmnElementId,
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	children := make([]ElementInstanceAssert, len(elementInstances))
	for i, elementInstance := range elementInstances {
		children[i] = ElementInstanceAssert{
			t: a.t,
			e: a.e,

			partition:         elementInstance.Partition,
			id:                elementInstance.Id,
			parentId:          elementInstance.ParentId,
			processInstanceId: elementInstance.ProcessInstanceId,
			bpmnElementId:     elementInstance.BpmnElementId,
		}
	}

	return children
}

// CompleteJob completes a due job of an element instance.
// Optionally, cmd can be used to set a [JobCompletion] and variables.
//
// Fails if no job could be found or locked, or the job completed with an error.
func (a ElementInstanceAssert) CompleteJob(cmd ...CompleteJobCmd) Job {
	if len(cmd) > 1 {
		a.Fatalf("expected zero or one cmd")
	}

	job := a.Job()

	lockedJobs, err := a.e.LockJobs(context.Background(), LockJobsCmd{
		Partition: job.Partition,
		Id:        job.Id,
		Limit:     1,
		WorkerId:  testWorkerId,
	})
	if err != nil {
		a.Fatalf("failed to lock job: %v", err)
	}
	if len(lockedJobs) == 0 {
		a.Fatalf("expected one job to lock, but got none")
	}

	var c CompleteJobCmd
	if len(cmd) != 0 {
		c = cmd[0]
	} else {
		c = CompleteJobCmd{}
	}

	c.Partition = lockedJobs[0].Partition
	c.Id = lockedJobs[0].Id
	c.WorkerId = testWorkerId

	completedJob, err := a.e.CompleteJob(context.Background(), c)
	if err != nil {
		a.Fatalf("failed to complete job %s: %v", lockedJobs[0], err)
	}
	if completedJob.HasError() {
		a.Fatalf("expected job %s to complete without an error, but got: %s", completedJob, completedJob.Error)
	}

	return completedJob
}

// CompleteJobWithError completes a due job of an element instance.
// Optionally, cmd can be used to set a [JobCompletion] and variables.
//
// Fails if no job could be found or locked, or the job completed without an error.
func (a ElementInstanceAssert) CompleteJobWithError(cmd ...CompleteJobCmd) Job {
	if len(cmd) > 1 {
		a.Fatalf("expected zero or one cmd")
	}

	job := a.Job()

	lockedJobs, err := a.e.LockJobs(context.Background(), LockJobsCmd{
		Partition: job.Partition,
		Id:        job.Id,
		Limit:     1,
		WorkerId:  testWorkerId,
	})
	if err != nil {
		a.Fatalf("failed to lock job: %v", err)
	}
	if len(lockedJobs) == 0 {
		a.Fatalf("expected one job to lock, but got none")
	}

	var c CompleteJobCmd
	if len(cmd) != 0 {
		c = cmd[0]
	} else {
		c = CompleteJobCmd{}
	}

	c.Partition = lockedJobs[0].Partition
	c.Id = lockedJobs[0].Id
	c.WorkerId = testWorkerId

	completedJob, err := a.e.CompleteJob(context.Background(), c)
	if err != nil {
		a.Fatalf("failed to complete job %s: %v", lockedJobs[0], err)
	}
	if !completedJob.HasError() {
		a.Fatalf("expected job %s to complete with an error", completedJob)
	}

	return completedJob
}

// ElementInstance returns the related element instance.
func (a ElementInstanceAssert) ElementInstance() ElementInstance {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.partition,
		Id:        a.id,
	})
	if err != nil {
		a.Fatalf("failed to query element instance: %v", err)
	}
	if len(elementInstances) != 1 {
		a.Fatalf("expected one element instance, but got %d: %+v", len(elementInstances), elementInstances)
	}

	return elementInstances[0]
}

// ExecuteTask executes a due task of an element instance.
//
// Fails if no task is completed or the task completed with an error.
func (a ElementInstanceAssert) ExecuteTask() Task {
	task := a.Task()

	completedTasks, _, err := a.e.ExecuteTasks(context.Background(), ExecuteTasksCmd{
		Partition: task.Partition,
		Id:        task.Id,
	})
	if err != nil {
		a.Fatalf("failed to execute task: %v", err)
	}
	if len(completedTasks) == 0 {
		a.Fatalf("expected one task to complete, but got none")
	}

	if completedTasks[0].HasError() {
		a.Fatalf("completed task %s has error: %s", completedTasks[0], completedTasks[0].Error)
	}

	return completedTasks[0]
}

func (a ElementInstanceAssert) Fatalf(format string, args ...any) {
	fatalf(a.t, format, args...)
}

// HasJob asserts that an element instance has an active (not completed) job of a certain type.
func (a ElementInstanceAssert) HasJob(jobType JobType) {
	if job := a.Job(); job.Type != jobType {
		a.Fatalf("expected element instance %s to have an active job of type %s, but was %s", a, jobType, job.Type)
	}
}

// HasNoVariable asserts that no element variable with name exists.
func (a ElementInstanceAssert) HasNoVariable(name string) {
	elementInstance := a.ElementInstance()

	variables, err := a.e.GetElementVariables(context.Background(), GetElementVariablesCmd{
		Partition:              elementInstance.Partition,
		ElementInstanceId:      elementInstance.Id,
		ExcludeParentVariables: true,
		Names:                  []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get element variable %s: %v", name, err)
	}
	if len(variables) != 0 {
		a.Fatalf("expected element instance %s to have no variable %s", a, name)
	}
}

// HasPassed asserts that a scope has passed a certain BPMN element.
func (a ElementInstanceAssert) HasPassed(bpmnElementId string) {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.partition,
		ParentId:  a.id,
		States:    []InstanceState{InstanceCompleted},
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	for _, elementInstance := range elementInstances {
		if elementInstance.BpmnElementId == bpmnElementId {
			return
		}
	}

	slices.SortFunc(elementInstances, func(a ElementInstance, b ElementInstance) int {
		if a.EndedAt.IsZero() {
			return -1
		} else if b.EndedAt.IsZero() {
			return 1
		}

		if a.EndedAt.Equal(b.EndedAt) {
			return int(a.Id - b.Id)
		} else if a.EndedAt.Before(b.EndedAt) {
			return -1
		} else {
			return 1
		}
	})

	passed := make([]string, len(elementInstances))
	for i, elementInstance := range elementInstances {
		passed[i] = elementInstance.BpmnElementId
	}

	a.Fatalf("expected scope %s to have passed %s\npassed elements: %s", a, bpmnElementId, strings.Join(passed, ", "))
}

// HasState asserts that an element instance is in state.
func (a ElementInstanceAssert) HasState(state InstanceState) {
	elementInstance := a.ElementInstance()
	if elementInstance.State != state {
		a.Fatalf("expected element instance %s to be %s, but is %s", a, state, elementInstance.State)
	}
}

// HasTask asserts that an element instance has an active (not completed) task of a certain type.
func (a ElementInstanceAssert) HasTask(taskType TaskType) {
	if task := a.Task(); task.Type != taskType {
		a.Fatalf("expected element instance %s to have an active task of type %s, but was %s", a, taskType, task.Type)
	}
}

// HasVariable asserts that an element variable with name exists.
func (a ElementInstanceAssert) HasVariable(name string) {
	variables, err := a.e.GetElementVariables(context.Background(), GetElementVariablesCmd{
		Partition:              a.partition,
		ElementInstanceId:      a.id,
		ExcludeParentVariables: true,
		Names:                  []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get element variable %s: %v", name, err)
	}
	if len(variables) == 0 {
		a.Fatalf("expected element instance %s to have variable %s", a, name)
	}
}

// IsCompleted asserts that an element instance is in state [InstanceCompleted].
func (a ElementInstanceAssert) IsCompleted() {
	a.HasState(InstanceCompleted)
}

// IsNotWaitingAt asserts that a scope is not waiting at a certain BPMN element.
func (a ElementInstanceAssert) IsNotWaitingAt(bpmnElementId string) {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.partition,
		ParentId:  a.id,
		States:    []InstanceState{InstanceCreated, InstanceStarted},
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	var isWaitingAt bool
	for _, elementInstance := range elementInstances {
		if elementInstance.BpmnElementId == bpmnElementId {
			isWaitingAt = true
		}
	}
	if !isWaitingAt {
		return
	}

	slices.SortFunc(elementInstances, func(a ElementInstance, b ElementInstance) int {
		return strings.Compare(a.BpmnElementId, b.BpmnElementId)
	})

	active := make([]string, len(elementInstances))
	for i, elementInstance := range elementInstances {
		active[i] = elementInstance.BpmnElementId
	}

	if len(elementInstances) != 0 {
		a.Fatalf("expected scope %s not to be waiting at %s\nactive elements: %s", a, bpmnElementId, strings.Join(active, ", "))
	}
}

// IsTerminated asserts that an element instance is in state [InstanceTerminated].
func (a ElementInstanceAssert) IsTerminated() {
	a.HasState(InstanceTerminated)
}

// IsWaitingAt asserts that a scope is waiting at a certain BPMN element.
// An assert for the waiting child element instance is returned.
func (a ElementInstanceAssert) IsWaitingAt(bpmnElementId string) ElementInstanceAssert {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.partition,
		ParentId:  a.id,
		States:    []InstanceState{InstanceCreated, InstanceStarted},
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	for _, elementInstance := range elementInstances {
		if elementInstance.BpmnElementId == bpmnElementId {
			return ElementInstanceAssert{
				t: a.t,
				e: a.e,

				partition:         elementInstance.Partition,
				id:                elementInstance.Id,
				parentId:          elementInstance.ParentId,
				processInstanceId: elementInstance.ProcessInstanceId,
				bpmnElementId:     elementInstance.BpmnElementId,
			}
		}
	}

	slices.SortFunc(elementInstances, func(a ElementInstance, b ElementInstance) int {
		return strings.Compare(a.BpmnElementId, b.BpmnElementId)
	})

	active := make([]string, len(elementInstances))
	for i, elementInstance := range elementInstances {
		active[i] = elementInstance.BpmnElementId
	}

	a.Fatalf("expected scope %s to be waiting at %s\nactive elements: %s", a, bpmnElementId, strings.Join(active, ", "))
	return ElementInstanceAssert{}
}

// Job returns the next active (not completed) job of an element instance.
func (a ElementInstanceAssert) Job() Job {
	jobs, err := a.e.CreateQuery().QueryJobs(context.Background(), JobCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		ElementInstanceId: a.id,
	})
	if err != nil {
		a.Fatalf("failed to query jobs: %v", err)
	}

	for i := len(jobs); i > 0; i-- {
		job := jobs[i-1]
		if !job.IsCompleted() {
			return job
		}
	}

	a.Fatalf("expected element instance %s to have an active job", a)
	return Job{}
}

// Parent asserts the parent of an element instance.
//
// Fails if no parent element instance exists.
func (a ElementInstanceAssert) Parent() ElementInstanceAssert {
	if a.parentId == 0 {
		a.Fatalf("expected element instance %s to have a parent", a)
	}

	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.partition,
		Id:        a.parentId,
	})
	if err != nil {
		a.Fatalf("failed to query element instance: %v", err)
	}
	if len(elementInstances) == 0 {
		a.Fatalf("expected one element instance, but got none")
	}

	return ElementInstanceAssert{
		t: a.t,
		e: a.e,

		partition:         elementInstances[0].Partition,
		id:                elementInstances[0].Id,
		parentId:          elementInstances[0].ParentId,
		processInstanceId: elementInstances[0].ProcessInstanceId,
		bpmnElementId:     elementInstances[0].BpmnElementId,
	}
}

func (a ElementInstanceAssert) String() string {
	return fmt.Sprintf("%s (%s/%d)", a.bpmnElementId, a.partition, a.id)
}

// Task returns the next active (not completed) task of an element instance.
func (a ElementInstanceAssert) Task() Task {
	tasks, err := a.e.CreateQuery().QueryTasks(context.Background(), TaskCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		ElementInstanceId: a.id,
	})
	if err != nil {
		a.Fatalf("failed to query tasks: %v", err)
	}

	for i := len(tasks); i > 0; i-- {
		task := tasks[i-1]
		if !task.IsCompleted() {
			return task
		}
	}

	a.Fatalf("expected element instance %s to have an active task", a)
	return Task{}
}

// UserTask returns the related user task.
func (a ElementInstanceAssert) UserTask() UserTask {
	userTasks, err := a.e.CreateQuery().QueryUserTasks(context.Background(), UserTaskCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		ElementInstanceId: a.id,
	})
	if err != nil {
		a.Fatalf("failed to query user task: %v", err)
	}
	if len(userTasks) == 0 {
		a.Fatalf("expected element instance %s to have a user task", a)
	}

	return userTasks[0]
}

func fatalf(t *testing.T, format string, args ...any) {
	data := map[string]string{
		"Error Trace": string(debug.Stack()),
		"Error":       fmt.Sprintf(format, args...),
		"Test":        t.Name(),
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "\n%s: %s", k, data[k])
	}

	t.Fatal(sb.String())
}
