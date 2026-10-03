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

const (
	testWorkerId = "test-worker"
)

func Assert2(t *testing.T, e Engine, processInstance ProcessInstance) (ProcessInstanceAssert2, ElementInstanceAssert) {
	elements, err := e.CreateQuery().QueryElements(context.Background(), ElementCriteria{
		ProcessId: processInstance.ProcessId,
	})
	if err != nil {
		t.Fatalf("failed to query elements: %v", err)
	}

	elementMap := make(map[string]Element, len(elements))
	for _, element := range elements {
		elementMap[element.BpmnElementId] = element
	}

	elementInstances, err := e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		BpmnElementId:     processInstance.BpmnProcessId,
	})
	if err != nil {
		t.Fatalf("failed to query element instances: %v", err)
	}
	if len(elementInstances) == 0 {
		t.Fatalf("expected process scope, but got none")
	}

	processInstanceAssert := ProcessInstanceAssert2{
		t: t,
		e: e,

		partition: processInstance.Partition,
		id:        processInstance.Id,
	}

	processScopeAssert := ElementInstanceAssert{
		t: t,
		e: e,

		partition:         elementInstances[0].Partition,
		id:                elementInstances[0].Id,
		parentId:          elementInstances[0].ParentId,
		processInstanceId: elementInstances[0].ProcessInstanceId,
		bpmnElementId:     elementInstances[0].BpmnElementId,
	}

	return processInstanceAssert, processScopeAssert
}

