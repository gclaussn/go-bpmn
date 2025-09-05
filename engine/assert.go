package engine

import (
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/model"
)

func Assert(t *testing.T, e Engine, processInstance ProcessInstance) *ProcessInstanceAssert {
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

	return &ProcessInstanceAssert{
		t: t,
		e: e,

		elements: elementMap,

		partition:         processInstance.Partition,
		processInstanceId: processInstance.Id,
	}
}

// AssertSignalStart asserts a process instance started by a signal start event.
//
// Since a process can have multiple signal start events, the ID of the BPMN start element must be provided.
func AssertSignalStart(t *testing.T, e Engine, processId int32, bpmnStartElementId string, variables ...map[string]*Data) *ProcessInstanceAssert {
	elements, err := e.CreateQuery().QueryElements(context.Background(), ElementCriteria{ProcessId: processId})
	if err != nil {
		t.Fatalf("failed to query elements: %v", err)
	}

	elementMap := make(map[string]Element, len(elements))
	for _, element := range elements {
		elementMap[element.BpmnElementId] = element
	}

	var element Element
	for bpmnElementId := range elements {
		if elements[bpmnElementId].BpmnElementId == bpmnStartElementId {
			element = elements[bpmnElementId]
			break
		}
	}

	if element.EventDefinition == nil {
		t.Fatalf("failed to find event definition: %v", err)
	}

	signalVariables := make(map[string]*Data)
	for _, v := range variables {
		for variableName, data := range v {
			signalVariables[variableName] = data
		}
	}

	_, err = e.SendSignal(context.Background(), SendSignalCmd{
		Name:      element.EventDefinition.SignalName,
		Variables: signalVariables,
		WorkerId:  "test-worker",
	})
	if err != nil {
		t.Fatalf("failed to send signal: %v", err)
	}

	completedTasks, failedTasks, err := e.ExecuteTasks(context.Background(), ExecuteTasksCmd{
		ElementId: element.Id,

		Limit: 1,
	})
	if err != nil {
		t.Fatalf("failed to execute trigger event task: %v", err)
	}

	if len(failedTasks) != 0 || len(completedTasks) == 0 {
		t.Fatal("trigger event task failed")
	}

	processInstances, err := e.CreateQuery().QueryProcessInstances(context.Background(), ProcessInstanceCriteria{ProcessId: processId})
	if err != nil {
		t.Fatalf("failed to query process instances: %v", err)
	}

	var startedProcessInstance ProcessInstance
	for _, processInstance := range processInstances {
		if processInstance.CreatedAt == *completedTasks[0].CompletedAt {
			startedProcessInstance = processInstance
			break
		}
	}
	if startedProcessInstance.Id == 0 {
		t.Fatal("failed to find process instance")
	}

	return &ProcessInstanceAssert{
		t: t,
		e: e,

		elements: elementMap,

		partition:         startedProcessInstance.Partition,
		processInstanceId: startedProcessInstance.Id,
	}
}

