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

func (c *client) CompleteJob(ctx context.Context, cmd engine.CompleteJobCmd) (engine.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	var job engine.Job
	path := resolve(server.PathJobsComplete, cmd.Partition, cmd.Id)
	if err := c.doPatch(ctx, path, cmd, &job); err != nil {
		return engine.Job{}, err
	}
	return job, nil
}

func (c *client) CreateProcess(ctx context.Context, cmd engine.CreateProcessCmd) (engine.Process, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	var process engine.Process
	if err := c.doPost(ctx, server.PathProcesses, cmd, &process); err != nil {
		return engine.Process{}, err
	}
	return process, nil
}

func (c *client) CreateProcessInstance(ctx context.Context, cmd engine.CreateProcessInstanceCmd) (engine.ProcessInstance, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	var processInstance engine.ProcessInstance
	if err := c.doPost(ctx, server.PathProcessInstances, cmd, &processInstance); err != nil {
		return engine.ProcessInstance{}, err
	}
	return processInstance, nil
}

func (c *client) CreateQuery() engine.Query {
	return &query{c: c}
}

func (c *client) ExecuteTasks(ctx context.Context, cmd engine.ExecuteTasksCmd) ([]engine.Task, []engine.Task, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	var resBody server.ExecuteTasksRes
	if err := c.doPost(ctx, server.PathTasksExecute, cmd, &resBody); err != nil {
		return nil, nil, err
	}
	return resBody.CompletedTasks, resBody.FailedTasks, nil
}

func (c *client) GetBpmnXml(ctx context.Context, cmd engine.GetBpmnXmlCmd) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	path := strings.Replace(server.PathProcessesBpmnXml, "{id}", strconv.Itoa(int(cmd.ProcessId)), 1)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+path, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create GET request: %v", err)
	}

	if c.options.OnRequest != nil {
		if err := c.options.OnRequest(req); err != nil {
			return "", err
		}
	}

	req.Header.Add(server.HeaderAuthorization, c.authorization)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute GET %s: %v", c.url+path, err)
	}

	if c.options.OnResponse != nil {
		if err := c.options.OnResponse(res); err != nil {
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

func (c *client) GetElementVariables(ctx context.Context, cmd engine.GetElementVariablesCmd) (map[string]engine.Data, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	path := resolve(server.PathElementInstancesVariables, cmd.Partition, cmd.ElementInstanceId)

	if len(cmd.Names) != 0 {
		path = fmt.Sprintf("%s?%s=%s", path, server.QueryNames, strings.Join(cmd.Names, ","))
	}

	var resBody server.GetVariablesRes
	if err := c.doGet(ctx, path, &resBody); err != nil {
		return nil, err
	}

	return resBody.Variables, nil
}

func (c *client) GetProcessVariables(ctx context.Context, cmd engine.GetProcessVariablesCmd) (map[string]engine.Data, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	path := resolve(server.PathProcessInstancesVariables, cmd.Partition, cmd.ProcessInstanceId)

	if len(cmd.Names) != 0 {
		path = fmt.Sprintf("%s?%s=%s", path, server.QueryNames, strings.Join(cmd.Names, ","))
	}

	var resBody server.GetVariablesRes
	if err := c.doGet(ctx, path, &resBody); err != nil {
		return nil, err
	}

	return resBody.Variables, nil
}

func (c *client) LockJobs(ctx context.Context, cmd engine.LockJobsCmd) ([]engine.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	var resBody server.LockJobsRes
	if err := c.doPost(ctx, server.PathJobsLock, cmd, &resBody); err != nil {
		return nil, err
	}
	return resBody.Jobs, nil
}

func (c *client) ResolveIncident(ctx context.Context, cmd engine.ResolveIncidentCmd) error {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	path := resolve(server.PathIncidentsResolve, cmd.Partition, cmd.Id)
	return c.doPatch(ctx, path, cmd, nil)
}

func (c *client) ResumeProcessInstance(ctx context.Context, cmd engine.ResumeProcessInstanceCmd) error {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	path := resolve(server.PathProcessInstancesResume, cmd.Partition, cmd.Id)
	return c.doPatch(ctx, path, cmd, nil)
}

func (c *client) SendSignal(ctx context.Context, cmd engine.SendSignalCmd) (engine.SignalEvent, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	var signalEvent engine.SignalEvent
	if err := c.doPost(ctx, server.PathEventsSignals, cmd, &signalEvent); err != nil {
		return engine.SignalEvent{}, err
	}
	return signalEvent, nil
}

func (c *client) SetElementVariables(ctx context.Context, cmd engine.SetElementVariablesCmd) error {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	path := resolve(server.PathElementInstancesVariables, cmd.Partition, cmd.ElementInstanceId)
	return c.doPut(ctx, path, cmd, nil)
}

func (c *client) SetProcessVariables(ctx context.Context, cmd engine.SetProcessVariablesCmd) error {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	path := resolve(server.PathProcessInstancesVariables, cmd.Partition, cmd.ProcessInstanceId)
	return c.doPut(ctx, path, cmd, nil)
}

func (c *client) SetTime(ctx context.Context, cmd engine.SetTimeCmd) error {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	return c.doPatch(ctx, server.PathTime, cmd, nil)
}

func (c *client) SuspendProcessInstance(ctx context.Context, cmd engine.SuspendProcessInstanceCmd) error {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	path := resolve(server.PathProcessInstancesSuspend, cmd.Partition, cmd.Id)
	return c.doPatch(ctx, path, cmd, nil)
}

func (c *client) UnlockJobs(ctx context.Context, cmd engine.UnlockJobsCmd) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	var resBody server.CountRes
	if err := c.doPost(ctx, server.PathJobsUnlock, cmd, &resBody); err != nil {
		return -1, err
	}
	return resBody.Count, nil
}

func (c *client) UnlockTasks(ctx context.Context, cmd engine.UnlockTasksCmd) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, c.options.Timeout)
	defer cancel()

	var resBody server.CountRes
	if err := c.doPost(ctx, server.PathTasksUnlock, cmd, &resBody); err != nil {
		return -1, err
	}
	return resBody.Count, nil
}

