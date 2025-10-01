package mem

import (
	"context"

	"github.com/gclaussn/go-bpmn/engine"
)

type query struct {
	e *memEngine

	defaultQueryLimit int
	options           engine.QueryOptions
}

func (q *query) QueryElements(_ context.Context, criteria engine.ElementCriteria) ([]engine.Element, error) {
	defer q.e.unlock()
	memCtx := q.e.rlock()
	return memCtx.Elements().Query(criteria, q.options)
}

func (q *query) QueryElementInstances(_ context.Context, criteria engine.ElementInstanceCriteria) ([]engine.ElementInstance, error) {
	defer q.e.unlock()
	memCtx := q.e.rlock()
	return memCtx.ElementInstances().Query(criteria, q.options)
}

func (q *query) QueryIncidents(_ context.Context, criteria engine.IncidentCriteria) ([]engine.Incident, error) {
	defer q.e.unlock()
	memCtx := q.e.rlock()
	return memCtx.Incidents().Query(criteria, q.options)
}

func (q *query) QueryJobs(_ context.Context, criteria engine.JobCriteria) ([]engine.Job, error) {
	defer q.e.unlock()
	memCtx := q.e.rlock()
	return memCtx.Jobs().Query(criteria, q.options)
}

func (q *query) QueryMessages(_ context.Context, criteria engine.MessageCriteria) ([]engine.Message, error) {
	defer q.e.unlock()
	memCtx := q.e.rlock()
	return memCtx.Messages().Query(criteria, q.options, memCtx.Time())
}

func (q *query) QueryProcesses(_ context.Context, criteria engine.ProcessCriteria) ([]engine.Process, error) {
	defer q.e.unlock()
	memCtx := q.e.rlock()
	return memCtx.Processes().Query(criteria, q.options)
}

func (q *query) QueryProcessInstances(_ context.Context, criteria engine.ProcessInstanceCriteria) ([]engine.ProcessInstance, error) {
	defer q.e.unlock()
	memCtx := q.e.rlock()
	return memCtx.ProcessInstances().Query(criteria, q.options)
}

func (q *query) QueryTasks(_ context.Context, criteria engine.TaskCriteria) ([]engine.Task, error) {
	defer q.e.unlock()
	memCtx := q.e.rlock()
	return memCtx.Tasks().Query(criteria, q.options)
}

func (q *query) QueryVariables(_ context.Context, criteria engine.VariableCriteria) ([]engine.Variable, error) {
	defer q.e.unlock()
	memCtx := q.e.rlock()
	return memCtx.Variables().Query(criteria, q.options)
}

func (q *query) SetOptions(options engine.QueryOptions) {
	if options.Limit <= 0 {
		options.Limit = q.defaultQueryLimit
	}

	q.options = options
}
