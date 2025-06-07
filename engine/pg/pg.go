package pg

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(databaseUrl string, customizers ...func(*Options)) (engine.Engine, error) {
	if databaseUrl == "" {
		return nil, errors.New("database URL is empty")
	}

	options := NewOptions()
	for _, customizer := range customizers {
		customizer(&options)
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	pgPoolConfig, err := pgxpool.ParseConfig(databaseUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %v", err)
	}

	if _, ok := pgPoolConfig.ConnConfig.RuntimeParams["application_name"]; !ok {
		pgPoolConfig.ConnConfig.RuntimeParams["application_name"] = options.Common.EngineId
	}

	if databaseSchema, ok := pgPoolConfig.ConnConfig.RuntimeParams["search_path"]; ok {
		options.databaseSchema = databaseSchema
	}

	pgPoolCtx, pgPoolCancel := context.WithTimeout(context.Background(), options.Timeout)
	defer pgPoolCancel()

	pgPool, err := pgxpool.NewWithConfig(pgPoolCtx, pgPoolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create database connection pool: %v", err)
	}

	pgCtxPoolSize := int(pgPoolConfig.MaxConns)
	pgCtxPool := make(chan *pgContext, pgCtxPoolSize)

	processCache := internal.NewProcessCache()

	for i := 0; i < pgCtxPoolSize; i++ {
		pgCtxPool <- &pgContext{options: options, processCache: processCache}
	}

	requireCtx, requireCancel := context.WithCancel(context.Background())

	pgEngine := pgEngine{
		requireCtx:    requireCtx,
		requireCancel: requireCancel,

		pgCtxPool: pgCtxPool,
		pgPool:    pgPool,
		txTimeout: options.Timeout,
	}

	if err := pgEngine.migrateAndPrepareDatabase(); err != nil {
		pgEngine.Shutdown()
		return nil, fmt.Errorf("failed to migrate and prepare database: %v", err)
	}

	if options.Common.TaskExecutorEnabled {
		pgEngine.taskExecutor = internal.NewTaskExecutor(
			&pgEngine,
			options.Common.TaskExecutorInterval,
			options.Common.TaskExecutorLimit,
		)

		pgEngine.taskExecutor.Execute()
	}

	return &pgEngine, nil
}

func NewOptions() Options {
	return Options{
		Common: engine.Options{
			DefaultQueryLimit:    1000,
			EngineId:             engine.DefaultEngineId,
			JobRetryCount:        1,
			TaskExecutorEnabled:  true,
			TaskExecutorInterval: 60 * time.Second,
			TaskExecutorLimit:    10,
		},

		DropPartitionEnabled: true,
		Timeout:              30 * time.Second,

		databaseSchema: "public",
	}
}

type Options struct {
	Common engine.Options // Common engine options.

	DropPartitionEnabled bool
	Timeout              time.Duration // Time limit for database transactions, utilized when no external context is provided.

	databaseSchema string // derived from database URL - see runtime parameter "search_path"
}

func (o Options) Validate() error {
	return o.Common.Validate()
}

type pgEngine struct {
	requireCtx    context.Context    // used to prevent the requiring of a context, when the engine is shut down
	requireCancel context.CancelFunc // invoked when a shutdown is initiated
	shutdownOnce  sync.Once          // used to prevent more than one shutdown

	pgCtxPool chan *pgContext
	pgPool    *pgxpool.Pool
	txTimeout time.Duration // utilized when no external context is provided

	taskExecutor *internal.TaskExecutor

	setTimeMutex sync.Mutex    // used to call SetTime exclusively
	offset       time.Duration // engine time offset
}

func (e *pgEngine) migrateAndPrepareDatabase() error {
	w, cancel := e.withTimeout()
	defer cancel()

	ctx, err := w.require()
	if err != nil {
		return err
	}

	if err := migrateDatabase(ctx); err != nil {
		return w.release(ctx, fmt.Errorf("failed to migrate database: %v", err))
	}

	if err := prepareDatabase(ctx); err != nil {
		return w.release(ctx, fmt.Errorf("failed to prepare database: %v", err))
	}

	return w.release(ctx, err)
}

// withTimeout wraps the engine using a context-aware implementation.
// The context is created with a configurable timeout.
func (e *pgEngine) withTimeout() (*pgEngineWithContext, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), e.txTimeout)
	return &pgEngineWithContext{e: e, ctx: ctx}, cancel
}

