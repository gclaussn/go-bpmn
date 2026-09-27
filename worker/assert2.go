package worker

import (
	"context"
	"slices"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
)

func Assert2(t *testing.T, w *Worker, processInstance engine.ProcessInstance) (*ProcessInstanceAssert2, *ScopeAssert) {
	piAssert, psAssert := engine.Assert2(t, w.e, processInstance)

	processInstanceAssert := ProcessInstanceAssert2{
		ProcessInstanceAssert2: piAssert,

		w: w,

		partition:         processInstance.Partition,
		processInstanceId: processInstance.Id,
	}

	scopeAssert := ScopeAssert{
		ScopeAssert: psAssert,

		w: w,
	}

	return &processInstanceAssert, &scopeAssert
}

type ProcessInstanceAssert2 struct {
	*engine.ProcessInstanceAssert2

	w *Worker

	partition         engine.Partition
	processInstanceId int32
}

func (a *ProcessInstanceAssert2) GetProcessVariable(name string, value any) {
	variables, err := a.w.e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		Names:             []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get process variable %s: %v", name, err)
	}

	var data *engine.Data
	for _, variable := range variables {
		if variable.Name == name {
			data = variable.Data
		}
	}
	if data == nil {
		a.Fatalf("expected process instance to have variable %s", name)
	}

	decoder := a.w.Decoder(data.Encoding)
	if decoder == nil {
		a.Fatalf("no decoder for encoding %s registered", data.Encoding)
	}

	if err := decoder.Decode(data.Value, &value); err != nil {
		a.Fatalf("failed to decode variable %s: %v", name, err)
	}
}

func (a *ProcessInstanceAssert2) HasNoProcessVariable(name string) {
	variables, err := a.w.e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
		Partition:         a.partition,
		ProcessInstanceId: a.processInstanceId,
		Names:             []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get process variable %s: %v", name, err)
	}

	if len(variables) != 0 {
		a.Fatalf("expected process instance to have no variable %s", name)
	}
}

type ScopeAssert struct {
	*engine.ScopeAssert

	w *Worker
}

func (a *ScopeAssert) ExecuteJob() {
	job := a.Job()

	lockedJobs, err := a.w.e.LockJobs(context.Background(), engine.LockJobsCmd{
		Partition: job.Partition,
		Id:        job.Id,
		Limit:     1,
		WorkerId:  a.w.id,
	})
	if err != nil {
		a.Fatalf("failed to lock job: %v", err)
	}
	if len(lockedJobs) == 0 {
		a.Fatalf("expected one job to lock, but got none")
	}

	executedJob, err := a.w.ExecuteJob(context.Background(), lockedJobs[0])
	if err != nil {
		a.Fatalf("failed to execute job %s: %v", lockedJobs[0], err)
	}
	if executedJob.HasError() {
		a.Fatalf("expected job %s to execute without an error, but got: %s", executedJob, executedJob.Error)
	}
}

func (a *ScopeAssert) ExecuteJobWithError() {
	job := a.Job()

	lockedJobs, err := a.w.e.LockJobs(context.Background(), engine.LockJobsCmd{
		Partition: job.Partition,
		Id:        job.Id,
		Limit:     1,
		WorkerId:  a.w.id,
	})
	if err != nil {
		a.Fatalf("failed to lock job: %v", err)
	}
	if len(lockedJobs) == 0 {
		a.Fatalf("expected one job to lock, but got none")
	}

	executedJob, err := a.w.ExecuteJob(context.Background(), lockedJobs[0])
	if err != nil {
		a.Fatalf("failed to execute job %s: %v", lockedJobs[0], err)
	}
	if !executedJob.HasError() {
		a.Fatalf("expected job %s to execute with an error", executedJob)
	}
}

func (a *ScopeAssert) GetElementVariable(bpmnElementId string, name string, value any) {
	scope := a.Scope()

	elementInstances, err := a.w.e.CreateQuery().QueryElementInstances(context.Background(), engine.ElementInstanceCriteria{
		Partition:     scope.Partition,
		ParentId:      scope.Id,
		BpmnElementId: bpmnElementId,
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}
	if len(elementInstances) == 0 {
		a.Fatalf("expected at least one element instance, but got none")
	}

	slices.SortFunc(elementInstances, func(a engine.ElementInstance, b engine.ElementInstance) int {
		return int(b.Id - a.Id)
	})

	variables, err := a.w.e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
		Partition:         scope.Partition,
		ElementInstanceId: elementInstances[0].Id,
		Names:             []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get element variable %s: %v", name, err)
	}

	var data *engine.Data
	for _, variable := range variables {
		if variable.BpmnElementId == bpmnElementId {
			data = variable.Data
		}
	}
	if data == nil {
		a.Fatalf("expected element instance %s to have variable %s", elementInstances[0], name)
	}

	decoder := a.w.Decoder(data.Encoding)
	if decoder == nil {
		a.Fatalf("no decoder for encoding %s registered", data.Encoding)
	}

	if err := decoder.Decode(data.Value, &value); err != nil {
		a.Fatalf("failed to decode variable: %v", err)
	}
}

func (a *ScopeAssert) HasNoElementVariable(bpmnElementId string, name string) {
	scope := a.Scope()

	elementInstances, err := a.w.e.CreateQuery().QueryElementInstances(context.Background(), engine.ElementInstanceCriteria{
		Partition:     scope.Partition,
		ParentId:      scope.Id,
		BpmnElementId: bpmnElementId,
	})
	if err != nil {
		a.Fatalf("failed to query element instances: %v", err)
	}
	if len(elementInstances) == 0 {
		a.Fatalf("expected at least one element instance, but got none")
	}

	slices.SortFunc(elementInstances, func(a engine.ElementInstance, b engine.ElementInstance) int {
		return int(b.Id - a.Id)
	})

	variables, err := a.w.e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
		Partition:         scope.Partition,
		ElementInstanceId: elementInstances[0].Id,
		Names:             []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get element variable %s: %v", name, err)
	}

	for _, variable := range variables {
		if variable.BpmnElementId == bpmnElementId {
			a.Fatalf("expected element instance %s to have no variable %s", elementInstances[0], name)
		}
	}
}
