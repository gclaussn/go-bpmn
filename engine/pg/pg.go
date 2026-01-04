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

	for range pgCtxPoolSize {
		pgCtxPool <- &pgContext{options: options, processCache: processCache}
	}

	acquireCtx, acquireCancel := context.WithCancel(context.Background())

	pgEngine := pgEngine{
		acquireCtx:    acquireCtx,
		acquireCancel: acquireCancel,

		pgCtxPool: pgCtxPool,
		pgPool:    pgPool,
		txTimeout: options.Timeout,

		defaultQueryLimit: options.Common.DefaultQueryLimit,
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
			TaskExecutorEnabled:  true,
			TaskExecutorInterval: 60 * time.Second,
			TaskExecutorLimit:    10,
			TaskRetryLimit:       0,
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
	acquireCtx    context.Context    // used to prevent the acquiring of a context, when the engine is shut down
	acquireCancel context.CancelFunc // invoked when a shutdown is initiated
	shutdownOnce  sync.Once          // used to prevent more than one shutdown

	pgCtxPool chan *pgContext
	pgPool    *pgxpool.Pool
	txTimeout time.Duration

	defaultQueryLimit int

	taskExecutor *internal.TaskExecutor

	setTimeMutex sync.Mutex    // used to call SetTime exclusively
	offset       time.Duration // engine time offset
}

func (e *pgEngine) migrateAndPrepareDatabase() error {
	return e.execute(func(pgCtx *pgContext) error {
		if err := migrateDatabase(pgCtx); err != nil {
			return fmt.Errorf("failed to migrate database: %v", err)
		}

		if err := prepareDatabase(pgCtx); err != nil {
			return fmt.Errorf("failed to prepare database: %v", err)
		}

		return nil
	})
}

// execute acquires a pgContext, executes the given function and releases the pgContext.
func (e *pgEngine) execute(fn func(*pgContext) error) error {
	pgCtx, cancel, err := e.acquire(context.Background())
	if err != nil {
		return err
	}

	defer cancel()

	err = fn(pgCtx)
	return e.release(pgCtx, err)
}

func (e *pgEngine) acquire(ctx context.Context) (*pgContext, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(ctx, e.txTimeout)

	now := time.Now()

	select {
	case <-e.acquireCtx.Done():
		cancel()
		return nil, nil, e.acquireCtx.Err()
	case pgCtx := <-e.pgCtxPool:
		tx, err := e.pgPool.Begin(ctx)
		if err != nil {
			cancel()
			e.pgCtxPool <- pgCtx
			return nil, nil, err
		}

		// must be UTC and truncated to millis, since TIMESTAMP(3) is used
		// otherwise tests are flaky
		pgCtx.time = now.UTC().Add(e.offset).Truncate(time.Millisecond)

		pgCtx.tx = tx
		pgCtx.txCtx = ctx

		return pgCtx, cancel, nil
	}
}

func (e *pgEngine) release(pgCtx *pgContext, err error) error {
	if err != nil {
		_ = pgCtx.tx.Rollback(pgCtx.txCtx)
	} else {
		err = pgCtx.tx.Commit(pgCtx.txCtx)
	}

	pgCtx.tx = nil
	pgCtx.txCtx = nil

	e.pgCtxPool <- pgCtx
	return err
}

func (e *pgEngine) CompleteJob(ctx context.Context, cmd engine.CompleteJobCmd) (engine.Job, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return engine.Job{}, err
	}

	defer cancel()
	job, err := internal.CompleteJob(pgCtx, cmd)
	return job, e.release(pgCtx, err)
}

func (e *pgEngine) CreateProcess(ctx context.Context, cmd engine.CreateProcessCmd) (engine.Process, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return engine.Process{}, err
	}

	defer cancel()
	process, err := internal.CreateProcess(pgCtx, cmd)
	return process, e.release(pgCtx, err)
}

func (e *pgEngine) CreateProcessInstance(ctx context.Context, cmd engine.CreateProcessInstanceCmd) (engine.ProcessInstance, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return engine.ProcessInstance{}, err
	}

	defer cancel()
	processInstance, err := internal.CreateProcessInstance(pgCtx, cmd)
	return processInstance, e.release(pgCtx, err)
}

func (e *pgEngine) CreateQuery() engine.Query {
	return &query{
		e: e,

		defaultQueryLimit: e.defaultQueryLimit,
		options:           engine.QueryOptions{Limit: e.defaultQueryLimit},
	}
}

