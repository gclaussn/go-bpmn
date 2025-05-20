package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
)

func newTaskTest(t *testing.T, e engine.Engine) taskTest {
	return taskTest{
		e: e,

		businessRuleTest: mustCreateProcess(t, e, "task/business-rule.bpmn", "businessRuleTest"),
		manualTest:       mustCreateProcess(t, e, "task/manual.bpmn", "manualTest"),
		scriptTest:       mustCreateProcess(t, e, "task/script.bpmn", "scriptTest"),
		sendTest:         mustCreateProcess(t, e, "task/send.bpmn", "sendTest"),
		serviceTest:      mustCreateProcess(t, e, "task/service.bpmn", "serviceTest"),
		taskTest:         mustCreateProcess(t, e, "task/task.bpmn", "taskTest"),
	}
}

type taskTest struct {
	e engine.Engine

	businessRuleTest engine.Process
	manualTest       engine.Process
	scriptTest       engine.Process
	sendTest         engine.Process
	serviceTest      engine.Process
	taskTest         engine.Process
}

func (x taskTest) businessRule(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.businessRuleTest)

	piAssert.IsWaitingAt("businessRuleTask")
	piAssert.CompleteJob()
	piAssert.IsEnded()
}

func (x taskTest) manual(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.manualTest)
	piAssert.IsEnded()
}

func (x taskTest) script(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.scriptTest)

	piAssert.IsWaitingAt("scriptTask")
	piAssert.CompleteJob()
	piAssert.IsEnded()
}

func (x taskTest) send(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.sendTest)

	piAssert.IsWaitingAt("sendTask")
	piAssert.CompleteJob()
	piAssert.IsEnded()
}

func (x taskTest) service(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.serviceTest)

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJob()
	piAssert.IsEnded()
}

func (x taskTest) task(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.taskTest)
	piAssert.IsEnded()
}

func (x taskTest) errorBpmnErrorCodeNotSupported(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.serviceTest)

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			BpmnErrorCode: "error-code",
		},
	})
}

func (x taskTest) errorBpmnEscalationCodeNotSupported(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.serviceTest)

	piAssert.IsWaitingAt("serviceTask")
	piAssert.CompleteJobWithError(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			BpmnEscalationCode: "esclation-code",
		},
	})
}
