package worker

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
)

// Assert asserts a process instance.
// Asserts for the process instance and it's process (root) scope are returned.
func Assert(t *testing.T, w *Worker, processInstance engine.ProcessInstance) (ProcessInstanceAssert, ElementInstanceAssert) {
	piAssert, psAssert := engine.Assert(t, w.e, processInstance)

	processInstanceAssert := ProcessInstanceAssert{
		ProcessInstanceAssert: piAssert,

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

// ProcessInstanceAssert is used to assert state and variables of a process instance.
type ProcessInstanceAssert struct {
	engine.ProcessInstanceAssert

	w *Worker

	partition         engine.Partition
	processInstanceId int32
}

// Child asserts the next, not ended child of a process instance.
// Asserts for the child process instance and it's process (root) scope are returned.
//
// If no child process instances exist or all child process instances are ended, false is returned.
func (a ProcessInstanceAssert) Child() (ProcessInstanceAssert, ElementInstanceAssert, bool) {
	child, childScope, ok := a.ProcessInstanceAssert.Child()

	subProcessInstance := child.ProcessInstance()

	embeddedChild := ProcessInstanceAssert{
		ProcessInstanceAssert: child,

		w: a.w,

		partition:         subProcessInstance.Partition,
		processInstanceId: subProcessInstance.Id,
	}

	embeddedChildScope := ElementInstanceAssert{
		ElementInstanceAssert: childScope,

		w: a.w,
	}

	return embeddedChild, embeddedChildScope, ok
}

// Children returns asserts for all child process instances.
func (a ProcessInstanceAssert) Children() []ProcessInstanceAssert {
	children := a.ProcessInstanceAssert.Children()
	embedded := make([]ProcessInstanceAssert, len(children))

	for i, child := range children {
		embedded[i] = ProcessInstanceAssert{
			ProcessInstanceAssert: child,

			w: a.w,
		}
	}

	return embedded
}

// GetVariable gets a process variable with name and stores the result in value.
//
// Fails if no such process variable exists or no decoder for the variable's encoding is registered.
func (a ProcessInstanceAssert) GetVariable(name string, value any) {
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

// Parent asserts the parent of a process instance.
//
// Fails if no parent process instance exists.
func (a ProcessInstanceAssert) Parent() ProcessInstanceAssert {
	parent := a.ProcessInstanceAssert.Parent()

	parentProcessInstance := parent.ProcessInstance()

	return ProcessInstanceAssert{
		ProcessInstanceAssert: parent,

		w: a.w,

		partition:         parentProcessInstance.Partition,
		processInstanceId: parentProcessInstance.Id,
	}
}

// ProcessScope asserts a process instance's process (root) scope.
func (a ProcessInstanceAssert) ProcessScope() ElementInstanceAssert {
	processScope := a.ProcessInstanceAssert.ProcessScope()

	return ElementInstanceAssert{
		ElementInstanceAssert: processScope,

		w: a.w,
	}
}

// ElementInstanceAssert is used to assert state and variables of an element instance.
type ElementInstanceAssert struct {
	engine.ElementInstanceAssert

	w *Worker
}

// Child asserts the next, not ended child of an element instance.
//
// If no child element instances exist or all child element instances are ended, false is returned.
func (a ElementInstanceAssert) Child() (ElementInstanceAssert, bool) {
	child, ok := a.ElementInstanceAssert.Child()

	return ElementInstanceAssert{
		ElementInstanceAssert: child,

		w: a.w,
	}, ok
}

// ChildAt asserts an element instance's next, not ended child instance at a certain BPMN element.
//
// If no such child element instance exists or all child element instances at the BPMN element are ended, false is returned.
func (a ElementInstanceAssert) ChildAt(bpmnElementId string) (ElementInstanceAssert, bool) {
	child, ok := a.ElementInstanceAssert.ChildAt(bpmnElementId)

	return ElementInstanceAssert{
		ElementInstanceAssert: child,

		w: a.w,
	}, ok
}

// Children returns asserts for all child element instances.
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

// ChildrenAt returns asserts for all child element instances at a certain BPMN element.
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

// ExecuteJob executes a due job of an element instance, using a registered [Handler].
//
// Fails if no job could be found or locked, or the job completed with an error.
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

// ExecuteJobWithError executes a due job of an element instance, using a registered [Handler].
//
// Fails if no job could be found or locked,or the job completed without an error.
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

// GetVariable gets an element variable with name and stores the result in value.
//
// Fails if no such element variable exists or no decoder for the variable's encoding is registered.
func (a ElementInstanceAssert) GetVariable(name string, value any) {
	elementInstance := a.ElementInstance()

	variables, err := a.w.e.GetElementVariables(context.Background(), engine.GetElementVariablesCmd{
		Partition:              elementInstance.Partition,
		ElementInstanceId:      elementInstance.Id,
		ExcludeParentVariables: true,
		Names:                  []string{name},
	})
	if err != nil {
		a.Fatalf("failed to get element variable %s: %v", name, err)
	}
	if len(variables) == 0 {
		a.Fatalf("expected element instance %s to have variable %s", a, name)
	}

	data := variables[0].Data

	decoder := a.w.Decoder(data.Encoding)
	if decoder == nil {
		a.Fatalf("no decoder for encoding %s registered", data.Encoding)
	}

	if err := decoder.Decode(data.Value, &value); err != nil {
		a.Fatalf("failed to decode variable: %v", err)
	}
}

// IsWaitingAt asserts that a scope is waiting at a certain BPMN element.
// An assert for the waiting child element instance is returned.
func (a ElementInstanceAssert) IsWaitingAt(bpmnElementId string) ElementInstanceAssert {
	assert := a.ElementInstanceAssert.IsWaitingAt(bpmnElementId)

	return ElementInstanceAssert{
		ElementInstanceAssert: assert,

		w: a.w,
	}
}

// Parent asserts the parent of an element instance.
//
// Fails if no parent element instance exists.
func (a ElementInstanceAssert) Parent() ElementInstanceAssert {
	parent := a.ElementInstanceAssert.Parent()

	return ElementInstanceAssert{
		ElementInstanceAssert: parent,

		w: a.w,
	}
}
