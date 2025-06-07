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

	memEngine := memEngine{ctx: ctx}

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

	offset       time.Duration
	taskExecutor *internal.TaskExecutor
}

func (e *memEngine) CompleteJob(cmd engine.CompleteJobCmd) (engine.Job, error) {
	defer e.unlock()
	return internal.CompleteJob(e.wlock(), cmd)
}

func (e *memEngine) CreateProcess(cmd engine.CreateProcessCmd) (engine.Process, error) {
	defer e.unlock()
	return internal.CreateProcess(e.wlock(), cmd)
}

func (e *memEngine) CreateProcessInstance(cmd engine.CreateProcessInstanceCmd) (engine.ProcessInstance, error) {
	defer e.unlock()
	return internal.CreateProcessInstance(e.wlock(), cmd)
}

func (e *memEngine) ExecuteTasks(cmd engine.ExecuteTasksCmd) ([]engine.Task, []engine.Task, error) {
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

func (e *memEngine) GetBpmnXml(cmd engine.GetBpmnXmlCmd) (string, error) {
	defer e.unlock()
	return internal.GetBpmnXml(e.rlock(), cmd)
}

func (e *memEngine) GetElementVariables(cmd engine.GetElementVariablesCmd) (map[string]engine.Data, error) {
	defer e.unlock()
	return internal.GetElementVariables(e.rlock(), cmd)
}

func (e *memEngine) GetProcessVariables(cmd engine.GetProcessVariablesCmd) (map[string]engine.Data, error) {
	defer e.unlock()
	return internal.GetProcessVariables(e.rlock(), cmd)
}

func (e *memEngine) LockJobs(cmd engine.LockJobsCmd) ([]engine.Job, error) {
	defer e.unlock()
	return internal.LockJobs(e.wlock(), cmd)
}

func (e *memEngine) Query(criteria any) ([]any, error) {
	return e.QueryWithOptions(criteria, engine.QueryOptions{})
}

func (e *memEngine) QueryWithOptions(criteria any, options engine.QueryOptions) ([]any, error) {
	query := internal.NewQuery(criteria)
	if query == nil {
		return nil, engine.Error{
			Type:   engine.ErrorQuery,
			Title:  "failed to create query",
			Detail: fmt.Sprintf("unsupported criteria type %T", criteria),
		}
	}

	defer e.unlock()
	ctx := e.rlock()

	if options.Limit <= 0 {
		options.Limit = ctx.Options().DefaultQueryLimit
	}

	return query(ctx, options)
}

func (e *memEngine) ResolveIncident(cmd engine.ResolveIncidentCmd) error {
	defer e.unlock()
	return internal.ResolveIncident(e.wlock(), cmd)
}

func (e *memEngine) ResumeProcessInstance(cmd engine.ResumeProcessInstanceCmd) error {
	defer e.unlock()
	return internal.ResumeProcessInstance(e.wlock(), cmd)
}

func (e *memEngine) SetElementVariables(cmd engine.SetElementVariablesCmd) error {
	defer e.unlock()
	return internal.SetElementVariables(e.wlock(), cmd)
}

func (e *memEngine) SetProcessVariables(cmd engine.SetProcessVariablesCmd) error {
	defer e.unlock()
	return internal.SetProcessVariables(e.wlock(), cmd)
}

func (e *memEngine) SetTime(cmd engine.SetTimeCmd) error {
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

func (e *memEngine) SuspendProcessInstance(cmd engine.SuspendProcessInstanceCmd) error {
	defer e.unlock()
	return internal.SuspendProcessInstance(e.wlock(), cmd)
}

func (e *memEngine) UnlockJobs(cmd engine.UnlockJobsCmd) (int, error) {
	defer e.unlock()
	return internal.UnlockJobs(e.wlock(), cmd)
}

func (e *memEngine) UnlockTasks(cmd engine.UnlockTasksCmd) (int, error) {
	defer e.unlock()
	return internal.UnlockTasks(e.wlock(), cmd)
}

func (e *memEngine) WithContext(_ context.Context) engine.Engine {
	// context is ignored, since there are no context-aware resources involved
	return e
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
