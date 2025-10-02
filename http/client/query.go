package client

import (
	"context"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/common"
)

type query struct {
	c *client

	options engine.QueryOptions
}

func (q *query) QueryElements(ctx context.Context, criteria engine.ElementCriteria) ([]engine.Element, error) {
	path := common.PathElementsQuery + encodeQueryOptions(q.options)

	var resBody common.ElementRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryElementInstances(ctx context.Context, criteria engine.ElementInstanceCriteria) ([]engine.ElementInstance, error) {
	path := common.PathElementInstancesQuery + encodeQueryOptions(q.options)

	var resBody common.ElementInstanceRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryIncidents(ctx context.Context, criteria engine.IncidentCriteria) ([]engine.Incident, error) {
	path := common.PathIncidentsQuery + encodeQueryOptions(q.options)

	var resBody common.IncidentRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryJobs(ctx context.Context, criteria engine.JobCriteria) ([]engine.Job, error) {
	path := common.PathJobsQuery + encodeQueryOptions(q.options)

	var resBody common.JobRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryMessages(ctx context.Context, criteria engine.MessageCriteria) ([]engine.Message, error) {
	path := common.PathEventsMessagesQuery + encodeQueryOptions(q.options)

	var resBody common.MessageRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryProcesses(ctx context.Context, criteria engine.ProcessCriteria) ([]engine.Process, error) {
	path := common.PathProcessesQuery + encodeQueryOptions(q.options)

	var resBody common.ProcessRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryProcessInstances(ctx context.Context, criteria engine.ProcessInstanceCriteria) ([]engine.ProcessInstance, error) {
	path := common.PathProcessInstancesQuery + encodeQueryOptions(q.options)

	var resBody common.ProcessInstanceRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryTasks(ctx context.Context, criteria engine.TaskCriteria) ([]engine.Task, error) {
	path := common.PathTasksQuery + encodeQueryOptions(q.options)

	var resBody common.TaskRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) QueryVariables(ctx context.Context, criteria engine.VariableCriteria) ([]engine.Variable, error) {
	path := common.PathVariablesQuery + encodeQueryOptions(q.options)

	var resBody common.VariableRes
	if err := q.c.doPost(ctx, path, criteria, &resBody); err != nil {
		return nil, err
	}

	return resBody.Results, nil
}

func (q *query) SetOptions(options engine.QueryOptions) {
	q.options = options
}