func (e *pgEngine) ExecuteTasks(ctx context.Context, cmd engine.ExecuteTasksCmd) ([]engine.Task, []engine.Task, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return nil, nil, err
	}

	lockedTasks, err := pgCtx.Tasks().Lock(cmd, pgCtx.Time())
	if err := e.release(pgCtx, err); err != nil {
		cancel()
		return nil, nil, err
	}

	cancel()

	completedTasks := make([]engine.Task, 0, len(lockedTasks))
	failedTasks := make([]engine.Task, 0)

	var errs []error
	for _, lockedTask := range lockedTasks {
		pgCtx, cancel, err := e.acquire(ctx)
		if err != nil {
			return nil, nil, err
		}

		defer cancel()

		err = internal.ExecuteTask(pgCtx, lockedTask)

		task := lockedTask.Task()
		if err := e.release(pgCtx, err); err != nil {
			errs = append(errs, fmt.Errorf("failed to execute task %s: %v", task, err))
			if onFailure := pgCtx.options.Common.OnTaskExecutionFailure; onFailure != nil {
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

func (e *pgEngine) GetBpmnXml(ctx context.Context, cmd engine.GetBpmnXmlCmd) (string, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return "", err
	}

	defer cancel()
	bpmnXml, err := internal.GetBpmnXml(pgCtx, cmd)
	return bpmnXml, e.release(pgCtx, err)
}

func (e *pgEngine) GetElementVariables(ctx context.Context, cmd engine.GetElementVariablesCmd) ([]engine.VariableData, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	variables, err := internal.GetElementVariables(pgCtx, cmd)
	return variables, e.release(pgCtx, err)
}

func (e *pgEngine) GetProcessVariables(ctx context.Context, cmd engine.GetProcessVariablesCmd) ([]engine.VariableData, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	variables, err := internal.GetProcessVariables(pgCtx, cmd)
	return variables, e.release(pgCtx, err)
}

func (e *pgEngine) LockJobs(ctx context.Context, cmd engine.LockJobsCmd) ([]engine.Job, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	jobs, err := internal.LockJobs(pgCtx, cmd)
	return jobs, e.release(pgCtx, err)
}

func (e *pgEngine) ResolveIncident(ctx context.Context, cmd engine.ResolveIncidentCmd) error {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return err
	}

	defer cancel()
	err = internal.ResolveIncident(pgCtx, cmd)
	return e.release(pgCtx, err)
}

func (e *pgEngine) ResumeProcessInstance(ctx context.Context, cmd engine.ResumeProcessInstanceCmd) error {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return err
	}

	defer cancel()
	err = internal.ResumeProcessInstance(pgCtx, cmd)
	return e.release(pgCtx, err)
}

func (e *pgEngine) SendMessage(ctx context.Context, cmd engine.SendMessageCmd) (engine.Message, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return engine.Message{}, err
	}

	defer cancel()
	message, err := internal.SendMessage(pgCtx, cmd)
	return message, e.release(pgCtx, err)
}

func (e *pgEngine) SendSignal(ctx context.Context, cmd engine.SendSignalCmd) (engine.Signal, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return engine.Signal{}, err
	}

	defer cancel()
	signal, err := internal.SendSignal(pgCtx, cmd)
	return signal, e.release(pgCtx, err)
}

func (e *pgEngine) SetElementVariables(ctx context.Context, cmd engine.SetElementVariablesCmd) error {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return err
	}

	defer cancel()
	err = internal.SetElementVariables(pgCtx, cmd)
	return e.release(pgCtx, err)
}

func (e *pgEngine) SetProcessVariables(ctx context.Context, cmd engine.SetProcessVariablesCmd) error {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return err
	}

	defer cancel()
	err = internal.SetProcessVariables(pgCtx, cmd)
	return e.release(pgCtx, err)
}

func (e *pgEngine) SetTime(ctx context.Context, cmd engine.SetTimeCmd) error {
	e.setTimeMutex.Lock()
	defer e.setTimeMutex.Unlock()

	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return err
	}

	defer cancel()

	old := pgCtx.Time()
	new := cmd.Time.UTC().Truncate(time.Millisecond)

	sub := new.Sub(old)
	if sub.Milliseconds() < 0 {
		return e.release(pgCtx, engine.Error{
			Type:  engine.ErrorConflict,
			Title: "failed to set time",
			Detail: fmt.Sprintf(
				"time %s is before engine time %s",
				new.Format(time.RFC3339),
				old.Format(time.RFC3339),
			),
		})
	}

	pgCtx.time = new

	err = prepareDatabase(pgCtx)
	if err := e.release(pgCtx, err); err != nil {
		return err
	}

	e.offset = e.offset + sub
	return nil
}

func (e *pgEngine) SuspendProcessInstance(ctx context.Context, cmd engine.SuspendProcessInstanceCmd) error {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return err
	}

	defer cancel()
	err = internal.SuspendProcessInstance(pgCtx, cmd)
	return e.release(pgCtx, err)
}

func (e *pgEngine) UnlockJobs(ctx context.Context, cmd engine.UnlockJobsCmd) (int, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return -1, err
	}

	defer cancel()
	count, err := internal.UnlockJobs(pgCtx, cmd)
	return count, e.release(pgCtx, err)
}

func (e *pgEngine) UnlockTasks(ctx context.Context, cmd engine.UnlockTasksCmd) (int, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return -1, err
	}

	defer cancel()
	count, err := internal.UnlockTasks(pgCtx, cmd)
	return count, e.release(pgCtx, err)
}

func (e *pgEngine) Shutdown() {
	e.shutdownOnce.Do(func() {
		if e.taskExecutor != nil {
			e.taskExecutor.Stop()
		}

		e.acquireCancel()
		e.pgPool.Close()

		for len(e.pgCtxPool) > 0 {
			ctx := <-e.pgCtxPool
			ctx.processCache.Clear()
		}

		close(e.pgCtxPool)
	})
}

// API key manager

func (e *pgEngine) CreateApiKey(ctx context.Context, secretId string) (ApiKey, string, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return ApiKey{}, "", err
	}

	defer cancel()
	apiKey, authorization, err := createApiKey(pgCtx, secretId)
	return apiKey, authorization, e.release(pgCtx, err)
}

func (e *pgEngine) GetApiKey(ctx context.Context, authorization string) (ApiKey, error) {
	pgCtx, cancel, err := e.acquire(ctx)
	if err != nil {
		return ApiKey{}, err
	}

	defer cancel()
	apiKey, err := getApiKey(pgCtx, authorization)
	return apiKey, e.release(pgCtx, err)
}
