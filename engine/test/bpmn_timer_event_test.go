package test

import (
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
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

	triggerAt := time.Now().Add(time.Hour * 1)

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.CompleteJob(engine.CompleteJobCmd{
		Completion: &engine.JobCompletion{
			Timer: &engine.Timer{
				Time: triggerAt,
			},
		},
	})

	if err := x.e.SetTime(engine.SetTimeCmd{
		Time: triggerAt,
	}); err != nil {
		t.Fatalf("failed to set time: %v", err)
	}

	piAssert.IsWaitingAt("timerCatchEvent")
	piAssert.ExecuteTask()

	piAssert.IsEnded()
}