func (c *client) Shutdown() {
	c.httpClient.CloseIdleConnections()
}

func (c *client) doGet(ctx context.Context, path string, resBody any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create GET request: %v", err)
	}

	if c.options.OnRequest != nil {
		if err := c.options.OnRequest(req); err != nil {
			return err
		}
	}

	req.Header.Add(server.HeaderAuthorization, c.authorization)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute %v", err)
	}

	if c.options.OnResponse != nil {
		if err := c.options.OnResponse(res); err != nil {
			return err
		}
	}

	return decodeJSONResponseBody(res, &resBody)
}

func (c *client) doPatch(ctx context.Context, path string, reqBody any, resBody any) error {
	b, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to create JSON request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.url+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("failed to create PATCH request: %v", err)
	}

	if c.options.OnRequest != nil {
		if err := c.options.OnRequest(req); err != nil {
			return err
		}
	}

	req.Header.Add(server.HeaderAuthorization, c.authorization)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute %v", err)
	}

	if c.options.OnResponse != nil {
		if err := c.options.OnResponse(res); err != nil {
			return err
		}
	}

	if resBody != nil {
		return decodeJSONResponseBody(res, &resBody)
	} else {
		return decodeJSONResponseBody(res, nil)
	}
}

func (c *client) doPost(ctx context.Context, path string, reqBody any, resBody any) error {
	b, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to create JSON request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %v", err)
	}

	if c.options.OnRequest != nil {
		if err := c.options.OnRequest(req); err != nil {
			return err
		}
	}

	req.Header.Add(server.HeaderAuthorization, c.authorization)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute %v", err)
	}

	if c.options.OnResponse != nil {
		if err := c.options.OnResponse(res); err != nil {
			return err
		}
	}

	return decodeJSONResponseBody(res, &resBody)
}

func (c *client) doPut(ctx context.Context, path string, reqBody any, resBody any) error {
	b, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to create JSON request body: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.url+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %v", err)
	}

	if c.options.OnRequest != nil {
		if err := c.options.OnRequest(req); err != nil {
			return err
		}
	}

	req.Header.Add(server.HeaderAuthorization, c.authorization)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute %v", err)
	}

	if c.options.OnResponse != nil {
		if err := c.options.OnResponse(res); err != nil {
			return err
		}
	}

	if resBody != nil {
		return decodeJSONResponseBody(res, &resBody)
	} else {
		return decodeJSONResponseBody(res, nil)
	}
}
