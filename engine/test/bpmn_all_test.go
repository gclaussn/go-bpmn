package test

import (
	"testing"
)

func TestBpmn(t *testing.T) {
	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	t.Run("special", func(t *testing.T) {
		for i, e := range engines {
			specialTest := newSpecialTest(t, e)

			t.Run(engineTypes[i]+"startEnd", specialTest.startEnd)
		}
	})

	t.Run("error event", func(t *testing.T) {
		for i, e := range engines {
			errorEventTest := newErrorEventTest(t, e)

			t.Run(engineTypes[i]+"boundary", errorEventTest.boundary)
			t.Run(engineTypes[i]+"boundary with code", errorEventTest.boundaryWithCode)
			t.Run(engineTypes[i]+"boundary without code", errorEventTest.boundaryWithoutCode)
			t.Run(engineTypes[i]+"boundary terminated", errorEventTest.boundaryTerminated)
			t.Run(engineTypes[i]+"boundary not found", errorEventTest.boundaryNotFound)
			t.Run(engineTypes[i]+"boundary multiple", errorEventTest.boundaryMultiple)
			t.Run(engineTypes[i]+"boundary with event definition", errorEventTest.boundaryWithEventDefinition)
		}
	})

	t.Run("escalation event", func(t *testing.T) {
		for i, e := range engines {
			escalationEventTest := newEscalationEventTest(t, e)

			t.Run(engineTypes[i]+"boundary", escalationEventTest.boundary)
			t.Run(engineTypes[i]+"boundary event definition", escalationEventTest.boundaryEventDefinition)
			t.Run(engineTypes[i]+"boundary non-interrupting", escalationEventTest.boundaryNonInterrupting)
		}
	})

	t.Run("exclusive gateway", func(t *testing.T) {
		for i, e := range engines {
			exclusiveGatewayTest := newExclusiveGatewayTest(t, e)

			t.Run(engineTypes[i]+"gateway", exclusiveGatewayTest.gateway)
			t.Run(engineTypes[i]+"gateway default", exclusiveGatewayTest.gatewayDefault)

			t.Run(engineTypes[i]+"completes with error when none BPMN element ID set", exclusiveGatewayTest.errorNoBpmnElementId)
			t.Run(engineTypes[i]+"completes with error when sequence flow not exists", exclusiveGatewayTest.errorSequenceFlowNotExits)
		}
	})

	t.Run("inclusive gateway", func(t *testing.T) {
		for i, e := range engines {
			inclusiveGatewayTest := newInclusiveGatewayTest(t, e)

			t.Run(engineTypes[i]+"gateway all", inclusiveGatewayTest.gatewayAll)
			t.Run(engineTypes[i]+"gateway one", inclusiveGatewayTest.gatewayOne)
			t.Run(engineTypes[i]+"gateway default", inclusiveGatewayTest.gatewayDefault)
			t.Run(engineTypes[i]+"gateway default implicit", inclusiveGatewayTest.gatewayDefaultImplicit)
			t.Run(engineTypes[i]+"gateway default explicit", inclusiveGatewayTest.gatewayDefaultExplicit)

			t.Run(engineTypes[i]+"completes with error when no BPMN element ID set", inclusiveGatewayTest.errorNoBpmnElementId)
			t.Run(engineTypes[i]+"completes with error when duplicate BPMN element ID set", inclusiveGatewayTest.errorDuplicateBpmnElementId)
			t.Run(engineTypes[i]+"completes with error when sequence flow not exists", inclusiveGatewayTest.errorSequenceFlowNotExits)
		}
	})

	t.Run("message event", func(t *testing.T) {
		for i, e := range engines {
			messageEventTest := newMessageEventTest(t, e)

			t.Run(engineTypes[i]+"boundary", messageEventTest.boundary)
			t.Run(engineTypes[i]+"boundary with message sent before", messageEventTest.boundaryMessageSentBefore)
			t.Run(engineTypes[i]+"boundary non-interrupting", messageEventTest.boundaryNonInterrupting)
			t.Run(engineTypes[i]+"boundary non-interrupting with message sent before", messageEventTest.boundaryNonInterruptingMessageSentBefore)
			t.Run(engineTypes[i]+"catch", messageEventTest.catch)
			t.Run(engineTypes[i]+"catchMessageSentBefore", messageEventTest.catchMessageSentBefore)
			t.Run(engineTypes[i]+"catch event definition", messageEventTest.catchDefinition)
			t.Run(engineTypes[i]+"end", messageEventTest.end)
			t.Run(engineTypes[i]+"start", messageEventTest.start)
			t.Run(engineTypes[i]+"startSingleton", messageEventTest.startSingleton)
			t.Run(engineTypes[i]+"start event definition", messageEventTest.startDefinition)
			t.Run(engineTypes[i]+"throw", messageEventTest.throw)
		}
	})

	t.Run("parallel gateway", func(t *testing.T) {
		for i, e := range engines {
			parallelGatewayTest := newParallelGatewayTest(t, e)

			t.Run(engineTypes[i]+"gateway", parallelGatewayTest.gateway)
			t.Run(engineTypes[i]+"serviceTasks", parallelGatewayTest.serviceTasks)
		}
	})

	t.Run("signal event", func(t *testing.T) {
		for i, e := range engines {
			signalEventTest := newSignalEventTest(t, e)

			t.Run(engineTypes[i]+"boundary", signalEventTest.boundary)
			t.Run(engineTypes[i]+"boundary non-interrupting", signalEventTest.boundaryNonInterrupting)
			t.Run(engineTypes[i]+"catch", signalEventTest.catch)
			t.Run(engineTypes[i]+"catch event definition", signalEventTest.catchDefinition)
			t.Run(engineTypes[i]+"start", signalEventTest.start)
			t.Run(engineTypes[i]+"start event definition", signalEventTest.startEventDefinition)
		}
	})

	t.Run("sub-process", func(t *testing.T) {
		for i, e := range engines {
			subProcessTest := subProcessTest{e}

			t.Run(engineTypes[i]+"start end", subProcessTest.startEnd)
			t.Run(engineTypes[i]+"boundary", subProcessTest.boundary)
			t.Run(engineTypes[i]+"boundary terminated", subProcessTest.boundaryTerminated)
			t.Run(engineTypes[i]+"nested", subProcessTest.nested)
			t.Run(engineTypes[i]+"nested termined", subProcessTest.nestedTerminated)
		}
	})

	t.Run("task", func(t *testing.T) {
		for i, e := range engines {
			taskTest := newTaskTest(t, e)

			t.Run(engineTypes[i]+"businessRule", taskTest.businessRule)
			t.Run(engineTypes[i]+"manual", taskTest.manual)
			t.Run(engineTypes[i]+"script", taskTest.script)
			t.Run(engineTypes[i]+"send", taskTest.send)
			t.Run(engineTypes[i]+"service", taskTest.service)
			t.Run(engineTypes[i]+"task", taskTest.task)

			t.Run(engineTypes[i]+"completes with error completed with BPMN esclation code", taskTest.errorBpmnEscalationCodeNotSupported)
		}
	})
}

