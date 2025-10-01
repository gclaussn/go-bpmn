package client

import (
	"context"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/server"
)

type query struct {
	c *client

	options engine.QueryOptions
}

func (q *query) QueryElements(ctx context.Context, criteria engine.ElementCriteria) ([]engine.Element, error) {
	path := server.PathElementsQuery + encodeQueryOptions(q.options)

	var resBody server.ElementRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryElementInstances(ctx context.Context, criteria engine.ElementInstanceCriteria) ([]engine.ElementInstance, error) {
	path := server.PathElementInstancesQuery + encodeQueryOptions(q.options)

	var resBody server.ElementInstanceRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryIncidents(ctx context.Context, criteria engine.IncidentCriteria) ([]engine.Incident, error) {
	path := server.PathIncidentsQuery + encodeQueryOptions(q.options)

	var resBody server.IncidentRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryJobs(ctx context.Context, criteria engine.JobCriteria) ([]engine.Job, error) {
	path := server.PathJobsQuery + encodeQueryOptions(q.options)

	var resBody server.JobRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryMessages(ctx context.Context, criteria engine.MessageCriteria) ([]engine.Message, error) {
	path := server.PathEventsMessagesQuery + encodeQueryOptions(q.options)

	var resBody server.MessageRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryProcesses(ctx context.Context, criteria engine.ProcessCriteria) ([]engine.Process, error) {
	path := server.PathProcessesQuery + encodeQueryOptions(q.options)

	var resBody server.ProcessRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryProcessInstances(ctx context.Context, criteria engine.ProcessInstanceCriteria) ([]engine.ProcessInstance, error) {
	path := server.PathProcessInstancesQuery + encodeQueryOptions(q.options)

	var resBody server.ProcessInstanceRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryTasks(ctx context.Context, criteria engine.TaskCriteria) ([]engine.Task, error) {
	path := server.PathTasksQuery + encodeQueryOptions(q.options)

	var resBody server.TaskRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryVariables(ctx context.Context, criteria engine.VariableCriteria) ([]engine.Variable, error) {
	path := server.PathVariablesQuery + encodeQueryOptions(q.options)

	var resBody server.VariableRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) SetOptions(options engine.QueryOptions) {
	q.options = options
}
