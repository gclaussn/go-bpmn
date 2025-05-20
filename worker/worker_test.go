package worker

import (
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

type testWorkerEngine struct {
	engine.Engine

	executeJobCalled int
	lockJobsCalled   int
}

func (e *testWorkerEngine) CompleteJob(cmd engine.CompleteJobCmd) (engine.Job, error) {
	e.executeJobCalled++
	return engine.Job{}, nil
}

func (e *testWorkerEngine) LockJobs(cmd engine.LockJobsCmd) ([]engine.Job, error) {
	var lockedJobs []engine.Job
	switch e.lockJobsCalled {
	case 0:
		lockedJobs = append(lockedJobs, engine.Job{ProcessId: 1, BpmnElementId: "a"})
		lockedJobs = append(lockedJobs, engine.Job{ProcessId: 2, BpmnElementId: "a"})
		lockedJobs = append(lockedJobs, engine.Job{ProcessId: 3, BpmnElementId: "a"})
	case 1:
		lockedJobs = append(lockedJobs, engine.Job{ProcessId: 1, BpmnElementId: "a"})
		lockedJobs = append(lockedJobs, engine.Job{ProcessId: 2, BpmnElementId: "a"})
	case 2:
		lockedJobs = append(lockedJobs, engine.Job{ProcessId: 1, BpmnElementId: "a"})
	}

	e.lockJobsCalled++
	return lockedJobs, nil
}

func TestWorkerStartAndStop(t *testing.T) {
	assert := assert.New(t)

	// given
	e := &testWorkerEngine{}

	w, _ := New(e, func(o *Options) {
		o.JobExecutorInterval = time.Millisecond * 100
	})

	delegator := Delegator{}
	delegator.Execute("a", func(_ JobContext) error {
		return nil
	})

	w.processes[1] = Process{delegator: delegator}
	w.processes[2] = Process{delegator: delegator}
	w.processes[3] = Process{delegator: delegator}

	// when
	w.Start()
	time.Sleep(time.Millisecond * 350)
	w.Stop()

	// then
	assert.Equal(6, e.executeJobCalled)
	assert.Equal(3, e.lockJobsCalled)
}
