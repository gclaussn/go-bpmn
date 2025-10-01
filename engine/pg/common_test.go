package pg

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

func mustCreateEngine(t *testing.T, customizers ...func(*Options)) engine.Engine {
	if testing.Short() {
		t.Skip()
	}

	databaseUrl := lookUpDatabaseUrl()
	if databaseUrl == "" {
		t.Skip("GO_BPMN_TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, databaseUrl)
	if err != nil {
		t.Fatalf("failed to establish database connection: %v", err)
	}

	defer conn.Close(ctx)

	databaseSchema := fmt.Sprintf("test_pg_%s", strings.Replace(time.Now().Format("20060102150405.000"), ".", "", 1))
	_, err = conn.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", databaseSchema))
	if err != nil {
		t.Fatalf("failed to create database schema: %v", err)
	}

	databaseUrl = fmt.Sprintf("%s?search_path=%s", databaseUrl, databaseSchema)

	customizers = append(customizers, func(o *Options) {
		o.Common.TaskExecutorEnabled = false
	})

	e, err := New(databaseUrl, customizers...)
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	return e
}

func mustInsertEntities(t *testing.T, e engine.Engine, entities []any) {
	pgEngine := e.(*pgEngine)

	pgCtx, cancel, err := pgEngine.acquire(context.Background())
	if err != nil {
		t.Fatalf("failed to require context: %v", err)
	}

	defer cancel()

	for _, entity := range entities {
		switch entity := entity.(type) {
		case *internal.ElementEntity:
			err = pgCtx.Elements().InsertBatch([]*internal.ElementEntity{entity})
		case *internal.ElementInstanceEntity:
			err = pgCtx.ElementInstances().Insert(entity)
		case *internal.IncidentEntity:
			err = pgCtx.Incidents().Insert(entity)
		case *internal.JobEntity:
			err = pgCtx.Jobs().Insert(entity)
		case *internal.MessageEntity:
			err = pgCtx.Messages().Insert(entity)
		case *internal.MessageVariableEntity:
			err = pgCtx.MessageVariables().InsertBatch([]*internal.MessageVariableEntity{entity})
		case *internal.ProcessEntity:
			err = pgCtx.Processes().Insert(entity)
		case *internal.ProcessInstanceEntity:
			err = pgCtx.ProcessInstances().Insert(entity)
		case *internal.SignalEntity:
			err = pgCtx.Signals().Insert(entity)
		case *internal.SignalVariableEntity:
			err = pgCtx.SignalVariables().InsertBatch([]*internal.SignalVariableEntity{entity})
		case *internal.TaskEntity:
			err = pgCtx.Tasks().Insert(entity)
		case *internal.VariableEntity:
			err = pgCtx.Variables().Insert(entity)
		default:
			pgEngine.release(pgCtx, nil)
			t.Fatalf("unsupported entity type %T", entity)
		}

		if err != nil {
			break
		}
	}

	if err := pgEngine.release(pgCtx, err); err != nil {
		t.Fatalf("failed to insert entities: %v", err)
	}
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

func mustUpdateEntities(t *testing.T, e engine.Engine, entities []any) {
	pgEngine := e.(*pgEngine)

	pgCtx, cancel, err := pgEngine.acquire(context.Background())
	if err != nil {
		t.Fatalf("failed to require context: %v", err)
	}

	defer cancel()

	for _, entity := range entities {
		switch entity := entity.(type) {
		case *internal.JobEntity:
			err = pgCtx.Jobs().Update(entity)
		case *internal.TaskEntity:
			err = pgCtx.Tasks().Update(entity)
		default:
			pgEngine.release(pgCtx, nil)
			t.Fatalf("unsupported entity type %T", entity)
		}

		if err != nil {
			break
		}
	}

	if err := pgEngine.release(pgCtx, err); err != nil {
		t.Fatalf("failed to update entities: %v", err)
	}
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
