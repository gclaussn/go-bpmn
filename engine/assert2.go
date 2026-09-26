package engine

import (
	"context"
	"fmt"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"testing"
)

const (
	testWorkerId = "test-worker"
)

func Assert2(t *testing.T, e Engine, processInstance ProcessInstance) (*ProcessInstanceAssert2, *ScopeAssert) {
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

		partition:         processInstance.Partition,
		processInstanceId: processInstance.Id,
	}

	scopeAssert := ScopeAssert{
		t: t,
		e: e,

		elementMap: elementMap,

		scope: elementInstances[0],
	}

	return &processInstanceAssert, &scopeAssert
}

type ProcessInstanceAssert2 struct {
	t *testing.T
	e Engine

	partition         Partition
	processInstanceId int32
}

func (a *ProcessInstanceAssert2) ElementInstances(criteria ...ElementInstanceCriteria) []ElementInstance {
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

func (a *ProcessInstanceAssert2) ExecuteTasks() []Task {
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
		a.Fatalf("expected no failed tasks, but got %d: %+v", len(failedTasks), failedTasks)
	}

	return completedTasks
}

func (a *ProcessInstanceAssert2) Fatalf(format string, args ...any) {
	fatalf(a.t, format, args...)
}

func (a *ProcessInstanceAssert2) HasNoProcessVariable(name string) {
	variables, err := a.e.GetProcessVariables(context.Background(), GetProcessVariablesCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
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

func (a *ProcessInstanceAssert2) HasProcessVariable(name string) {
	variables, err := a.e.GetProcessVariables(context.Background(), GetProcessVariablesCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
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

func (a *ProcessInstanceAssert2) IsCompleted() {
	if a.ProcessInstance().State != InstanceCompleted {
		a.Fatalf("expected process instance to be completed")
	}
}

func (a *ProcessInstanceAssert2) IsNotCompleted() {
	if a.ProcessInstance().State == InstanceCompleted {
		a.Fatalf("expected process instance not to be completed")
	}
}

func (a *ProcessInstanceAssert2) Jobs(criteria ...JobCriteria) []Job {
	var c JobCriteria
	if len(criteria) != 0 {
		c = criteria[0]
	} else {
		c = JobCriteria{}
	}

	c.Partition = a.partition
	c.ProcessInstanceId = a.processInstanceId

	results, err := a.e.CreateQuery().QueryJobs(context.Background(), c)
	if err != nil {
		a.Fatalf("failed to query jobs: %v", err)
	}

	return results
}

func (a *ProcessInstanceAssert2) ProcessInstance() ProcessInstance {
	results, err := a.e.CreateQuery().QueryProcessInstances(context.Background(), ProcessInstanceCriteria{
		Partition: a.partition,
		Id:        a.processInstanceId,
	})
	if err != nil {
		a.Fatalf("failed to query process instance: %v", err)
	}

	if len(results) != 1 {
		a.Fatalf("expected one process instance, but got %d: %+v", len(results), results)
	}

	return results[0]
}

func (a *ProcessInstanceAssert2) Tasks(criteria ...TaskCriteria) []Task {
	var c TaskCriteria
	if len(criteria) != 0 {
		c = criteria[0]
	} else {
		c = TaskCriteria{}
	}

	c.Partition = a.partition
	c.ProcessInstanceId = a.processInstanceId

	results, err := a.e.CreateQuery().QueryTasks(context.Background(), c)
	if err != nil {
		a.Fatalf("failed to query tasks: %v", err)
	}

	return results
}

func (a *ProcessInstanceAssert2) UserTasks(criteria ...UserTaskCriteria) []UserTask {
	var c UserTaskCriteria
	if len(criteria) != 0 {
		c = criteria[0]
	} else {
		c = UserTaskCriteria{}
	}

	c.Partition = a.partition
	c.ProcessInstanceId = a.processInstanceId

	results, err := a.e.CreateQuery().QueryUserTasks(context.Background(), c)
	if err != nil {
		a.Fatalf("failed to query user tasks: %v", err)
	}

	return results
}

type ScopeAssert struct {
	t *testing.T
	e Engine

	elementMap map[string]Element

	scope ElementInstance

	elementInstance *ElementInstance
}

func (a *ScopeAssert) CompleteJob(completeJobCmds ...CompleteJobCmd) {
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

	a.elementInstance = nil
}

func (a *ScopeAssert) CompleteJobWithError(completeJobCmds ...CompleteJobCmd) Job {
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

func (a *ScopeAssert) ElementInstance() ElementInstance {
	if a.elementInstance == nil {
		a.Fatalf("call IsWaitingAt first")
	}

	results, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.scope.Partition,
		Id:        a.elementInstance.Id,
	})
	if err != nil {
		a.Fatalf("failed to query element instance: %v", err)
	}

	if len(results) != 1 {
		a.Fatalf("expected one element instance, but got %d: %+v", len(results), results)
	}

	return results[0]
}

func (a *ScopeAssert) ExecuteTask() {
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

	a.elementInstance = nil
}

func (a *ScopeAssert) Fatalf(format string, args ...any) {
	fatalf(a.t, format, args...)
}

func (a *ScopeAssert) HasPassed(bpmnElementId string) {
	results, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.scope.Partition,
		ParentId:  a.scope.Id,
		States:    []InstanceState{InstanceCompleted},
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

	passed := make([]string, len(results))
	for i, result := range results {
		passed[i] = result.BpmnElementId
	}

	a.Fatalf("expected scope %s to have passed %s\npassed elements: %s", a.scope, bpmnElementId, strings.Join(passed, ", "))
}

func (a *ScopeAssert) IsNotWaitingAt(bpmnElementId string) {
	if _, ok := a.elementMap[bpmnElementId]; !ok {
		a.Fatalf("expected scope %s not to be waiting at %s: process has no such BPMN element", a.scope, bpmnElementId)
	}

	results, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.scope.Partition,
		ParentId:  a.scope.Id,
		States:    []InstanceState{InstanceCreated, InstanceStarted},
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	var isWaitingAt bool
	for _, result := range results {
		if result.BpmnElementId == bpmnElementId {
			isWaitingAt = true
		}
	}
	if !isWaitingAt {
		return
	}

	slices.SortFunc(results, func(a ElementInstance, b ElementInstance) int {
		return strings.Compare(a.BpmnElementId, b.BpmnElementId)
	})

	active := make([]string, len(results))
	for i, result := range results {
		active[i] = result.BpmnElementId
	}

	if len(results) != 0 {
		a.Fatalf("expected scope %s not to be waiting at %s\nactive elements: %s", a.scope, bpmnElementId, strings.Join(active, ", "))
	}
}

func (a *ScopeAssert) IsWaitingAt(bpmnElementId string) {
	if _, ok := a.elementMap[bpmnElementId]; !ok {
		a.Fatalf("expected scope %s to be waiting at %s: process has no such BPMN element", a.scope, bpmnElementId)
	}

	results, err := a.e.CreateQuery().QueryElementInstances(context.Background(), ElementInstanceCriteria{
		Partition: a.scope.Partition,
		ParentId:  a.scope.Id,
		States:    []InstanceState{InstanceCreated, InstanceStarted},
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}

	for _, result := range results {
		if result.BpmnElementId == bpmnElementId {
			a.elementInstance = &result
			return
		}
	}

	slices.SortFunc(results, func(a ElementInstance, b ElementInstance) int {
		return strings.Compare(a.BpmnElementId, b.BpmnElementId)
	})

	active := make([]string, len(results))
	for i, result := range results {
		active[i] = result.BpmnElementId
	}

	a.Fatalf("expected scope %s to be waiting at %s\nactive elements: %s", a.scope, bpmnElementId, strings.Join(active, ", "))
}

func (a *ScopeAssert) Job() Job {
	if a.elementInstance == nil {
		a.Fatalf("call IsWaitingAt first")
	}

	results, err := a.e.CreateQuery().QueryJobs(context.Background(), JobCriteria{
		Partition:         a.scope.Partition,
		ProcessInstanceId: a.scope.ProcessInstanceId,
		ElementInstanceId: a.elementInstance.Id,
	})
	if err != nil {
		a.Fatalf("failed to query jobs: %v", err)
	}

	for i := len(results); i > 0; i-- {
		result := results[i-1]
		if !result.IsCompleted() {
			return result
		}
	}

	a.Fatalf("expected scope %s to have an active job at %s", a.scope, a.elementInstance.BpmnElementId)
	return Job{}
}

func (a *ScopeAssert) Task() Task {
	if a.elementInstance == nil {
		a.Fatalf("call IsWaitingAt first")
	}

	results, err := a.e.CreateQuery().QueryTasks(context.Background(), TaskCriteria{
		Partition:         a.scope.Partition,
		ProcessInstanceId: a.scope.ProcessInstanceId,
		ElementInstanceId: a.elementInstance.Id,
	})
	if err != nil {
		a.Fatalf("failed to query tasks: %v", err)
	}

	for i := len(results); i > 0; i-- {
		result := results[i-1]
		if !result.IsCompleted() {
			return result
		}
	}

	a.Fatalf("expected scope %s to have an active task at %s", a.scope, a.elementInstance.BpmnElementId)
	return Task{}
}

func (a *ScopeAssert) UserTask() UserTask {
	if a.elementInstance == nil {
		a.Fatalf("call IsWaitingAt first")
	}

	results, err := a.e.CreateQuery().QueryUserTasks(context.Background(), UserTaskCriteria{
		Partition:         a.scope.Partition,
		ProcessInstanceId: a.scope.ProcessInstanceId,
		ElementInstanceId: a.elementInstance.Id,
	})
	if err != nil {
		a.Fatalf("failed to query user task: %v", err)
	}

	if len(results) == 0 {
		a.Fatalf("expected scope %s to have a user task at %s", a.scope, a.elementInstance.BpmnElementId)
	}

	return results[0]
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
