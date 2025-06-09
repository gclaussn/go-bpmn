package engine

import (
	"fmt"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"testing"
)

func Assert(t *testing.T, e Engine, processInstance ProcessInstance) *ProcessInstanceAssert {
	results, err := e.Query(ElementCriteria{
		ProcessId: processInstance.ProcessId,
	})
	if err != nil {
		t.Fatalf("failed to query elements: %v", err)
	}

	elements := make(map[string]Element, len(results))
	for i := 0; i < len(results); i++ {
		element := results[i].(Element)
		elements[element.BpmnElementId] = element
	}

	return &ProcessInstanceAssert{
		t: t,
		e: e,

		elements: elements,

		partition:         processInstance.Partition,
		processInstanceId: processInstance.Id,
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

	lockedJobs, err := a.e.LockJobs(LockJobsCmd{
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

	completedJob, err := a.e.CompleteJob(completeJobCmd)
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

	lockedJobs, err := a.e.LockJobs(LockJobsCmd{
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

	completedJob, err := a.e.CompleteJob(completeJobCmd)
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

	results, err := a.e.Query(ElementInstanceCriteria{
		Partition: a.partition,
		Id:        a.elementInstanceId,
	})
	if err != nil {
		a.Fatalf("failed to query element instance: %v", err)
	}

	if len(results) != 1 {
		a.Fatalf("expected one element instance, but got %d", len(results))
	}

	return results[0].(ElementInstance)
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

	results, err := a.e.Query(c)
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	elementInstances := make([]ElementInstance, len(results))
	for i := 0; i < len(elementInstances); i++ {
		elementInstances[i] = results[i].(ElementInstance)
	}

	return elementInstances
}

func (a *ProcessInstanceAssert) ExecuteTask() {
	task := a.Task()

	completedTasks, failedTasks, err := a.e.ExecuteTasks(ExecuteTasksCmd{
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

	if len(failedTasks) != 0 {
		a.Fatalf("expected zero failed tasks, but got %d", len(failedTasks))
	}

	a.bpmnElementId = ""
	a.elementInstanceId = 0
}

func (a *ProcessInstanceAssert) ExecuteTasks() []Task {
	completedTasks, failedTasks, err := a.e.ExecuteTasks(ExecuteTasksCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		Limit:             100,
	})
	if err != nil {
		a.Fatalf("failed to execute tasks: %v", err)
	}

	for i := 0; i < len(completedTasks); i++ {
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
	results, err := a.e.Query(ElementInstanceCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		States:            []InstanceState{InstanceEnded},
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	elementInstances := make([]ElementInstance, len(results))
	for i := 0; i < len(results); i++ {
		elementInstances[i] = results[i].(ElementInstance)

		if elementInstances[i].BpmnElementId == bpmnElementId {
			return
		}
	}

	slices.SortFunc(elementInstances, func(a ElementInstance, b ElementInstance) int {
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

	passed := make([]string, len(elementInstances))
	for i := 0; i < len(elementInstances); i++ {
		passed[i] = elementInstances[i].BpmnElementId
	}

	a.Fatalf("expected process instance to have passed %s, but was not\npassed elements: %s", bpmnElementId, strings.Join(passed, ", "))
}

func (a *ProcessInstanceAssert) IsEnded() {
	if !a.ProcessInstance().IsEnded() {
		a.Fatalf("expected process instance to be ended, but is not ended")
	}
}

func (a *ProcessInstanceAssert) IsNotEnded() {
	if a.ProcessInstance().IsEnded() {
		a.Fatalf("expected process instance not to be ended, but is ended")
	}
}

func (a *ProcessInstanceAssert) IsNotWaitingAt(bpmnElementId string) {
	if _, ok := a.elements[bpmnElementId]; !ok {
		a.Fatalf("expected process instance not to be waiting at %s: process has no such BPMN element", bpmnElementId)
	}

	results, err := a.e.Query(ElementInstanceCriteria{
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

	results, err := a.e.Query(ElementInstanceCriteria{
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
		a.elementInstanceId = results[0].(ElementInstance).Id
		return
	}

	a.Fatalf("expected process instance to be waiting at %s: no active element instance found", bpmnElementId)
}

func (a *ProcessInstanceAssert) Job() Job {
	if a.elementInstanceId == 0 {
		a.Fatalf("call IsWaitingAt first")
	}

	results, err := a.e.Query(JobCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		ElementInstanceId: a.elementInstanceId,
	})
	if err != nil {
		a.Fatalf("failed to query jobs: %v", err)
	}

	for i := 0; i < len(results); i++ {
		job := results[i].(Job)
		if !job.IsCompleted() {
			return job
		}
	}

	a.Fatalf("expected process instance to have an active job at %s", a.bpmnElementId)
	return Job{}
}

func (a *ProcessInstanceAssert) ProcessInstance() ProcessInstance {
	results, err := a.e.Query(ProcessInstanceCriteria{
		Partition: a.partition,
		Id:        a.processInstanceId,
	})
	if err != nil {
		a.Fatalf("failed to query process instance: %v", err)
	}

	if len(results) != 1 {
		a.Fatalf("expected one process instance, but got %d", len(results))
	}

	return results[0].(ProcessInstance)
}

func (a *ProcessInstanceAssert) Task() Task {
	if a.elementInstanceId == 0 {
		a.Fatalf("call IsWaitingAt first")
	}

	results, err := a.e.Query(TaskCriteria{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		ElementInstanceId: a.elementInstanceId,
	})
	if err != nil {
		a.Fatalf("failed to query tasks: %v", err)
	}

	for i := 0; i < len(results); i++ {
		task := results[i].(Task)
		if !task.IsCompleted() {
			return task
		}
	}

	a.Fatalf("expected process instance to have an active task at %s", a.bpmnElementId)
	return Task{}
}
