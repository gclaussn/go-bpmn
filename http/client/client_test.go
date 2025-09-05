package client

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/mem"
	"github.com/gclaussn/go-bpmn/engine/pg"
	"github.com/gclaussn/go-bpmn/http/server"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/stretchr/testify/assert"
)

func TestClientServer(t *testing.T) {
	assert := assert.New(t)

	e, _ := mem.New()
	defer e.Shutdown()

	s, err := server.New(e, func(o *server.Options) {
		o.BasicAuthUsername = "test"
		o.BasicAuthPassword = "test"

		o.ShutdownDelay = 0
	})
	if err != nil {
		t.Fatalf("failed to create HTTP server: %v", err)
	}

	s.ListenAndServe()
	defer s.Shutdown()

	time.Sleep(time.Millisecond * 1000) // wait for server to start

	authorization := "Basic " + base64.StdEncoding.EncodeToString([]byte("test:test"))

	client, err := New("http://127.0.0.1:8080", authorization)
	if err != nil {
		t.Fatalf("failed to create HTTP client: %v", err)
	}

	defer client.Shutdown()

	// given
	createProcessCmd := engine.CreateProcessCmd{
		BpmnProcessId: "serviceTest",
		BpmnXml:       mustReadBpmnFile(t, "task/service.bpmn"),
		Version:       "1",
		Tags: map[string]string{
			"a": "b",
			"x": "y",
		},
		WorkerId: "test",
	}

	// when
	process, err := client.CreateProcess(context.Background(), createProcessCmd)
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	t.Run("create process", func(t *testing.T) {
		// then
		assert.Equal(engine.Process{
			Id: 1,

			BpmnProcessId: createProcessCmd.BpmnProcessId,
			CreatedAt:     process.CreatedAt,
			CreatedBy:     createProcessCmd.WorkerId,
			Tags: map[string]string{
				"a": "b",
				"x": "y",
			},
			Version: createProcessCmd.Version,
		}, process)

		assert.NotEmpty(process.CreatedAt)
	})

	t.Run("create process with empty command returns 400", func(t *testing.T) {
		// when
		_, err := client.CreateProcess(context.Background(), engine.CreateProcessCmd{})

		// then
		assert.IsTypef(server.Problem{}, err, "expected problem")

		problem := err.(server.Problem)
		assert.Equal(http.StatusBadRequest, problem.Status)
		assert.Equal(server.ProblemTypeHttpRequestBody, problem.Type)
		assert.NotEmpty(problem.Title)
		assert.NotEmpty(problem.Detail)
		assert.NotEmpty(problem.Errors)
	})

	t.Run("query processes", func(t *testing.T) {
		// when
		results, err := client.CreateQuery().QueryProcesses(context.Background(), engine.ProcessCriteria{})
		if err != nil {
			t.Fatalf("failed to query processes %v", err)
		}

		// then
		assert.Len(results, 1)

		assert.Equal(engine.Process{
			Id: process.Id,

			BpmnProcessId: createProcessCmd.BpmnProcessId,
			CreatedAt:     process.CreatedAt,
			CreatedBy:     createProcessCmd.WorkerId,
			Tags:          process.Tags,
			Version:       createProcessCmd.Version,
		}, results[0])

		assert.NotEmpty(process.CreatedAt)
	})

	t.Run("query elements", func(t *testing.T) {
		// when
		results, err := client.CreateQuery().QueryElements(context.Background(), engine.ElementCriteria{})
		if err != nil {
			t.Fatalf("failed to query elements: %v", err)
		}

		// then
		assert.Len(results, 4)

		assert.Equal(engine.Element{
			Id: 1,

			ProcessId: process.Id,

			BpmnElementId:   "serviceTest",
			BpmnElementType: model.ElementProcess,
		}, results[0])
	})

	t.Run("get BPMN XML", func(t *testing.T) {
		// when
		bpmnXml, err := client.GetBpmnXml(context.Background(), engine.GetBpmnXmlCmd{ProcessId: process.Id})
		if err != nil {
			t.Fatalf("failed to get BPMN XML: %v", err)
		}

		// then
		assert.Equal(bpmnXml, createProcessCmd.BpmnXml)
	})

	t.Run("get BPMN XML of not existing process returns 404", func(t *testing.T) {
		// when
		bpmnXml, err := client.GetBpmnXml(context.Background(), engine.GetBpmnXmlCmd{ProcessId: 1_000_000})

		// then
		assert.Empty(bpmnXml)
		assert.IsTypef(engine.Error{}, err, "expected problem")

		engineErr := err.(engine.Error)
		assert.Equal(engine.ErrorNotFound, engineErr.Type)
		assert.NotEmpty(engineErr.Title)
		assert.NotEmpty(engineErr.Detail)
	})

	t.Run("send signal", func(t *testing.T) {
		cmd := engine.SendSignalCmd{
			Name:     "test-signal",
			WorkerId: "test",
		}

		signalEvent, err := client.SendSignal(context.Background(), cmd)
		if err != nil {
			t.Fatalf("failed to send signal: %v", err)
		}

		assert.False(signalEvent.Partition.IsZero())
		assert.NotEmpty(signalEvent.Id)

		assert.NotEmpty(signalEvent.CreatedAt)
		assert.Equal(cmd.WorkerId, signalEvent.CreatedBy)
		assert.Equal(cmd.Name, signalEvent.Name)
		assert.Equal(0, signalEvent.SubscriberCount)
	})
}

func TestClientServerWithApiKeyManager(t *testing.T) {
	assert := assert.New(t)

	e, _ := mem.New()
	defer e.Shutdown()

	s, err := server.New(e, func(o *server.Options) {
		o.ApiKeyManager = &testClientServerApiKeyManager{}

		o.ShutdownDelay = 0
	})
	if err != nil {
		t.Fatalf("failed to create HTTP server: %v", err)
	}

	s.ListenAndServe()
	defer s.Shutdown()

	time.Sleep(time.Millisecond * 1000) // wait for server to start

	t.Run("returns 401 when invalid authorization is configured", func(t *testing.T) {
		authorization := "Basic " + base64.StdEncoding.EncodeToString([]byte("test:test"))

		client, err := New("http://127.0.0.1:8080", authorization)
		if err != nil {
			t.Fatalf("failed to create HTTP client: %v", err)
		}

		defer client.Shutdown()

		_, err = client.CreateProcess(context.Background(), engine.CreateProcessCmd{})
		assert.NotNilf(err, "expected error")
		assert.Contains(err.Error(), "POST /processes: HTTP 401")
	})

	t.Run("returns 400 when valid authorization is configured", func(t *testing.T) {
		client, err := New("http://127.0.0.1:8080", "valid")
		if err != nil {
			t.Fatalf("failed to create HTTP client: %v", err)
		}

		defer client.Shutdown()

		_, err = client.CreateProcess(context.Background(), engine.CreateProcessCmd{})
		assert.IsTypef(server.Problem{}, err, "expected problem")

		problem := err.(server.Problem)
		assert.Equal(http.StatusBadRequest, problem.Status)
	})
}

type testClientServerApiKeyManager struct {
	pg.ApiKeyManager
}

func (a *testClientServerApiKeyManager) GetApiKey(_ context.Context, authorization string) (pg.ApiKey, error) {
	if authorization == "valid" {
		return pg.ApiKey{}, nil
	} else {
		return pg.ApiKey{}, errors.New("invalid")
	}
}
