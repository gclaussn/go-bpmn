package worker

import (
	"context"
	"slices"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
)

func Assert(t *testing.T, w *Worker, processInstance engine.ProcessInstance) *ProcessInstanceAssert {
	assert := engine.Assert(t, w.e, processInstance)
	return &ProcessInstanceAssert{t, w, assert}
}

type ProcessInstanceAssert struct {
	t *testing.T
	w *Worker
	a *engine.ProcessInstanceAssert // decorated assert
}

func (a *ProcessInstanceAssert) ExecuteJob() {
	processInstance := a.a.ProcessInstance()
	elementInstance := a.a.ElementInstance()

	lockedJobs, err := a.w.e.LockJobs(context.Background(), engine.LockJobsCmd{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		ElementInstanceId: elementInstance.Id,
		WorkerId:          a.w.id,
	})
	if err != nil {
		a.a.Fatalf("failed to lock job: %v", err)
	}

	if len(lockedJobs) == 0 {
		a.a.Fatalf("no job locked")
	}

	executedJob, err := a.w.ExecuteJob(context.Background(), lockedJobs[0])
	if err != nil {
		a.a.Fatalf("failed to execute job %s: %v", lockedJobs[0], err)
	}
	if executedJob.HasError() {
		a.a.Fatalf("failed to execute job %s: %s", lockedJobs[0], executedJob.Error)
	}
}

func (a *ProcessInstanceAssert) ExecuteJobWithError() {
	processInstance := a.a.ProcessInstance()
	elementInstance := a.a.ElementInstance()

	lockedJobs, err := a.w.e.LockJobs(context.Background(), engine.LockJobsCmd{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		ElementInstanceId: elementInstance.Id,
		WorkerId:          a.w.id,
	})
	if err != nil {
		a.a.Fatalf("failed to lock job: %v", err)
	}

	if len(lockedJobs) == 0 {
		a.a.Fatalf("no job locked")
	}

	executedJob, err := a.w.ExecuteJob(context.Background(), lockedJobs[0])
	if err != nil {
		a.a.Fatalf("failed to execute job %s: %v", lockedJobs[0], err)
	}
	if !executedJob.HasError() {
		a.a.Fatalf("expected job %s to complete with an error, but is not", lockedJobs[0])
	}
}

func (a *ProcessInstanceAssert) ExecuteTask() {
	processInstance := a.a.ProcessInstance()
	elementInstance := a.a.ElementInstance()

	completedTasks, _, err := a.w.e.ExecuteTasks(context.Background(), engine.ExecuteTasksCmd{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		ElementInstanceId: elementInstance.Id,
	})
	if err != nil {
		a.t.Fatalf("failed to execute task: %v", err)
	}

	if len(completedTasks) == 0 {
		a.a.Fatalf("no task completed")
	}

	if completedTasks[0].HasError() {
		a.a.Fatalf("completed task %s has error: %s", completedTasks[0], completedTasks[0].Error)
	}
}

func (a *ProcessInstanceAssert) GetElementVariable(bpmnElementId string, name string, value any) {
	processInstance := a.a.ProcessInstance()

	elementInstances := a.a.ElementInstances(engine.ElementInstanceCriteria{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		BpmnElementId:     bpmnElementId,
	})

	if len(elementInstances) == 0 {
		a.a.Fatalf("failed to find element instance for BPMN element ID %s", bpmnElementId)
	}

	slices.SortFunc(elementInstances, func(a engine.ElementInstance, b engine.ElementInstance) int {
		return int(b.Id - a.Id)
	})

	elementVariables, err := a.w.e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
		Partition:         processInstance.Partition,
		ElementInstanceId: elementInstances[0].Id,
		Names:             []string{name},
	})
	if err != nil {
		a.a.Fatalf("failed to get element variable %s: %v", name, err)
	}

	data, ok := elementVariables[name]
	if !ok {
		a.a.Fatalf("expected element variable %s to exist, but is not", name)
	}

	decoder := a.w.Decoder(data.Encoding)
	if decoder == nil {
		a.a.Fatalf("no decoder for encoding %s registered", data.Encoding)
	}

	decoder.Decode(data.Value, &value)
}

func (a *ProcessInstanceAssert) GetProcessVariable(name string, value any) {
	processInstance := a.a.ProcessInstance()

	processVariables, err := a.w.e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		Names:             []string{name},
	})
	if err != nil {
		a.a.Fatalf("failed to get process variable %s: %v", name, err)
	}

	data, ok := processVariables[name]
	if !ok {
		a.a.Fatalf("expected process variable %s to exist, but is not", name)
	}

	decoder := a.w.Decoder(data.Encoding)
	if decoder == nil {
		a.a.Fatalf("no decoder for encoding %s registered", data.Encoding)
	}

	decoder.Decode(data.Value, &value)
}

func (a *ProcessInstanceAssert) IsCompleted() {
	a.a.IsCompleted()
}

func (a *ProcessInstanceAssert) IsNotCompleted() {
	a.a.IsNotCompleted()
}

func (a *ProcessInstanceAssert) IsWaitingAt(bpmnElementId string) {
	a.a.IsWaitingAt(bpmnElementId)
}

func (a *ProcessInstanceAssert) HasNoProcessVariable(name string) {
	processInstance := a.a.ProcessInstance()

	processVariables, err := a.w.e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		Names:             []string{name},
	})
	if err != nil {
		a.a.Fatalf("failed to get process variable %s: %v", name, err)
	}

	if _, ok := processVariables[name]; ok {
		a.a.Fatalf("expected process instance to have no variable %s, but has", name)
	}
}
