package test

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/mem"
	"github.com/gclaussn/go-bpmn/engine/pg"
	"github.com/jackc/pgx/v5"
)

const testWorkerId = "test-worker"

var databaseSchema string

func mustCreateEngines(t *testing.T) ([]engine.Engine, []string) {
	encryptionKey, err := engine.NewEncryptionKey()
	if err != nil {
		t.Fatalf("failed to create encryption key: %v", err)
	}

	encryption, err := engine.NewEncryption(encryptionKey)
	if err != nil {
		t.Fatalf("failed to create encryption: %v", err)
	}

	var engines []engine.Engine
	var engineTypes []string

	// create mem engine
	memEngine, err := mem.New(func(o *mem.Options) {
		o.Encryption = encryption
	})
	if err != nil {
		t.Fatalf("failed to create mem engine: %v", err)
	}

	engines = append(engines, memEngine)
	engineTypes = append(engineTypes, "mem_")

	databaseUrl := lookUpDatabaseUrl()
	if testing.Short() || databaseUrl == "" {
		return engines, engineTypes
	}

	// create pg engine
	if databaseUrl == "" {
		// docker run --rm -d -p 5432:5432 -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=test postgres:15.2-alpine
		databaseUrl = "postgres://postgres:postgres@localhost:5432/test"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		t.Fatalf("failed to establish database connection: %v", err)
	}

	defer conn.Close(ctx)

	if databaseSchema == "" {
		databaseSchema = fmt.Sprintf("test_%s", strings.Replace(time.Now().Format("20060102150405.000"), ".", "", 1))
		_, err = conn.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", databaseSchema))
		if err != nil {
			t.Fatalf("failed to create database schema: %v", err)
		}
	} else {
		for _, table := range pg.Tables {
			_, err = conn.Exec(ctx, fmt.Sprintf("TRUNCATE %s.%s", databaseSchema, table))
			if err != nil {
				t.Fatalf("failed to truncate table %s: %v", table, err)
			}
		}
	}

	databaseUrl = fmt.Sprintf("%s?search_path=%s", databaseUrl, databaseSchema)

	pgEngine, err := pg.New(databaseUrl, func(o *pg.Options) {
		o.Encryption = encryption
		o.TaskExecutorEnabled = false
	})
	if err != nil {
		t.Fatalf("failed to create pg engine: %v", err)
	}

	engines = append(engines, pgEngine)
	engineTypes = append(engineTypes, "pg_")

	return engines, engineTypes
}

func mustCreateProcess(t *testing.T, e engine.Engine, fileName string, bpmnProcessId string, cmd ...engine.CreateProcessCmd) engine.Process {
	bpmnXml := mustReadBpmnFile(t, fileName)

	var c engine.CreateProcessCmd
	if len(cmd) != 0 {
		c = cmd[0]
	} else {
		c = engine.CreateProcessCmd{}
	}

	c.BpmnProcessId = bpmnProcessId
	c.BpmnXml = bpmnXml
	c.WorkerId = testWorkerId

	if c.Version == "" {
		c.Version = t.Name()
	}

	process, err := e.CreateProcess(context.Background(), c)
	if err != nil {
		t.Fatalf("failed to create process: %v", err)
	}

	return process
}

func mustCreateProcessInstance(t *testing.T, e engine.Engine, process engine.Process, cmd ...engine.CreateProcessInstanceCmd) (engine.ProcessInstanceAssert, engine.ElementInstanceAssert) {
	var c engine.CreateProcessInstanceCmd
	if len(cmd) != 0 {
		c = cmd[0]
	} else {
		c = engine.CreateProcessInstanceCmd{}
	}

	c.BpmnProcessId = process.BpmnProcessId
	c.Version = process.Version
	c.WorkerId = process.CreatedBy

	processInstance, err := e.CreateProcessInstance(context.Background(), c)
	if err != nil {
		t.Fatalf("failed to create process instance: %v", err)
	}

	return engine.Assert(t, e, processInstance)
}

func mustReadBpmnFile(t *testing.T, fileName string) string {
	bpmnFile, err := os.Open("../../test/bpmn/" + fileName)
	if err != nil {
		t.Fatalf("failed to open BPMN file: %v", err)
	}

	defer bpmnFile.Close()

	b, err := io.ReadAll(bpmnFile)
	if err != nil {
		t.Fatalf("failed to read BPMN XML: %v", err)
	}

	return string(b)
}

func lookUpDatabaseUrl() string {
	if databaseUrl := os.Getenv("GO_BPMN_TEST_DATABASE_URL"); databaseUrl != "" {
		return databaseUrl
	}
	if _, ok := os.LookupEnv("VSCODE_PID"); ok {
		// assume test database is available when running tests from vscode
		return "postgres://postgres:postgres@localhost:5432/test"
	}
	return ""
}
