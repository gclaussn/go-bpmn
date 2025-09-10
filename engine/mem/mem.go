package mem

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
)

func New(customizers ...func(*Options)) (engine.Engine, error) {
	options := NewOptions()
	for _, customizer := range customizers {
		customizer(&options)
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	ctx := newMemContext(options)

	memEngine := memEngine{ctx: ctx, defaultQueryLimit: options.Common.DefaultQueryLimit}

	if options.Common.TaskExecutorEnabled {
		memEngine.taskExecutor = internal.NewTaskExecutor(
			&memEngine,
			options.Common.TaskExecutorInterval,
			options.Common.TaskExecutorLimit,
		)

		memEngine.taskExecutor.Execute()
	}

	return &memEngine, nil
}

func NewOptions() Options {
	return Options{
		Common: engine.Options{
			DefaultQueryLimit:    1000,
			EngineId:             engine.DefaultEngineId,
			JobRetryCount:        1,
			TaskExecutorEnabled:  false,
			TaskExecutorInterval: 60 * time.Second,
			TaskExecutorLimit:    10,
		},
	}
}

type Options struct {
	Common engine.Options // Common options
}

func (o Options) Validate() error {
	return o.Common.Validate()
}

type memEngine struct {
	ctxMutex   sync.RWMutex
	ctx        *memContext
	isReadLock bool

	defaultQueryLimit int

	offset       time.Duration
	taskExecutor *internal.TaskExecutor
}

func (e *memEngine) CompleteJob(_ context.Context, cmd engine.CompleteJobCmd) (engine.Job, error) {
	defer e.unlock()
	return internal.CompleteJob(e.wlock(), cmd)
}

func (e *memEngine) CreateProcess(_ context.Context, cmd engine.CreateProcessCmd) (engine.Process, error) {
	defer e.unlock()
	return internal.CreateProcess(e.wlock(), cmd)
}

func (e *memEngine) CreateProcessInstance(_ context.Context, cmd engine.CreateProcessInstanceCmd) (engine.ProcessInstance, error) {
	defer e.unlock()
	return internal.CreateProcessInstance(e.wlock(), cmd)
}

func (e *memEngine) CreateQuery() engine.Query {
	return &query{
		e: e,

		defaultQueryLimit: e.defaultQueryLimit,
		options:           engine.QueryOptions{Limit: e.defaultQueryLimit},
	}
}

func (e *memEngine) ExecuteTasks(_ context.Context, cmd engine.ExecuteTasksCmd) ([]engine.Task, []engine.Task, error) {
	defer e.unlock()
	ctx := e.wlock()

	lockedTasks, err := ctx.Tasks().Lock(cmd, ctx.Time())
	if err != nil {
		return nil, nil, err
	}

	var (
		completedTasks []engine.Task
		failedTasks    []engine.Task

		errs []error
	)
	for _, lockedTask := range lockedTasks {
		err := internal.ExecuteTask(ctx, lockedTask)

		task := lockedTask.Task()
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to execute task %s: %v", task, err))
			if onFailure := ctx.options.Common.OnTaskExecutionFailure; onFailure != nil {
				onFailure(task, err)
			}

			failedTasks = append(failedTasks, task)
		} else {
			completedTasks = append(completedTasks, task)
		}
	}

	if len(errs) == 0 {
		return completedTasks, failedTasks, nil
	} else {
		return completedTasks, failedTasks, errors.Join(errs...)
	}
}

func (e *memEngine) GetBpmnXml(_ context.Context, cmd engine.GetBpmnXmlCmd) (string, error) {
	defer e.unlock()
	return internal.GetBpmnXml(e.rlock(), cmd)
}

func (e *memEngine) GetElementVariables(_ context.Context, cmd engine.GetElementVariablesCmd) (map[string]engine.Data, error) {
	defer e.unlock()
	return internal.GetElementVariables(e.rlock(), cmd)
}

