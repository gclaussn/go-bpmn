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

func (e *testWorkerEngine) CreateProcess(cmd engine.CreateProcessCmd) (engine.Process, error) {
	return engine.Process{Id: 1}, nil
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

type testDelegate struct {
}

func (d testDelegate) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	return engine.CreateProcessCmd{
		BpmnProcessId: "test",
		Version:       "1",
	}, nil
}

func (d testDelegate) Delegate(delegator Delegator) error {
	return nil
}

func TestRegister(t *testing.T) {
	assert := assert.New(t)

	// given
	e := &testWorkerEngine{}

	w, _ := New(e)

	t.Run("returns error when already registered", func(t *testing.T) {
		// given
		w.processes[1] = Process{}

		// when
		_, err := w.Register(testDelegate{})

		// then
		assert.Error(err)
		assert.Contains(err.Error(), "test:1")
	})
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
