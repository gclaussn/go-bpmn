package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
)

type taskTest struct {
	e engine.Engine
}

func (x taskTest) businessRule(t *testing.T) {
	process := mustCreateProcess(t, x.e, "task/business-rule.bpmn", "businessRuleTest")

	piAssert, psAssert := mustCreateProcessInstance2(t, x.e, process)

	psAssert.IsWaitingAt("businessRuleTask").CompleteJob()
	piAssert.IsCompleted()
}

func (x taskTest) manual(t *testing.T) {
	process := mustCreateProcess(t, x.e, "task/manual.bpmn", "manualTest")

	piAssert, _ := mustCreateProcessInstance2(t, x.e, process)
	piAssert.IsCompleted()
}

func (x taskTest) script(t *testing.T) {
	process := mustCreateProcess(t, x.e, "task/script.bpmn", "scriptTest")

	piAssert, psAssert := mustCreateProcessInstance2(t, x.e, process)

	psAssert.IsWaitingAt("scriptTask").CompleteJob()
	piAssert.IsCompleted()
}

func (x taskTest) send(t *testing.T) {
	process := mustCreateProcess(t, x.e, "task/send.bpmn", "sendTest")

	piAssert, psAssert := mustCreateProcessInstance2(t, x.e, process)

	psAssert.IsWaitingAt("sendTask").CompleteJob()
	piAssert.IsCompleted()
}

func (x taskTest) service(t *testing.T) {
	process := mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

	piAssert, psAssert := mustCreateProcessInstance2(t, x.e, process)

	psAssert.IsWaitingAt("serviceTask").CompleteJob()
	piAssert.IsCompleted()
}

func (x taskTest) task(t *testing.T) {
	process := mustCreateProcess(t, x.e, "task/task.bpmn", "taskTest")

	piAssert, _ := mustCreateProcessInstance2(t, x.e, process)
	piAssert.IsCompleted()
}

func (x taskTest) errorBpmnErrorCodeNotSupported(t *testing.T) {
	process := mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

	_, psAssert := mustCreateProcessInstance2(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	serviceTask.HasJob(engine.JobExecute)
	serviceTask.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			ErrorCode: "error-code",
		},
	})
}

func (x taskTest) errorBpmnEscalationCodeNotSupported(t *testing.T) {
	process := mustCreateProcess(t, x.e, "task/service.bpmn", "serviceTest")

	_, psAssert := mustCreateProcessInstance2(t, x.e, process)

	serviceTask := psAssert.IsWaitingAt("serviceTask")

	serviceTask.HasJob(engine.JobExecute)
	serviceTask.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			EscalationCode: "esclation-code",
		},
	})
}
