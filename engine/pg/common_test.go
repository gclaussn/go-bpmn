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

	databaseSchema := fmt.Sprintf("test_%s", strings.Replace(time.Now().Format("20060102150405.000"), ".", "", 1))
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
	w, cancel := e.(*pgEngine).withTimeout()
	defer cancel()

	ctx, err := w.require()
	if err != nil {
		t.Fatalf("failed to require context: %v", err)
	}

	for _, entity := range entities {
		switch entity := entity.(type) {
		case *internal.ElementInstanceEntity:
			err = ctx.ElementInstances().Insert(entity)
		case *internal.ElementEntity:
			err = ctx.Elements().InsertBatch([]*internal.ElementEntity{entity})
		case *internal.IncidentEntity:
			err = ctx.Incidents().Insert(entity)
		case *internal.JobEntity:
			err = ctx.Jobs().Insert(entity)
		case *internal.ProcessEntity:
			err = ctx.Processes().Insert(entity)
		case *internal.ProcessInstanceEntity:
			err = ctx.ProcessInstances().Insert(entity)
		case *internal.TaskEntity:
			err = ctx.Tasks().Insert(entity)
		case *internal.VariableEntity:
			err = ctx.Variables().Insert(entity)
		default:
			w.release(ctx, nil)
			t.Fatalf("unsupported entity type %T", entity)
		}

		if err != nil {
			break
		}
	}

	if err := w.release(ctx, err); err != nil {
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
	w, cancel := e.(*pgEngine).withTimeout()
	defer cancel()

	ctx, err := w.require()
	if err != nil {
		t.Fatalf("failed to require context: %v", err)
	}

	for _, entity := range entities {
		switch entity := entity.(type) {
		case *internal.JobEntity:
			err = ctx.Jobs().Update(entity)
		case *internal.TaskEntity:
			err = ctx.Tasks().Update(entity)
		default:
			w.release(ctx, nil)
			t.Fatalf("unsupported entity type %T", entity)
		}

		if err != nil {
			break
		}
	}

	if err := w.release(ctx, err); err != nil {
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