// AsserTimerStart asserts a process instance started by a timer start event.
//
// startTime is used to increase the engine's time, so that the related trigger timer event task becomes due.
// Since a process can have multiple timer start events, startTime must equal the task's due date.
// As a result, the related task as well as the created process instance are found.
func AsserTimerStart(t *testing.T, e Engine, processId int32, startTime time.Time) *ProcessInstanceAssert {
	startTime = startTime.UTC().Truncate(time.Millisecond)

	if err := e.SetTime(context.Background(), SetTimeCmd{
		Time: startTime,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	elements, err := e.CreateQuery().QueryElements(context.Background(), ElementCriteria{
		ProcessId: processId,
	})
	if err != nil {
		t.Fatalf("failed to query elements: %v", err)
	}

	limit := 0
	for _, element := range elements {
		if element.BpmnElementType == model.ElementTimerStartEvent {
			limit++
		}
	}

	completedTasks, failedTasks, err := e.ExecuteTasks(context.Background(), ExecuteTasksCmd{
		ProcessId: processId,
		Type:      TaskTriggerEvent,

		Limit: limit,
	})
	if err != nil {
		t.Fatalf("failed to execute tasks: %v", err)
	}

	if len(failedTasks) != 0 {
		t.Fatal("one or multiple trigger event tasks failed")
	}

	var createdAt time.Time
	for _, completedTask := range completedTasks {
		if completedTask.DueAt != startTime {
			continue
		}

		if completedTask.HasError() {
			t.Fatalf("trigger event task %s has error: %s", completedTask, completedTask.Error)
		}

		createdAt = *completedTask.CompletedAt
		break
	}

	if createdAt.IsZero() {
		t.Fatalf("failed to find trigger event task for start time %v", startTime)
	}

	processInstances, err := e.CreateQuery().QueryProcessInstances(context.Background(), ProcessInstanceCriteria{
		ProcessId: processId,
	})
	if err != nil {
		t.Fatalf("failed to query process instances: %v", err)
	}

	var startedProcessInstance ProcessInstance
	for _, processInstance := range processInstances {
		if processInstance.CreatedAt == createdAt {
			startedProcessInstance = processInstance
			break
		}
	}
	if startedProcessInstance.Id == 0 {
		t.Fatal("failed to find process instance")
	}

	elementMap := make(map[string]Element, len(elements))
	for _, element := range elements {
		elementMap[element.BpmnElementId] = element
	}

	return &ProcessInstanceAssert{
		t: t,
		e: e,

		elements: elementMap,

		partition:         startedProcessInstance.Partition,
		processInstanceId: startedProcessInstance.Id,
	}
}

type ProcessInstanceAssert struct {
	t *testing.T
	e Engine

	elements map[string]Element

	partition         Partition
	processInstanceId int32
	elementInstanceId int32
	bpmnElementId     string
}

func (a *ProcessInstanceAssert) CompleteJob(completeJobCmds ...CompleteJobCmd) {
	job := a.Job()

	lockedJobs, err := a.e.LockJobs(context.Background(), LockJobsCmd{
		Partition: job.Partition,
		Id:        job.Id,
		WorkerId:  "test-worker",
	})
	if err != nil {
		a.Fatalf("failed to lock job: %v", err)
	}

	if len(lockedJobs) == 0 {
		a.Fatalf("no job locked")
	}

	var completeJobCmd CompleteJobCmd
	if len(completeJobCmds) != 0 {
		completeJobCmd = completeJobCmds[0]
	} else {
		completeJobCmd = CompleteJobCmd{}
	}

	completeJobCmd.Id = lockedJobs[0].Id
	completeJobCmd.Partition = a.partition
	completeJobCmd.WorkerId = "test-worker"

	completedJob, err := a.e.CompleteJob(context.Background(), completeJobCmd)
	if err != nil {
		a.Fatalf("failed to complete job %s: %v", lockedJobs[0], err)
	}
	if completedJob.HasError() {
		a.Fatalf("failed to complete job %s: %s", lockedJobs[0], completedJob.Error)
	}

	a.bpmnElementId = ""
	a.elementInstanceId = 0
}

func (a *ProcessInstanceAssert) CompleteJobWithError(completeJobCmds ...CompleteJobCmd) Job {
	job := a.Job()

	lockedJobs, err := a.e.LockJobs(context.Background(), LockJobsCmd{
		Partition: job.Partition,
		Id:        job.Id,
		WorkerId:  "test-worker",
	})
	if err != nil {
		a.Fatalf("failed to lock job: %v", err)
	}

	if len(lockedJobs) == 0 {
		a.Fatalf("no job locked")
	}

	var completeJobCmd CompleteJobCmd
	if len(completeJobCmds) != 0 {
		completeJobCmd = completeJobCmds[0]
	} else {
		completeJobCmd = CompleteJobCmd{}
	}

	completeJobCmd.Id = lockedJobs[0].Id
	completeJobCmd.Partition = a.partition
	completeJobCmd.WorkerId = "test-worker"

	completedJob, err := a.e.CompleteJob(context.Background(), completeJobCmd)
	if err != nil {
		a.Fatalf("failed to complete job %s: %v", lockedJobs[0], err)
	}
	if !completedJob.HasError() {
		a.Fatalf("expected job %s to complete with an error, but is not", lockedJobs[0])
	}

	return completedJob
}

func (a *ProcessInstanceAssert) ElementInstance() ElementInstance {
	if a.elementInstanceId == 0 {
		a.Fatalf("call IsWaitingAt first")
	}

	results, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.partition,
		Id:        a.elementInstanceId,
	})
	if err != nil {
		a.Fatalf("failed to query element instance: %v", err)
	}

	if len(results) != 1 {
		a.Fatalf("expected one element instance, but got %d", len(results))
	}

	return results[0]
}

func (a *ProcessInstanceAssert) ElementInstances(criteria ...ElementInstanceCriteria) []ElementInstance {
	var c ElementInstanceCriteria
	if len(criteria) != 0 {
		c = criteria[0]
	} else {
		c = ElementInstanceCriteria{}
	}

	c.Partition = a.partition
	c.ProcessInstanceId = a.processInstanceId

	results, err := a.e.CreateQuery().QueryElementInstances(context.Background(), c)
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	return results
}

func (a *ProcessInstanceAssert) ExecuteTask() {
	task := a.Task()

	completedTasks, _, err := a.e.ExecuteTasks(context.Background(), ExecuteTasksCmd{
		Partition: task.Partition,
		Id:        task.Id,
	})
	if err != nil {
		a.Fatalf("failed to execute task: %v", err)
	}

	if len(completedTasks) == 0 {
		a.Fatalf("no task completed")
	}

	if completedTasks[0].HasError() {
		a.Fatalf("completed task %s has error: %s", completedTasks[0], completedTasks[0].Error)
	}

	a.bpmnElementId = ""
	a.elementInstanceId = 0
}

func (a *ProcessInstanceAssert) ExecuteTasks() []Task {
	completedTasks, failedTasks, err := a.e.ExecuteTasks(context.Background(), ExecuteTasksCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
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
		a.Fatalf("expected zero failed tasks, but got %d", len(failedTasks))
	}

	return completedTasks
}

