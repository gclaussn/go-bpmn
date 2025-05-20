package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/server"
)

func New(url string, authorization string, customizers ...func(*Options)) (engine.Engine, error) {
	if url == "" {
		return nil, errors.New("URL is empty")
	}
	if authorization == "" {
		return nil, errors.New("authorization is empty")
	}

	options := NewOptions()
	for _, customizer := range customizers {
		customizer(&options)
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	httpClient := http.Client{}

	if options.Configure != nil {
		options.Configure(&httpClient)
	}

	client := client{
		httpClient:    &httpClient,
		url:           url,
		authorization: authorization,
		options:       options,
	}

	return &client, nil
}

func NewOptions() Options {
	return Options{
		Timeout: 40 * time.Second,
	}
}

type Options struct {
	Timeout time.Duration // Time limit for requests made by the HTTP client, utilized when no external context is provided.

	// OnRequest is an optional function that accepts a [*http.Request]. It is called before a HTTP request is send.
	OnRequest func(*http.Request) error
	// OnResponse is an optional function that accepts a [*http.Response]. It is called after a HTTP response is returned.
	OnResponse func(*http.Response) error

	Configure func(*http.Client) // Optional function, used to configure the underlying HTTP client.
}

func (o Options) Validate() error {
	return nil
}

type client struct {
	httpClient    *http.Client
	url           string
	authorization string
	options       Options
}

func (c *client) withTimeout() (*clientWithContext, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), c.options.Timeout)
	return &clientWithContext{c: c, ctx: ctx}, cancel
}

type clientWithContext struct {
	c   *client
	ctx context.Context
}

func (c *client) CompleteJob(cmd engine.CompleteJobCmd) (engine.Job, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.CompleteJob(cmd)
}

func (c *clientWithContext) CompleteJob(cmd engine.CompleteJobCmd) (engine.Job, error) {
	var job engine.Job
	path := resolve(server.PathJobsComplete, cmd.Partition, cmd.Id)
	if err := c.doPatch(path, cmd, &job); err != nil {
		return engine.Job{}, err
	}
	return job, nil
}

func (c *client) CreateProcess(cmd engine.CreateProcessCmd) (engine.Process, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.CreateProcess(cmd)
}

func (c *clientWithContext) CreateProcess(cmd engine.CreateProcessCmd) (engine.Process, error) {
	var process engine.Process
	if err := c.doPost(server.PathProcesses, cmd, &process); err != nil {
		return engine.Process{}, err
	}
	return process, nil
}

func (c *client) CreateProcessInstance(cmd engine.CreateProcessInstanceCmd) (engine.ProcessInstance, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.CreateProcessInstance(cmd)
}

func (c *clientWithContext) CreateProcessInstance(cmd engine.CreateProcessInstanceCmd) (engine.ProcessInstance, error) {
	var processInstance engine.ProcessInstance
	if err := c.doPost(server.PathProcessInstances, cmd, &processInstance); err != nil {
		return engine.ProcessInstance{}, err
	}
	return processInstance, nil
}

func (c *client) ExecuteTasks(cmd engine.ExecuteTasksCmd) ([]engine.Task, []engine.Task, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.ExecuteTasks(cmd)
}

func (c *clientWithContext) ExecuteTasks(cmd engine.ExecuteTasksCmd) ([]engine.Task, []engine.Task, error) {
	var resBody server.ExecuteTasksRes
	if err := c.doPost(server.PathTasksExecute, cmd, &resBody); err != nil {
		return nil, nil, err
	}
	return resBody.CompletedTasks, resBody.FailedTasks, nil
}

func (c *client) GetBpmnXml(cmd engine.GetBpmnXmlCmd) (string, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.GetBpmnXml(cmd)
}

func (c *clientWithContext) GetBpmnXml(cmd engine.GetBpmnXmlCmd) (string, error) {
	client := c.c

	path := strings.Replace(server.PathProcessesBpmnXml, "{id}", strconv.Itoa(int(cmd.ProcessId)), 1)

	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, client.url+path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create GET request: %v", err)
	}

	if client.options.OnRequest != nil {
		if err := client.options.OnRequest(req); err != nil {
			return "", err
		}
	}

	req.Header.Add(server.HeaderAuthorization, client.authorization)

	res, err := client.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute GET %s: %v", client.url+path, err)
	}

	if client.options.OnResponse != nil {
		if err := client.options.OnResponse(res); err != nil {
			return "", err
		}
	}

	contentType := res.Header.Get(server.HeaderContentType)
	if contentType != server.ContentTypeXml {
		return "", decodeJSONResponseBody(res, nil)
	}

	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	return string(b), nil
}

