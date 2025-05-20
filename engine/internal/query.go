package internal

import "github.com/gclaussn/go-bpmn/engine"

func NewQuery(criteria any) Query {
	switch criteria := criteria.(type) {
	case engine.ElementCriteria:
		return newElementQuery(criteria)
	case engine.ElementInstanceCriteria:
		return newElementInstanceQuery(criteria)
	case engine.IncidentCriteria:
		return newIncidentQuery(criteria)
	case engine.JobCriteria:
		return newJobQuery(criteria)
	case engine.ProcessCriteria:
		return newProcessQuery(criteria)
	case engine.ProcessInstanceCriteria:
		return newProcessInstanceQuery(criteria)
	case engine.TaskCriteria:
		return newTaskQuery(criteria)
	case engine.VariableCriteria:
		return newVariableQuery(criteria)
	default:
		return nil
	}
}

type Query func(Context, engine.QueryOptions) ([]any, error)

func newElementQuery(criteria engine.ElementCriteria) Query {
	return func(ctx Context, options engine.QueryOptions) ([]any, error) {
		return ctx.Elements().Query(criteria, options)
	}
}

func newElementInstanceQuery(criteria engine.ElementInstanceCriteria) Query {
	return func(ctx Context, options engine.QueryOptions) ([]any, error) {
		return ctx.ElementInstances().Query(criteria, options)
	}
}

func newIncidentQuery(criteria engine.IncidentCriteria) Query {
	return func(ctx Context, options engine.QueryOptions) ([]any, error) {
		return ctx.Incidents().Query(criteria, options)
	}
}

func newJobQuery(criteria engine.JobCriteria) Query {
	return func(ctx Context, options engine.QueryOptions) ([]any, error) {
		return ctx.Jobs().Query(criteria, options)
	}
}

func newProcessQuery(criteria engine.ProcessCriteria) Query {
	return func(ctx Context, options engine.QueryOptions) ([]any, error) {
		return ctx.Processes().Query(criteria, options)
	}
}

func newProcessInstanceQuery(criteria engine.ProcessInstanceCriteria) Query {
	return func(ctx Context, options engine.QueryOptions) ([]any, error) {
		return ctx.ProcessInstances().Query(criteria, options)
	}
}

func newTaskQuery(criteria engine.TaskCriteria) Query {
	return func(ctx Context, options engine.QueryOptions) ([]any, error) {
		return ctx.Tasks().Query(criteria, options)
	}
}

func newVariableQuery(criteria engine.VariableCriteria) Query {
	return func(ctx Context, options engine.QueryOptions) ([]any, error) {
		return ctx.Variables().Query(criteria, options)
	}
}