func TestBpmnTimerEvent(t *testing.T) {
	t.Run("boundary", func(t *testing.T) {
		engines, engineTypes := mustCreateEngines(t)
		for _, e := range engines {
			defer e.Shutdown()
		}

		for i, e := range engines {
			timerEventTest := newTimerEventTest(t, e)

			t.Run(engineTypes[i]+"boundary", timerEventTest.boundary)
		}
	})

	t.Run("boundary with timer", func(t *testing.T) {
		engines, engineTypes := mustCreateEngines(t)
		for _, e := range engines {
			defer e.Shutdown()
		}

		for i, e := range engines {
			timerEventTest := newTimerEventTest(t, e)

			t.Run(engineTypes[i]+"boundary", timerEventTest.boundaryWithTimer)
		}
	})

	t.Run("boundary non-interrupting", func(t *testing.T) {
		engines, engineTypes := mustCreateEngines(t)
		for _, e := range engines {
			defer e.Shutdown()
		}

		for i, e := range engines {
			timerEventTest := newTimerEventTest(t, e)

			t.Run(engineTypes[i]+"boundary", timerEventTest.boundaryNonInterrupting)
		}
	})

	t.Run("catch", func(t *testing.T) {
		engines, engineTypes := mustCreateEngines(t)
		for _, e := range engines {
			defer e.Shutdown()
		}

		for i, e := range engines {
			timerEventTest := newTimerEventTest(t, e)

			t.Run(engineTypes[i]+"catch", timerEventTest.catch)
		}
	})

	t.Run("catch with timer", func(t *testing.T) {
		engines, engineTypes := mustCreateEngines(t)
		for _, e := range engines {
			defer e.Shutdown()
		}

		for i, e := range engines {
			timerEventTest := newTimerEventTest(t, e)

			t.Run(engineTypes[i]+"catch", timerEventTest.catchWithTimer)
		}
	})

	t.Run("start", func(t *testing.T) {
		engines, engineTypes := mustCreateEngines(t)
		for _, e := range engines {
			defer e.Shutdown()
		}

		for i, e := range engines {
			timerEventTest := newTimerEventTest(t, e)

			t.Run(engineTypes[i]+"start", timerEventTest.start)
		}
	})
}
