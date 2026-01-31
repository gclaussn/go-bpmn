package worker

import (
	"context"
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

func (e *testWorkerEngine) CreateProcess(ctx context.Context, cmd engine.CreateProcessCmd) (engine.Process, error) {
	return engine.Process{Id: 1}, nil
}

func (e *testWorkerEngine) CompleteJob(ctx context.Context, cmd engine.CompleteJobCmd) (engine.Job, error) {
	e.executeJobCalled++
	return engine.Job{}, nil
}

func (e *testWorkerEngine) LockJobs(ctx context.Context, cmd engine.LockJobsCmd) ([]engine.Job, error) {
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

type testHandler struct {
}

func (h testHandler) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	return engine.CreateProcessCmd{
		BpmnProcessId: "test",
		Version:       "1",
	}, nil
}

func (h testHandler) Handle(mux JobMux) error {
	return nil
}

func TestRegister(t *testing.T) {
	assert := assert.New(t)

	// given
	e := &testWorkerEngine{}

	w, _ := New(e)

	t.Run("returns error when already registered", func(t *testing.T) {
		// given
		w.processHandles[1] = ProcessHandle{}

		// when
		_, err := w.Register(testHandler{})

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

	mux := JobMux{}
	mux.Execute("a", func(_ JobContext) error {
		return nil
	})

	w.processHandles[1] = ProcessHandle{mux: mux}
	w.processHandles[2] = ProcessHandle{mux: mux}
	w.processHandles[3] = ProcessHandle{mux: mux}

	// when
	w.Start()
	time.Sleep(time.Millisecond * 350)
	w.Stop()

	// then
	assert.Equal(6, e.executeJobCalled)
	assert.Equal(3, e.lockJobsCalled)
}
