package client

import (
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/server"
)

func newClientQuery(criteria any) clientQuery {
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

type clientQuery func(*clientWithContext, engine.QueryOptions) ([]any, error)

func newElementQuery(criteria engine.ElementCriteria) clientQuery {
	return func(c *clientWithContext, options engine.QueryOptions) ([]any, error) {
		path := server.PathElementsQuery + encodeQueryOptions(options)

		var resBody server.ElementRes
		if err := c.doPost(path, criteria, &resBody); err != nil {
			return nil, err
		}

		results := make([]any, resBody.Count)
		for i, result := range resBody.Results {
			results[i] = result
		}

		return results, nil
	}
}

func newElementInstanceQuery(criteria engine.ElementInstanceCriteria) clientQuery {
	return func(c *clientWithContext, options engine.QueryOptions) ([]any, error) {
		path := server.PathElementInstancesQuery + encodeQueryOptions(options)

		var resBody server.ElementInstanceRes
		if err := c.doPost(path, criteria, &resBody); err != nil {
			return nil, err
		}

		results := make([]any, resBody.Count)
		for i, result := range resBody.Results {
			results[i] = result
		}

		return results, nil
	}
}

func newIncidentQuery(criteria engine.IncidentCriteria) clientQuery {
	return func(c *clientWithContext, options engine.QueryOptions) ([]any, error) {
		path := server.PathIncidentsQuery + encodeQueryOptions(options)

		var resBody server.IncidentRes
		if err := c.doPost(path, criteria, &resBody); err != nil {
			return nil, err
		}

		results := make([]any, resBody.Count)
		for i, result := range resBody.Results {
			results[i] = result
		}

		return results, nil
	}
}

func newJobQuery(criteria engine.JobCriteria) clientQuery {
	return func(c *clientWithContext, options engine.QueryOptions) ([]any, error) {
		path := server.PathJobsQuery + encodeQueryOptions(options)

		var resBody server.JobRes
		if err := c.doPost(path, criteria, &resBody); err != nil {
			return nil, err
		}

		results := make([]any, resBody.Count)
		for i, result := range resBody.Results {
			results[i] = result
		}

		return results, nil
	}
}

func newProcessQuery(criteria engine.ProcessCriteria) clientQuery {
	return func(c *clientWithContext, options engine.QueryOptions) ([]any, error) {
		path := server.PathProcessesQuery + encodeQueryOptions(options)

		var resBody server.ProcessRes
		if err := c.doPost(path, criteria, &resBody); err != nil {
			return nil, err
		}

		results := make([]any, resBody.Count)
		for i, result := range resBody.Results {
			results[i] = result
		}

		return results, nil
	}
}

func newProcessInstanceQuery(criteria engine.ProcessInstanceCriteria) clientQuery {
	return func(c *clientWithContext, options engine.QueryOptions) ([]any, error) {
		path := server.PathProcessInstancesQuery + encodeQueryOptions(options)

		var resBody server.ProcessInstanceRes
		if err := c.doPost(path, criteria, &resBody); err != nil {
			return nil, err
		}

		results := make([]any, resBody.Count)
		for i, result := range resBody.Results {
			results[i] = result
		}

		return results, nil
	}
}

func newTaskQuery(criteria engine.TaskCriteria) clientQuery {
	return func(c *clientWithContext, options engine.QueryOptions) ([]any, error) {
		path := server.PathTasksQuery + encodeQueryOptions(options)

		var resBody server.TaskRes
		if err := c.doPost(path, criteria, &resBody); err != nil {
			return nil, err
		}

		results := make([]any, resBody.Count)
		for i, result := range resBody.Results {
			results[i] = result
		}

		return results, nil
	}
}

func newVariableQuery(criteria engine.VariableCriteria) clientQuery {
	return func(c *clientWithContext, options engine.QueryOptions) ([]any, error) {
		path := server.PathVariablesQuery + encodeQueryOptions(options)

		var resBody server.VariableRes
		if err := c.doPost(path, criteria, &resBody); err != nil {
			return nil, err
		}

		results := make([]any, resBody.Count)
		for i, result := range resBody.Results {
			results[i] = result
		}

		return results, nil
	}
}
