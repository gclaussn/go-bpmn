package worker

import (
	"slices"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
)

func Assert(t *testing.T, worker *Worker, processInstance engine.ProcessInstance) *ProcessInstanceAssert {
	assert := engine.Assert(t, worker.engine, processInstance)
	return &ProcessInstanceAssert{t, worker, assert}
}

type ProcessInstanceAssert struct {
	t      *testing.T
	worker *Worker
	assert *engine.ProcessInstanceAssert // decorated assert
}

func (a *ProcessInstanceAssert) ExecuteJob() {
	processInstance := a.assert.ProcessInstance()
	elementInstance := a.assert.ElementInstance()

	lockedJobs, err := a.worker.engine.LockJobs(engine.LockJobsCmd{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		ElementInstanceId: elementInstance.Id,
		WorkerId:          a.worker.id,
	})
	if err != nil {
		a.assert.Fatalf("failed to lock job: %v", err)
	}

	if len(lockedJobs) == 0 {
		a.assert.Fatalf("no job locked")
	}

	executedJob, err := a.worker.ExecuteJob(lockedJobs[0])
	if err != nil {
		a.assert.Fatalf("failed to execute job %s: %v", lockedJobs[0], err)
	}
	if executedJob.HasError() {
		a.assert.Fatalf("failed to execute job %s: %s", lockedJobs[0], executedJob.Error)
	}
}

func (a *ProcessInstanceAssert) ExecuteJobWithError() {
	processInstance := a.assert.ProcessInstance()
	elementInstance := a.assert.ElementInstance()

	lockedJobs, err := a.worker.engine.LockJobs(engine.LockJobsCmd{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		ElementInstanceId: elementInstance.Id,
		WorkerId:          a.worker.id,
	})
	if err != nil {
		a.assert.Fatalf("failed to lock job: %v", err)
	}

	if len(lockedJobs) == 0 {
		a.assert.Fatalf("no job locked")
	}

	executedJob, err := a.worker.ExecuteJob(lockedJobs[0])
	if err != nil {
		a.assert.Fatalf("failed to execute job %s: %v", lockedJobs[0], err)
	}
	if !executedJob.HasError() {
		a.assert.Fatalf("expected job %s to complete with an error, but is not", lockedJobs[0])
	}
}

func (a *ProcessInstanceAssert) ExecuteTask() {
	processInstance := a.assert.ProcessInstance()
	elementInstance := a.assert.ElementInstance()

	completedTasks, _, err := a.worker.engine.ExecuteTasks(engine.ExecuteTasksCmd{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		ElementInstanceId: elementInstance.Id,
	})
	if err != nil {
		a.t.Fatalf("failed to execute task: %v", err)
	}

	if len(completedTasks) == 0 {
		a.assert.Fatalf("no task completed")
	}

	if completedTasks[0].HasError() {
		a.assert.Fatalf("completed task %s has error: %s", completedTasks[0], completedTasks[0].Error)
	}
}

func (a *ProcessInstanceAssert) GetElementVariable(bpmnElementId string, name string, value any) {
	processInstance := a.assert.ProcessInstance()

	elementInstances := a.assert.ElementInstances(engine.ElementInstanceCriteria{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		BpmnElementId:     bpmnElementId,
	})

	if len(elementInstances) == 0 {
		a.assert.Fatalf("failed to find element instance for BPMN element ID %s", bpmnElementId)
	}

	slices.SortFunc(elementInstances, func(a engine.ElementInstance, b engine.ElementInstance) int {
		return int(b.Id - a.Id)
	})

	elementVariables, err := a.worker.engine.GetElementVariables(engine.GetElementVariablesCmd{
		Partition:         processInstance.Partition,
		ElementInstanceId: elementInstances[0].Id,
		Names:             []string{name},
	})
	if err != nil {
		a.assert.Fatalf("failed to get element variable %s: %v", name, err)
	}

	data, ok := elementVariables[name]
	if !ok {
		a.assert.Fatalf("expected element variable %s to exist, but is not", name)
	}

	decoder := a.worker.Decoder(data.Encoding)
	if decoder == nil {
		a.assert.Fatalf("no decoder for encoding %s registered", data.Encoding)
	}

	decoder.Decode(data.Value, &value)
}

func (a *ProcessInstanceAssert) GetProcessVariable(name string, value any) {
	processInstance := a.assert.ProcessInstance()

	processVariables, err := a.worker.engine.GetProcessVariables(engine.GetProcessVariablesCmd{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		Names:             []string{name},
	})
	if err != nil {
		a.assert.Fatalf("failed to get process variable %s: %v", name, err)
	}

	data, ok := processVariables[name]
	if !ok {
		a.assert.Fatalf("expected process variable %s to exist, but is not", name)
	}

	decoder := a.worker.Decoder(data.Encoding)
	if decoder == nil {
		a.assert.Fatalf("no decoder for encoding %s registered", data.Encoding)
	}

	decoder.Decode(data.Value, &value)
}

func (a *ProcessInstanceAssert) IsEnded() {
	a.assert.IsEnded()
}

func (a *ProcessInstanceAssert) IsNotEnded() {
	a.assert.IsNotEnded()
}

func (a *ProcessInstanceAssert) IsWaitingAt(bpmnElementId string) {
	a.assert.IsWaitingAt(bpmnElementId)
}

func (a *ProcessInstanceAssert) ProcessVariableNotExists(name string) {
	processInstance := a.assert.ProcessInstance()

	processVariables, err := a.worker.engine.GetProcessVariables(engine.GetProcessVariablesCmd{
		Partition:         processInstance.Partition,
		ProcessInstanceId: processInstance.Id,
		Names:             []string{name},
	})
	if err != nil {
		a.assert.Fatalf("failed to get process variable %s: %v", name, err)
	}

	if _, ok := processVariables[name]; ok {
		a.assert.Fatalf("expected process variable %s to not exist, but is", name)
	}
}