func (c *client) GetElementVariables(cmd engine.GetElementVariablesCmd) (map[string]engine.Data, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.GetElementVariables(cmd)
}

func (c *clientWithContext) GetElementVariables(cmd engine.GetElementVariablesCmd) (map[string]engine.Data, error) {
	path := resolve(server.PathElementInstancesVariables, cmd.Partition, cmd.ElementInstanceId)

	if len(cmd.Names) != 0 {
		path = fmt.Sprintf("%s?%s=%s", path, server.QueryNames, strings.Join(cmd.Names, ","))
	}

	var resBody server.GetVariablesRes
	if err := c.doGet(path, &resBody); err != nil {
		return nil, err
	}

	return resBody.Variables, nil
}

func (c *client) GetProcessVariables(cmd engine.GetProcessVariablesCmd) (map[string]engine.Data, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.GetProcessVariables(cmd)
}

func (c *clientWithContext) GetProcessVariables(cmd engine.GetProcessVariablesCmd) (map[string]engine.Data, error) {
	path := resolve(server.PathProcessInstancesVariables, cmd.Partition, cmd.ProcessInstanceId)

	if len(cmd.Names) != 0 {
		path = fmt.Sprintf("%s?%s=%s", path, server.QueryNames, strings.Join(cmd.Names, ","))
	}

	var resBody server.GetVariablesRes
	if err := c.doGet(path, &resBody); err != nil {
		return nil, err
	}

	return resBody.Variables, nil
}

func (c *client) LockJobs(cmd engine.LockJobsCmd) ([]engine.Job, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.LockJobs(cmd)
}

func (c *clientWithContext) LockJobs(cmd engine.LockJobsCmd) ([]engine.Job, error) {
	var resBody server.LockJobsRes
	if err := c.doPost(server.PathJobsLock, cmd, &resBody); err != nil {
		return nil, err
	}
	return resBody.Jobs, nil
}

func (c *client) Query(criteria any) ([]any, error) {
	return c.QueryWithOptions(criteria, engine.QueryOptions{})
}

func (c *clientWithContext) Query(criteria any) ([]any, error) {
	return c.QueryWithOptions(criteria, engine.QueryOptions{})
}

func (c *client) QueryWithOptions(criteria any, options engine.QueryOptions) ([]any, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.QueryWithOptions(criteria, options)
}

func (c *clientWithContext) QueryWithOptions(criteria any, options engine.QueryOptions) ([]any, error) {
	query := newClientQuery(criteria)
	if query == nil {
		return nil, engine.Error{
			Type:   engine.ErrorQuery,
			Title:  "failed to create query",
			Detail: fmt.Sprintf("unsupported criteria type %T", criteria),
		}
	}

	return query(c, options)
}

func (c *client) ResolveIncident(cmd engine.ResolveIncidentCmd) error {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.ResolveIncident(cmd)
}

func (c *clientWithContext) ResolveIncident(cmd engine.ResolveIncidentCmd) error {
	path := resolve(server.PathIncidentsResolve, cmd.Partition, cmd.Id)
	return c.doPatch(path, cmd, nil)
}

func (c *client) ResumeProcessInstance(cmd engine.ResumeProcessInstanceCmd) error {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.ResumeProcessInstance(cmd)
}

func (c *clientWithContext) ResumeProcessInstance(cmd engine.ResumeProcessInstanceCmd) error {
	path := resolve(server.PathProcessInstancesResume, cmd.Partition, cmd.Id)
	return c.doPatch(path, cmd, nil)
}

func (c *client) SetElementVariables(cmd engine.SetElementVariablesCmd) error {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.SetElementVariables(cmd)
}

func (c *clientWithContext) SetElementVariables(cmd engine.SetElementVariablesCmd) error {
	path := resolve(server.PathElementInstancesVariables, cmd.Partition, cmd.ElementInstanceId)
	return c.doPut(path, cmd, nil)
}

func (c *client) SetProcessVariables(cmd engine.SetProcessVariablesCmd) error {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.SetProcessVariables(cmd)
}

func (c *clientWithContext) SetProcessVariables(cmd engine.SetProcessVariablesCmd) error {
	path := resolve(server.PathProcessInstancesVariables, cmd.Partition, cmd.ProcessInstanceId)
	return c.doPut(path, cmd, nil)
}

func (c *client) SetTime(cmd engine.SetTimeCmd) error {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.SetTime(cmd)
}

func (c *clientWithContext) SetTime(cmd engine.SetTimeCmd) error {
	return c.doPatch(server.PathTime, cmd, nil)
}