func (a *ProcessInstanceAssert) Fatalf(format string, args ...any) {
	data := map[string]string{
		"Error Trace": string(debug.Stack()),
		"Error":       fmt.Sprintf(format, args...),
		"Test":        a.t.Name(),
	}

	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("\n%s: %s", k, data[k]))
	}

	a.t.Fatal(sb.String())
}

func (a *ProcessInstanceAssert) HasPassed(bpmnElementId string) {
	results, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		States:            []InstanceState{InstanceCompleted},
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	for _, result := range results {
		if result.BpmnElementId == bpmnElementId {
			return
		}
	}

	slices.SortFunc(results, func(a ElementInstance, b ElementInstance) int {
		if a.EndedAt == nil {
			return -1
		} else if b.EndedAt == nil {
			return 1
		}

		if *a.EndedAt == *b.EndedAt {
			return int(a.Id - b.Id)
		} else if a.EndedAt.Before(*b.EndedAt) {
			return -1
		} else {
			return 1
		}
	})

	passed := make([]string, len(results))
	for i, result := range results {
		passed[i] = result.BpmnElementId
	}

	a.Fatalf("expected process instance to have passed %s, but has not\npassed elements: %s", bpmnElementId, strings.Join(passed, ", "))
}

func (a *ProcessInstanceAssert) IsCompleted() {
	if a.ProcessInstance().State != InstanceCompleted {
		a.Fatalf("expected process instance to be completed, but is not")
	}
}

func (a *ProcessInstanceAssert) IsNotCompleted() {
	if a.ProcessInstance().State == InstanceCompleted {
		a.Fatalf("expected process instance not to be completed, but is")
	}
}

func (a *ProcessInstanceAssert) IsNotWaitingAt(bpmnElementId string) {
	if _, ok := a.elements[bpmnElementId]; !ok {
		a.Fatalf("expected process instance not to be waiting at %s: process has no such BPMN element", bpmnElementId)
	}

	results, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		BpmnElementId:     bpmnElementId,
		States:            []InstanceState{InstanceStarted},
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	if len(results) != 0 {
		a.Fatalf("expected process instance not to be waiting at %s: active element instances found: %d", bpmnElementId, len(results))
	}
}

func (a *ProcessInstanceAssert) IsWaitingAt(bpmnElementId string) {
	if _, ok := a.elements[bpmnElementId]; !ok {
		a.Fatalf("expected process instance to be waiting at %s: process has no such BPMN element", bpmnElementId)
	}

	results, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		BpmnElementId:     bpmnElementId,
		States:            []InstanceState{InstanceStarted},
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	if len(results) != 0 {
		a.bpmnElementId = bpmnElementId
		a.elementInstanceId = results[0].Id
		return
	}

	a.Fatalf("expected process instance to be waiting at %s: no active element instance found", bpmnElementId)
}

func (a *ProcessInstanceAssert) HasNoProcessVariable(name string) {
	processVariables, err := a.e.GetProcessVariables(context.Background(), GetProcessVariablesCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		Names:             []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get process variable %s: %v", name, err)
	}

	if _, ok := processVariables[name]; ok {
		a.Fatalf("expected process instance to have no variable %s, but has", name)
	}
}

func (a *ProcessInstanceAssert) HasProcessVariable(name string) {
	processVariables, err := a.e.GetProcessVariables(context.Background(), GetProcessVariablesCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		Names:             []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get process variable %s: %v", name, err)
	}

	if _, ok := processVariables[name]; !ok {
		a.Fatalf("expected process instance to have variable %s, but has not", name)
	}
}

func (a *ProcessInstanceAssert) Job() Job {
	if a.elementInstanceId == 0 {
		a.Fatalf("call IsWaitingAt first")
	}

	results, err := a.e.CreateQuery().QueryJobs(context.Background(), JobCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		ElementInstanceId: a.elementInstanceId,
	})
	if err != nil {
		a.Fatalf("failed to query jobs: %v", err)
	}

	for _, result := range results {
		if !result.IsCompleted() {
			return result
		}
	}

	a.Fatalf("expected process instance to have an active job at %s", a.bpmnElementId)
	return Job{}
}

func (a *ProcessInstanceAssert) ProcessInstance() ProcessInstance {
	results, err := a.e.CreateQuery().QueryProcessInstances(context.Background(), ProcessInstanceCriteria{
		Partition: a.partition,
		Id:        a.processInstanceId,
	})
	if err != nil {
		a.Fatalf("failed to query process instance: %v", err)
	}

	if len(results) != 1 {
		a.Fatalf("expected one process instance, but got %d", len(results))
	}

	return results[0]
}

func (a *ProcessInstanceAssert) Task() Task {
	if a.elementInstanceId == 0 {
		a.Fatalf("call IsWaitingAt first")
	}

	results, err := a.e.CreateQuery().QueryTasks(context.Background(), TaskCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		ElementInstanceId: a.elementInstanceId,
	})
	if err != nil {
		a.Fatalf("failed to query tasks: %v", err)
	}

	for _, result := range results {
		if !result.IsCompleted() {
			return result
		}
	}

	a.Fatalf("expected process instance to have an active task at %s", a.bpmnElementId)
	return Task{}
}
