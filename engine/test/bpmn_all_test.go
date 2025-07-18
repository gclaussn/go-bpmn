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

	t.Run("exclusive gateway", func(t *testing.T) {
		for i, e := range engines {
			exclusiveGatewayTest := newExclusiveGatewayTest(t, e)

			t.Run(engineTypes[i]+"gateway", exclusiveGatewayTest.gateway)

			t.Run(engineTypes[i]+"completes with error when none BPMN element ID set", exclusiveGatewayTest.errorNoBpmnElementId)
			t.Run(engineTypes[i]+"completes with error when sequence flow not exists", exclusiveGatewayTest.errorSequenceFlowNotExits)
		}
	})

	t.Run("inclusive gateway", func(t *testing.T) {
		for i, e := range engines {
			inclusiveGatewayTest := newInclusiveGatewayTest(t, e)

			t.Run(engineTypes[i]+"gateway all", inclusiveGatewayTest.gatewayAll)
			t.Run(engineTypes[i]+"gateway one", inclusiveGatewayTest.gatewayOne)

			t.Run(engineTypes[i]+"completes with error when no BPMN element ID set", inclusiveGatewayTest.errorNoBpmnElementId)
			t.Run(engineTypes[i]+"completes with error when duplicate BPMN element ID set", inclusiveGatewayTest.errorDuplicateBpmnElementId)
			t.Run(engineTypes[i]+"completes with error when sequence flow not exists", inclusiveGatewayTest.errorSequenceFlowNotExits)
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

			t.Run(engineTypes[i]+"catch", signalEventTest.catch)
			t.Run(engineTypes[i]+"start", signalEventTest.start)
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

			t.Run(engineTypes[i]+"completes with error when completed with BPMN error code", taskTest.errorBpmnErrorCodeNotSupported)
			t.Run(engineTypes[i]+"completes with error completed with BPMN esclation code", taskTest.errorBpmnEscalationCodeNotSupported)
		}
	})
}

func TestBpmnTimerEvent(t *testing.T) {
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