func (e *memEngine) GetProcessVariables(_ context.Context, cmd engine.GetProcessVariablesCmd) (map[string]engine.Data, error) {
	defer e.unlock()
	return internal.GetProcessVariables(e.rlock(), cmd)
}

func (e *memEngine) LockJobs(_ context.Context, cmd engine.LockJobsCmd) ([]engine.Job, error) {
	defer e.unlock()
	return internal.LockJobs(e.wlock(), cmd)
}

func (e *memEngine) ResolveIncident(_ context.Context, cmd engine.ResolveIncidentCmd) error {
	defer e.unlock()
	return internal.ResolveIncident(e.wlock(), cmd)
}

func (e *memEngine) ResumeProcessInstance(_ context.Context, cmd engine.ResumeProcessInstanceCmd) error {
	defer e.unlock()
	return internal.ResumeProcessInstance(e.wlock(), cmd)
}

func (e *memEngine) SendSignal(_ context.Context, cmd engine.SendSignalCmd) (engine.Signal, error) {
	defer e.unlock()
	return internal.SendSignal(e.wlock(), cmd)
}

func (e *memEngine) SetElementVariables(_ context.Context, cmd engine.SetElementVariablesCmd) error {
	defer e.unlock()
	return internal.SetElementVariables(e.wlock(), cmd)
}

func (e *memEngine) SetProcessVariables(_ context.Context, cmd engine.SetProcessVariablesCmd) error {
	defer e.unlock()
	return internal.SetProcessVariables(e.wlock(), cmd)
}

func (e *memEngine) SetTime(_ context.Context, cmd engine.SetTimeCmd) error {
	defer e.unlock()
	ctx := e.wlock()

	old := ctx.Time()
	new := cmd.Time.UTC().Truncate(time.Millisecond)

	sub := new.Sub(old)
	if sub.Milliseconds() < 0 {
		return engine.Error{
			Type:  engine.ErrorConflict,
			Title: "failed to set time",
			Detail: fmt.Sprintf(
				"time %s is before engine time %s",
				new.Format(time.RFC3339),
				old.Format(time.RFC3339),
			),
		}
	}

	e.offset = e.offset + sub
	return nil
}

func (e *memEngine) SuspendProcessInstance(_ context.Context, cmd engine.SuspendProcessInstanceCmd) error {
	defer e.unlock()
	return internal.SuspendProcessInstance(e.wlock(), cmd)
}

func (e *memEngine) UnlockJobs(_ context.Context, cmd engine.UnlockJobsCmd) (int, error) {
	defer e.unlock()
	return internal.UnlockJobs(e.wlock(), cmd)
}

func (e *memEngine) UnlockTasks(_ context.Context, cmd engine.UnlockTasksCmd) (int, error) {
	defer e.unlock()
	return internal.UnlockTasks(e.wlock(), cmd)
}

func (e *memEngine) Shutdown() {
	if e.taskExecutor != nil {
		e.taskExecutor.Stop()
	}

	e.ctx.clear()
}

func (e *memEngine) rlock() *memContext {
	now := time.Now()

	e.ctxMutex.RLock()
	e.isReadLock = true

	// must be UTC and truncated to millis (see engine/pg/pg.go:pgEngineWithContext#require)
	e.ctx.time = now.UTC().Add(e.offset).Truncate(time.Millisecond)

	return e.ctx
}

func (e *memEngine) wlock() *memContext {
	now := time.Now()

	e.ctxMutex.Lock()
	e.isReadLock = false

	// must be UTC and truncated to millis (see engine/pg/pg.go:pgEngineWithContext#require)
	e.ctx.time = now.UTC().Add(e.offset).Truncate(time.Millisecond)

	return e.ctx
}

func (e *memEngine) unlock() {
	if e.isReadLock {
		e.ctxMutex.RUnlock()
	} else {
		e.ctxMutex.Unlock()
	}
}