func (c *client) SuspendProcessInstance(cmd engine.SuspendProcessInstanceCmd) error {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.SuspendProcessInstance(cmd)
}

func (c *clientWithContext) SuspendProcessInstance(cmd engine.SuspendProcessInstanceCmd) error {
	path := resolve(server.PathProcessInstancesSuspend, cmd.Partition, cmd.Id)
	return c.doPatch(path, cmd, nil)
}

func (c *client) UnlockJobs(cmd engine.UnlockJobsCmd) (int, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.UnlockJobs(cmd)
}

func (c *clientWithContext) UnlockJobs(cmd engine.UnlockJobsCmd) (int, error) {
	var resBody server.CountRes
	if err := c.doPost(server.PathJobsUnlock, cmd, &resBody); err != nil {
		return -1, err
	}
	return resBody.Count, nil
}

func (c *client) UnlockTasks(cmd engine.UnlockTasksCmd) (int, error) {
	w, cancel := c.withTimeout()
	defer cancel()
	return w.UnlockTasks(cmd)
}

func (c *clientWithContext) UnlockTasks(cmd engine.UnlockTasksCmd) (int, error) {
	var resBody server.CountRes
	if err := c.doPost(server.PathTasksUnlock, cmd, &resBody); err != nil {
		return -1, err
	}
	return resBody.Count, nil
}

func (c *client) WithContext(ctx context.Context) engine.Engine {
	return &clientWithContext{c, ctx}
}

func (c *clientWithContext) WithContext(ctx context.Context) engine.Engine {
	return &clientWithContext{c.c, ctx}
}

func (c *client) Shutdown() {
	c.httpClient.CloseIdleConnections()
}

func (c *clientWithContext) Shutdown() {
	c.c.Shutdown()
}

func (c *clientWithContext) doGet(path string, resBody any) error {
	client := c.c

	req, err := http.NewRequestWithContext(c.ctx, http.MethodGet, client.url+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %v", err)
	}

	if client.options.OnRequest != nil {
		if err := client.options.OnRequest(req); err != nil {
			return err
		}
	}

	req.Header.Add(server.HeaderAuthorization, client.authorization)

	res, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute %v", err)
	}

	if client.options.OnResponse != nil {
		if err := client.options.OnResponse(res); err != nil {
			return err
		}
	}

	return decodeJSONResponseBody(res, &resBody)
}

func (c *clientWithContext) doPatch(path string, reqBody any, resBody any) error {
	client := c.c

	b, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to create JSON request body: %v", err)
	}

	req, err := http.NewRequestWithContext(c.ctx, http.MethodPatch, client.url+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("failed to create PATCH request: %v", err)
	}

	if c.c.options.OnRequest != nil {
		if err := client.options.OnRequest(req); err != nil {
			return err
		}
	}

	req.Header.Add(server.HeaderAuthorization, client.authorization)

	res, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute %v", err)
	}

	if client.options.OnResponse != nil {
		if err := client.options.OnResponse(res); err != nil {
			return err
		}
	}

	if resBody != nil {
		return decodeJSONResponseBody(res, &resBody)
	} else {
		return decodeJSONResponseBody(res, nil)
	}
}

func (c *clientWithContext) doPost(path string, reqBody any, resBody any) error {
	client := c.c

	b, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to create JSON request body: %v", err)
	}

	req, err := http.NewRequestWithContext(c.ctx, http.MethodPost, client.url+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %v", err)
	}

	if client.options.OnRequest != nil {
		if err := client.options.OnRequest(req); err != nil {
			return err
		}
	}

	req.Header.Add(server.HeaderAuthorization, client.authorization)

	res, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute %v", err)
	}

	if client.options.OnResponse != nil {
		if err := client.options.OnResponse(res); err != nil {
			return err
		}
	}

	return decodeJSONResponseBody(res, &resBody)
}

func (c *clientWithContext) doPut(path string, reqBody any, resBody any) error {
	client := c.c

	b, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to create JSON request body: %v", err)
	}

	req, err := http.NewRequestWithContext(c.ctx, http.MethodPut, client.url+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %v", err)
	}

	if client.options.OnRequest != nil {
		if err := client.options.OnRequest(req); err != nil {
			return err
		}
	}

	req.Header.Add(server.HeaderAuthorization, client.authorization)

	res, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute %v", err)
	}

	if client.options.OnResponse != nil {
		if err := client.options.OnResponse(res); err != nil {
			return err
		}
	}

	if resBody != nil {
		return decodeJSONResponseBody(res, &resBody)
	} else {
		return decodeJSONResponseBody(res, nil)
	}
}
