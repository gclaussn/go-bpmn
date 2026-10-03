package worker

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
)

func Assert2(t *testing.T, w *Worker, processInstance engine.ProcessInstance) (ProcessInstanceAssert2, ElementInstanceAssert) {
	piAssert, psAssert := engine.Assert2(t, w.e, processInstance)

	processInstanceAssert := ProcessInstanceAssert2{
		ProcessInstanceAssert2: piAssert,

		w: w,

		partition:         processInstance.Partition,
		processInstanceId: processInstance.Id,
	}

	processScopeAssert := ElementInstanceAssert{
		ElementInstanceAssert: psAssert,

		w: w,
	}

	return processInstanceAssert, processScopeAssert
}

type ProcessInstanceAssert2 struct {
	engine.ProcessInstanceAssert2

	w *Worker

	partition         engine.Partition
	processInstanceId int32
}

func (a ProcessInstanceAssert2) GetProcessVariable(name string, value any) {
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

func (a ProcessInstanceAssert2) HasNoProcessVariable(name string) {
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

type ElementInstanceAssert struct {
	engine.ElementInstanceAssert

	w *Worker
}

func (a ElementInstanceAssert) Child() (ElementInstanceAssert, bool) {
	child, ok := a.ElementInstanceAssert.Child()

	return ElementInstanceAssert{
		ElementInstanceAssert: child,

		w: a.w,
	}, ok
}

func (a ElementInstanceAssert) ChildAt(bpmnElementId string) (ElementInstanceAssert, bool) {
	child, ok := a.ElementInstanceAssert.ChildAt(bpmnElementId)

	return ElementInstanceAssert{
		ElementInstanceAssert: child,

		w: a.w,
	}, ok
}

func (a ElementInstanceAssert) Children() []ElementInstanceAssert {
	children := a.ElementInstanceAssert.Children()
	embedded := make([]ElementInstanceAssert, len(children))

	for i, child := range children {
		embedded[i] = ElementInstanceAssert{
			ElementInstanceAssert: child,

			w: a.w,
		}
	}

	return embedded
}

func (a ElementInstanceAssert) ChildrenAt(bpmnElementId string) []ElementInstanceAssert {
	children := a.ElementInstanceAssert.ChildrenAt(bpmnElementId)
	embedded := make([]ElementInstanceAssert, len(children))

	for i, child := range children {
		embedded[i] = ElementInstanceAssert{
			ElementInstanceAssert: child,

			w: a.w,
		}
	}

	return embedded
}

func (a ElementInstanceAssert) ExecuteJob() {
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

func (a ElementInstanceAssert) ExecuteJobWithError() {
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

func (a ElementInstanceAssert) GetElementVariable(bpmnElementId string, name string, value any) {
	elementInstance := a.ElementInstance()

	variables, err := a.w.e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
		Partition:         elementInstance.Partition,
		ElementInstanceId: elementInstance.Id,
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
		a.Fatalf("expected element instance %s to have variable %s", a, name)
	}

	decoder := a.w.Decoder(data.Encoding)
	if decoder == nil {
		a.Fatalf("no decoder for encoding %s registered", data.Encoding)
	}

	if err := decoder.Decode(data.Value, &value); err != nil {
		a.Fatalf("failed to decode variable: %v", err)
	}
}

func (a ElementInstanceAssert) HasNoElementVariable(bpmnElementId string, name string) {
	elementInstance := a.ElementInstance()

	variables, err := a.w.e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
		Partition:         elementInstance.Partition,
		ElementInstanceId: elementInstance.Id,
		Names:             []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get element variable %s: %v", name, err)
	}

	for _, variable := range variables {
		if variable.BpmnElementId == bpmnElementId {
			a.Fatalf("expected element instance %s to have no variable %s", a, name)
		}
	}
}

func (a ElementInstanceAssert) Parent() ElementInstanceAssert {
	parent := a.ElementInstanceAssert.Parent()

	return ElementInstanceAssert{
		ElementInstanceAssert: parent,

		w: a.w,
	}
}
