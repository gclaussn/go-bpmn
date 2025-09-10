package test

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func newTimerEventTest(t *testing.T, e engine.Engine) timerEventTest {
	return timerEventTest{
		e: e,

		catchTest: mustCreateProcess(t, e, "event/timer-catch.bpmn", "timerCatchTest"),
	}
}

type timerEventTest struct {
	e engine.Engine

	catchTest engine.Process
}

func (x timerEventTest) catch(t *testing.T) {
	piAssert := mustCreateProcessInstance(t, x.e, x.catchTest)

	triggerAt := time.Now().Add(time.Hour)

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				Time: triggerAt,
			},
		},
	})

	if err := x.e.SetTime(context.Background(), engine.SetTimeCmd{
		Time: triggerAt,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsCompleted()
}

func (x timerEventTest) start(t *testing.T) {
	bpmnXml := mustReadBpmnFile(t, "event/timer-start.bpmn")

	process, err := x.e.CreateProcess(context.Background(), engine.CreateProcessCmd{
		BpmnProcessId: "timerStartTest",
		BpmnXml:       bpmnXml,
		Timers: map[string]*engine.Timer{
			"timerStartEvent": {
				TimeCycle: "0 * * * *",
			},
		},
		Version:  "1",
		WorkerId: testWorkerId,
	})
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	piAssert1 := engine.AsserTimerStart(t, x.e, process.Id, "timerStartEvent")
	piAssert1.IsCompleted()

	piAssert2 := engine.AsserTimerStart(t, x.e, process.Id, "timerStartEvent")
	piAssert2.IsCompleted()

	assert := assert.New(t)
	assert.NotEqual(piAssert1.ProcessInstance().String(), piAssert2.ProcessInstance().String())
}