func AssertMessageStart2(t *testing.T, e Engine, process Process, cmd SendMessageCmd) (ProcessInstanceAssert2, ElementInstanceAssert) {
	message, err := e.SendMessage(context.Background(), cmd)
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	if !message.IsCorrelated {
		t.Fatalf("expected message to be correlated")
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
		if element.EventDefinition.MessageName == cmd.Name && !element.EventDefinition.IsSuspended {
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

	return Assert2(t, e, ProcessInstance{
		Partition: completedTasks[0].Partition,
		Id:        completedTasks[0].ProcessInstanceId,

		BpmnProcessId: messageEvent.ParentBpmnElementId,
	})
}

func AssertSignalStart2(t *testing.T, e Engine, process Process, cmd SendSignalCmd) (ProcessInstanceAssert2, ElementInstanceAssert) {
	signal, err := e.SendSignal(context.Background(), cmd)
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	elements, err := e.CreateQuery().QueryElements(context.Background(), ElementCriteria{})
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

	return Assert2(t, e, ProcessInstance{
		Partition: completedTasks[0].Partition,
		Id:        completedTasks[0].ProcessInstanceId,

		BpmnProcessId: signalEvent.ParentBpmnElementId,
	})
}

func AsserTimerStart2(t *testing.T, e Engine, process Process, startEventId string) (ProcessInstanceAssert2, ElementInstanceAssert) {
	elements, err := e.CreateQuery().QueryElements(context.Background(), ElementCriteria{
		ProcessId: process.Id,
	})
	if err != nil {
		t.Fatalf("failed to query elements: %v", err)
	}

	var timerEvent Element
	for _, element := range elements {
		if element.BpmnElementId == startEventId {
			timerEvent = element
			break
		}
	}

	if timerEvent.Id == 0 {
		t.Fatalf("expected to find timer event")
	}
	if timerEvent.BpmnElementType != model.ElementTimerStartEvent {
		t.Fatalf("expected timer event to be a start event: %+v", timerEvent)
	}

	tasks, err := e.CreateQuery().QueryTasks(context.Background(), TaskCriteria{
		ElementId: timerEvent.Id,
		Type:      TaskTriggerEvent,
	})
	if err != nil {
		t.Fatalf("failed to query tasks: %v", err)
	}

	var nextTrigger Task
	for _, task := range tasks {
		if !task.IsCompleted() {
			nextTrigger = task
			break
		}
	}
	if nextTrigger.Id == 0 {
		t.Fatal("failed to find trigger event task")
	}

	if _, _, err := e.SetTime(context.Background(), SetTimeCmd{
		Time: nextTrigger.DueAt,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	completedTasks, _, err := e.ExecuteTasks(context.Background(), ExecuteTasksCmd{
		Partition: nextTrigger.Partition,
		Id:        nextTrigger.Id,
	})
	if err != nil {
		t.Fatalf("failed to execute task: %v", err)
	}
	if len(completedTasks) == 0 {
		t.Fatal("expected trigger event task to complete")
	}

	return Assert2(t, e, ProcessInstance{
		Partition: completedTasks[0].Partition,
		Id:        completedTasks[0].ProcessInstanceId,

		BpmnProcessId: timerEvent.ParentBpmnElementId,
	})
}

type ProcessInstanceAssert2 struct {
	t *testing.T
	e Engine

	partition Partition
	id        int32
}

func (a ProcessInstanceAssert2) ElementInstances(criteria ...ElementInstanceCriteria) []ElementInstance {
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

func (a ProcessInstanceAssert2) ExecuteTasks() []Task {
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

func (a ProcessInstanceAssert2) Fatalf(format string, args ...any) {
	fatalf(a.t, format, args...)
}

func (a ProcessInstanceAssert2) HasNoProcessVariable(name string) {
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
			a.Fatalf("expected process instance to have no variable %s", name)
		}
	}
}

func (a ProcessInstanceAssert2) HasProcessVariable(name string) {
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

	a.Fatalf("expected process instance to have variable %s", name)
}

func (a ProcessInstanceAssert2) IsActive() {
	processInstance := a.ProcessInstance()
	if processInstance.State != InstanceStarted {
		a.Fatalf("expected process instance not to be completed, but is %s", processInstance.State)
	}
}

func (a ProcessInstanceAssert2) IsCompleted() {
	if a.ProcessInstance().State != InstanceCompleted {
		a.Fatalf("expected process instance to be completed")
	}
}

func (a ProcessInstanceAssert2) Jobs(criteria ...JobCriteria) []Job {
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

func (a ProcessInstanceAssert2) ProcessInstance() ProcessInstance {
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

func (a ProcessInstanceAssert2) Tasks(criteria ...TaskCriteria) []Task {
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

func (a ProcessInstanceAssert2) UserTasks(criteria ...UserTaskCriteria) []UserTask {
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

type ElementInstanceAssert struct {
	t *testing.T
	e Engine

	partition         Partition
	id                int32
	parentId          int32
	processInstanceId int32
	bpmnElementId     string
}

func (a ElementInstanceAssert) Child() (ElementInstanceAssert, bool) {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.partition,
		ParentId:  a.parentId,
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

func (a ElementInstanceAssert) ChildAt(bpmnElementId string) (ElementInstanceAssert, bool) {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition:     a.partition,
		ParentId:      a.parentId,
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

func (a ElementInstanceAssert) Children() []ElementInstanceAssert {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.partition,
		ParentId:  a.parentId,
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

func (a ElementInstanceAssert) ChildrenAt(bpmnElementId string) []ElementInstanceAssert {
	elementInstances, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition:     a.partition,
		ParentId:      a.parentId,
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

func (a ElementInstanceAssert) CompleteJob(completeJobCmds ...CompleteJobCmd) {
	if len(completeJobCmds) > 1 {
		a.Fatalf("expected zero or one complete job command")
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

	var completeJobCmd CompleteJobCmd
	if len(completeJobCmds) != 0 {
		completeJobCmd = completeJobCmds[0]
	} else {
		completeJobCmd = CompleteJobCmd{}
	}

	completeJobCmd.Partition = lockedJobs[0].Partition
	completeJobCmd.Id = lockedJobs[0].Id
	completeJobCmd.WorkerId = testWorkerId

	completedJob, err := a.e.CompleteJob(context.Background(), completeJobCmd)
	if err != nil {
		a.Fatalf("failed to complete job %s: %v", lockedJobs[0], err)
	}
	if completedJob.HasError() {
		a.Fatalf("expected job %s to complete without an error, but got: %s", completedJob, completedJob.Error)
	}
}

func (a ElementInstanceAssert) CompleteJobWithError(completeJobCmds ...CompleteJobCmd) Job {
	if len(completeJobCmds) > 1 {
		a.Fatalf("expected zero or one complete job command")
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

	var completeJobCmd CompleteJobCmd
	if len(completeJobCmds) != 0 {
		completeJobCmd = completeJobCmds[0]
	} else {
		completeJobCmd = CompleteJobCmd{}
	}

	completeJobCmd.Partition = lockedJobs[0].Partition
	completeJobCmd.Id = lockedJobs[0].Id
	completeJobCmd.WorkerId = testWorkerId

	completedJob, err := a.e.CompleteJob(context.Background(), completeJobCmd)
	if err != nil {
		a.Fatalf("failed to complete job %s: %v", lockedJobs[0], err)
	}
	if !completedJob.HasError() {
		a.Fatalf("expected job %s to complete with an error", completedJob)
	}

	return completedJob
}

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

func (a ElementInstanceAssert) ExecuteTask() {
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
}

func (a ElementInstanceAssert) Fatalf(format string, args ...any) {
	fatalf(a.t, format, args...)
}

func (a ElementInstanceAssert) HasJob(jobType JobType) {
	job := a.Job()
	if job.Type != jobType {
		a.Fatalf("expected element instance %s to have an active job of type %s", a, jobType)
	}
}

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

func (a ElementInstanceAssert) HasTask(taskType TaskType) {
	task := a.Task()
	if task.Type != taskType {
		a.Fatalf("expected element instance %s to have an active task of type %s", a, taskType)
	}
}

func (a ElementInstanceAssert) IsActive() {
	elementInstance := a.ElementInstance()
	if elementInstance.State != InstanceCreated && elementInstance.State != InstanceStarted {
		a.Fatalf("expected element instance %s not to be completed, but is %s", a, elementInstance.State)
	}
}

func (a ElementInstanceAssert) IsCompleted() {
	elementInstance := a.ElementInstance()
	if elementInstance.State != InstanceCompleted {
		a.Fatalf("expected element instance %s to be completed, but is %s", a, elementInstance.State)
	}
}

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

func (a ElementInstanceAssert) IsTerminated() {
	elementInstance := a.ElementInstance()
	if elementInstance.State != InstanceTerminated {
		a.Fatalf("expected element instance %s to be terminated, but is %s", a, elementInstance.State)
	}
}

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
