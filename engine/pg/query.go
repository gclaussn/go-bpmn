package pg

import (
	"context"

	"github.com/gclaussn/go-bpmn/engine"
)

type query struct {
	e *pgEngine

	defaultQueryLimit int
	options           engine.QueryOptions
}

func (q *query) QueryElements(ctx context.Context, criteria engine.ElementCriteria) ([]engine.Element, error) {
	pgCtx, cancel, err := q.e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	results, err := pgCtx.Elements().Query(criteria, q.options)
	return results, q.e.release(pgCtx, err)
}

func (q *query) QueryElementInstances(ctx context.Context, criteria engine.ElementInstanceCriteria) ([]engine.ElementInstance, error) {
	pgCtx, cancel, err := q.e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	results, err := pgCtx.ElementInstances().Query(criteria, q.options)
	return results, q.e.release(pgCtx, err)
}

func (q *query) QueryIncidents(ctx context.Context, criteria engine.IncidentCriteria) ([]engine.Incident, error) {
	pgCtx, cancel, err := q.e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	results, err := pgCtx.Incidents().Query(criteria, q.options)
	return results, q.e.release(pgCtx, err)
}

func (q *query) QueryJobs(ctx context.Context, criteria engine.JobCriteria) ([]engine.Job, error) {
	pgCtx, cancel, err := q.e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	results, err := pgCtx.Jobs().Query(criteria, q.options)
	return results, q.e.release(pgCtx, err)
}

func (q *query) QueryMessages(ctx context.Context, criteria engine.MessageCriteria) ([]engine.Message, error) {
	pgCtx, cancel, err := q.e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	results, err := pgCtx.Messages().Query(criteria, q.options, pgCtx.Time())
	return results, q.e.release(pgCtx, err)
}

func (q *query) QueryProcesses(ctx context.Context, criteria engine.ProcessCriteria) ([]engine.Process, error) {
	pgCtx, cancel, err := q.e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	results, err := pgCtx.Processes().Query(criteria, q.options)
	return results, q.e.release(pgCtx, err)
}

func (q *query) QueryProcessInstances(ctx context.Context, criteria engine.ProcessInstanceCriteria) ([]engine.ProcessInstance, error) {
	pgCtx, cancel, err := q.e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	results, err := pgCtx.ProcessInstances().Query(criteria, q.options)
	return results, q.e.release(pgCtx, err)
}

func (q *query) QueryTasks(ctx context.Context, criteria engine.TaskCriteria) ([]engine.Task, error) {
	pgCtx, cancel, err := q.e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	results, err := pgCtx.Tasks().Query(criteria, q.options)
	return results, q.e.release(pgCtx, err)
}

func (q *query) QueryVariables(ctx context.Context, criteria engine.VariableCriteria) ([]engine.Variable, error) {
	pgCtx, cancel, err := q.e.acquire(ctx)
	if err != nil {
		return nil, err
	}

	defer cancel()
	results, err := pgCtx.Variables().Query(criteria, q.options)
	return results, q.e.release(pgCtx, err)
}

func (q *query) SetOptions(options engine.QueryOptions) {
	if options.Limit <= 0 {
		options.Limit = q.defaultQueryLimit
	}

	q.options = options
}