// pgEngineWithContext is wrapper that is used with an external context.
type pgEngineWithContext struct {
	e   *pgEngine       // wrapped engine
	ctx context.Context // external context
}

func (e *pgEngineWithContext) require() (*pgContext, error) {
	now := time.Now()

	pgEngine := e.e

	select {
	case <-pgEngine.requireCtx.Done():
		return nil, pgEngine.requireCtx.Err()
	case pgCtx := <-pgEngine.pgCtxPool:
		tx, err := pgEngine.pgPool.Begin(e.ctx)
		if err != nil {
			pgEngine.pgCtxPool <- pgCtx
			return nil, err
		}

		// must be UTC and truncated to millis, since TIMESTAMP(3) is used
		// otherwise tests are flaky
		pgCtx.time = now.UTC().Add(pgEngine.offset).Truncate(time.Millisecond)

		pgCtx.tx = tx
		pgCtx.txCtx = e.ctx

		return pgCtx, nil
	}
}

func (e *pgEngineWithContext) release(pgCtx *pgContext, err error) error {
	if err != nil {
		_ = pgCtx.tx.Rollback(pgCtx.txCtx)
	} else {
		err = pgCtx.tx.Commit(pgCtx.txCtx)
	}

	pgCtx.tx = nil
	pgCtx.txCtx = nil

	e.e.pgCtxPool <- pgCtx
	return err
}

func (e *pgEngine) CompleteJob(cmd engine.CompleteJobCmd) (engine.Job, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.CompleteJob(cmd)
}

func (e *pgEngineWithContext) CompleteJob(cmd engine.CompleteJobCmd) (engine.Job, error) {
	ctx, err := e.require()
	if err != nil {
		return engine.Job{}, err
	}

	job, err := internal.CompleteJob(ctx, cmd)
	return job, e.release(ctx, err)
}

func (e *pgEngine) CreateProcess(cmd engine.CreateProcessCmd) (engine.Process, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.CreateProcess(cmd)
}

func (e *pgEngineWithContext) CreateProcess(cmd engine.CreateProcessCmd) (engine.Process, error) {
	ctx, err := e.require()
	if err != nil {
		return engine.Process{}, err
	}

	process, err := internal.CreateProcess(ctx, cmd)
	return process, e.release(ctx, err)
}

func (e *pgEngine) CreateProcessInstance(cmd engine.CreateProcessInstanceCmd) (engine.ProcessInstance, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.CreateProcessInstance(cmd)
}

func (e *pgEngineWithContext) CreateProcessInstance(cmd engine.CreateProcessInstanceCmd) (engine.ProcessInstance, error) {
	ctx, err := e.require()
	if err != nil {
		return engine.ProcessInstance{}, err
	}

	processInstance, err := internal.CreateProcessInstance(ctx, cmd)
	return processInstance, e.release(ctx, err)
}

func (e *pgEngine) ExecuteTasks(cmd engine.ExecuteTasksCmd) ([]engine.Task, []engine.Task, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.ExecuteTasks(cmd)
}

func (e *pgEngineWithContext) ExecuteTasks(cmd engine.ExecuteTasksCmd) ([]engine.Task, []engine.Task, error) {
	ctx, err := e.require()
	if err != nil {
		return nil, nil, err
	}

	lockedTasks, err := ctx.Tasks().Lock(cmd, ctx.Time())
	if err := e.release(ctx, err); err != nil {
		return nil, nil, err
	}

	var (
		completedTasks []engine.Task
		failedTasks    []engine.Task

		errs []error
	)
	for _, lockedTask := range lockedTasks {
		ctx, err := e.require()
		if err != nil {
			return nil, nil, err
		}

		err = internal.ExecuteTask(ctx, lockedTask)

		task := lockedTask.Task()
		if err := e.release(ctx, err); err != nil {
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

func (e *pgEngine) GetBpmnXml(cmd engine.GetBpmnXmlCmd) (string, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.GetBpmnXml(cmd)
}

func (e *pgEngineWithContext) GetBpmnXml(cmd engine.GetBpmnXmlCmd) (string, error) {
	ctx, err := e.require()
	if err != nil {
		return "", err
	}

	bpmnXml, err := internal.GetBpmnXml(ctx, cmd)
	return bpmnXml, e.release(ctx, err)
}

func (e *pgEngine) GetElementVariables(cmd engine.GetElementVariablesCmd) (map[string]engine.Data, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.GetElementVariables(cmd)
}

func (e *pgEngineWithContext) GetElementVariables(cmd engine.GetElementVariablesCmd) (map[string]engine.Data, error) {
	ctx, err := e.require()
	if err != nil {
		return nil, err
	}

	variables, err := internal.GetElementVariables(ctx, cmd)
	return variables, e.release(ctx, err)
}

func (e *pgEngine) GetProcessVariables(cmd engine.GetProcessVariablesCmd) (map[string]engine.Data, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.GetProcessVariables(cmd)
}

func (e *pgEngineWithContext) GetProcessVariables(cmd engine.GetProcessVariablesCmd) (map[string]engine.Data, error) {
	ctx, err := e.require()
	if err != nil {
		return nil, err
	}

	variables, err := internal.GetProcessVariables(ctx, cmd)
	return variables, e.release(ctx, err)
}

func (e *pgEngine) LockJobs(cmd engine.LockJobsCmd) ([]engine.Job, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.LockJobs(cmd)
}

func (e *pgEngineWithContext) LockJobs(cmd engine.LockJobsCmd) ([]engine.Job, error) {
	ctx, err := e.require()
	if err != nil {
		return nil, err
	}

	jobs, err := internal.LockJobs(ctx, cmd)
	return jobs, e.release(ctx, err)
}

func (e *pgEngine) Query(criteria any) ([]any, error) {
	return e.QueryWithOptions(criteria, engine.QueryOptions{})
}

func (e *pgEngineWithContext) Query(criteria any) ([]any, error) {
	return e.QueryWithOptions(criteria, engine.QueryOptions{})
}

func (e *pgEngine) QueryWithOptions(criteria any, options engine.QueryOptions) ([]any, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.QueryWithOptions(criteria, options)
}

func (e *pgEngineWithContext) QueryWithOptions(criteria any, options engine.QueryOptions) ([]any, error) {
	query := internal.NewQuery(criteria)
	if query == nil {
		return nil, engine.Error{
			Type:   engine.ErrorQuery,
			Title:  "failed to create query",
			Detail: fmt.Sprintf("unsupported criteria type %T", criteria),
		}
	}

	ctx, err := e.require()
	if err != nil {
		return nil, err
	}

	if options.Limit <= 0 {
		options.Limit = ctx.Options().DefaultQueryLimit
	}

	results, err := query(ctx, options)
	return results, e.release(ctx, err)
}

func (e *pgEngine) ResolveIncident(cmd engine.ResolveIncidentCmd) error {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.ResolveIncident(cmd)
}

func (e *pgEngineWithContext) ResolveIncident(cmd engine.ResolveIncidentCmd) error {
	ctx, err := e.require()
	if err != nil {
		return err
	}

	err = internal.ResolveIncident(ctx, cmd)
	return e.release(ctx, err)
}

func (e *pgEngine) ResumeProcessInstance(cmd engine.ResumeProcessInstanceCmd) error {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.ResumeProcessInstance(cmd)
}

func (e *pgEngineWithContext) ResumeProcessInstance(cmd engine.ResumeProcessInstanceCmd) error {
	ctx, err := e.require()
	if err != nil {
		return err
	}

	err = internal.ResumeProcessInstance(ctx, cmd)
	return e.release(ctx, err)
}

func (e *pgEngine) SetElementVariables(cmd engine.SetElementVariablesCmd) error {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.SetElementVariables(cmd)
}

func (e *pgEngineWithContext) SetElementVariables(cmd engine.SetElementVariablesCmd) error {
	ctx, err := e.require()
	if err != nil {
		return err
	}

	err = internal.SetElementVariables(ctx, cmd)
	return e.release(ctx, err)
}

func (e *pgEngine) SetProcessVariables(cmd engine.SetProcessVariablesCmd) error {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.SetProcessVariables(cmd)
}

func (e *pgEngineWithContext) SetProcessVariables(cmd engine.SetProcessVariablesCmd) error {
	ctx, err := e.require()
	if err != nil {
		return err
	}

	err = internal.SetProcessVariables(ctx, cmd)
	return e.release(ctx, err)
}

func (e *pgEngine) SetTime(cmd engine.SetTimeCmd) error {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.SetTime(cmd)
}

func (e *pgEngineWithContext) SetTime(cmd engine.SetTimeCmd) error {
	pgEngine := e.e

	pgEngine.setTimeMutex.Lock()
	defer pgEngine.setTimeMutex.Unlock()

	ctx, err := e.require()
	if err != nil {
		return err
	}

	old := ctx.Time()
	new := cmd.Time.UTC().Truncate(time.Millisecond)

	sub := new.Sub(old)
	if sub.Milliseconds() < 0 {
		return e.release(ctx, engine.Error{
			Type:  engine.ErrorConflict,
			Title: "failed to set time",
			Detail: fmt.Sprintf(
				"time %s is before engine time %s",
				new.Format(time.RFC3339),
				old.Format(time.RFC3339),
			),
		})
	}

	ctx.time = new

	err = prepareDatabase(ctx)
	if err := e.release(ctx, err); err != nil {
		return err
	}

	pgEngine.offset = pgEngine.offset + sub
	return nil
}

func (e *pgEngine) SuspendProcessInstance(cmd engine.SuspendProcessInstanceCmd) error {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.SuspendProcessInstance(cmd)
}

func (e *pgEngineWithContext) SuspendProcessInstance(cmd engine.SuspendProcessInstanceCmd) error {
	ctx, err := e.require()
	if err != nil {
		return err
	}

	err = internal.SuspendProcessInstance(ctx, cmd)
	return e.release(ctx, err)
}

func (e *pgEngine) UnlockJobs(cmd engine.UnlockJobsCmd) (int, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.UnlockJobs(cmd)
}

func (e *pgEngineWithContext) UnlockJobs(cmd engine.UnlockJobsCmd) (int, error) {
	ctx, err := e.require()
	if err != nil {
		return -1, err
	}

	count, err := internal.UnlockJobs(ctx, cmd)
	return count, e.release(ctx, err)
}

func (e *pgEngine) UnlockTasks(cmd engine.UnlockTasksCmd) (int, error) {
	w, cancel := e.withTimeout()
	defer cancel()
	return w.UnlockTasks(cmd)
}

func (e *pgEngineWithContext) UnlockTasks(cmd engine.UnlockTasksCmd) (int, error) {
	ctx, err := e.require()
	if err != nil {
		return -1, err
	}

	count, err := internal.UnlockTasks(ctx, cmd)
	return count, e.release(ctx, err)
}

func (e *pgEngine) WithContext(ctx context.Context) engine.Engine {
	return &pgEngineWithContext{e, ctx}
}

func (e *pgEngineWithContext) WithContext(ctx context.Context) engine.Engine {
	return &pgEngineWithContext{e.e, ctx}
}

func (e *pgEngine) Shutdown() {
	e.shutdownOnce.Do(func() {
		if e.taskExecutor != nil {
			e.taskExecutor.Stop()
		}

		e.requireCancel()
		e.pgPool.Close()

		for len(e.pgCtxPool) > 0 {
			ctx := <-e.pgCtxPool
			ctx.processCache.Clear()
		}

		close(e.pgCtxPool)
	})
}

func (e *pgEngineWithContext) Shutdown() {
	e.e.Shutdown()
}

// API key manager

func (e *pgEngine) CreateApiKey(secretId string) (ApiKey, string, error) {
	w, cancel := e.withTimeout()
	defer cancel()

	ctx, err := w.require()
	if err != nil {
		return ApiKey{}, "", err
	}

	apiKey, authorization, err := createApiKey(ctx, secretId)
	return apiKey, authorization, w.release(ctx, err)
}

func (e *pgEngine) GetApiKey(authorization string) (ApiKey, error) {
	w, cancel := e.withTimeout()
	defer cancel()

	ctx, err := w.require()
	if err != nil {
		return ApiKey{}, err
	}

	apiKey, err := getApiKey(ctx, authorization)
	return apiKey, w.release(ctx, err)
}
