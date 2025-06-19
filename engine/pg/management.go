package pg

import (
	"bufio"
	"bytes"
	"embed"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

const tablePartitionLayout string = "20060102" // Layout, used for formatting table partition dates

var Tables = []string{
	"api_key",
	"element",
	"element_instance",
	"incident",
	"job",
	"process",
	"process_instance",
	"process_instance_queue",
	"process_instance_queue_element",
	"task",
	"variable",
}

var partitionedTables = []string{
	"element_instance",
	"incident",
	"job",
	"process_instance",
	"process_instance_queue_element",
	"task",
	"variable",
}

//go:embed ddl migration sql
var resources embed.FS

// migrateDatabase
func migrateDatabase(ctx *pgContext) error {
	b, err := resources.ReadFile("migration/version.txt")
	if err != nil {
		return fmt.Errorf("failed to read resource migration/version.txt: %v", err)
	}

	versions := make([]string, 0, 1)

	scanner := bufio.NewScanner(bytes.NewReader(b))
	for scanner.Scan() {
		versions = append(versions, scanner.Text())
	}

	schemaVersion, err := selectSchemaVersion(ctx)
	if err != nil {
		return err
	}

	if schemaVersion != "" {
		return nil
	}

	ddl, err := resources.ReadDir("ddl")
	if err != nil {
		return fmt.Errorf("failed to list resources under ddl: %v", err)
	}

	for _, entry := range ddl {
		if entry.IsDir() {
			continue
		}

		name := "ddl/" + entry.Name()
		b, err := resources.ReadFile(name)
		if err != nil {
			return fmt.Errorf("failed to read resource %s: %v", name, err)
		}

		createTable := string(b)
		if _, err := ctx.tx.Exec(ctx.txCtx, createTable); err != nil {
			return fmt.Errorf("failed to execute %s: %v", name, err)
		}
	}

	idx, err := resources.ReadDir("ddl/idx")
	if err != nil {
		return fmt.Errorf("failed to list resources under ddl/idx: %v", err)
	}

	for _, entry := range idx {
		name := "ddl/idx/" + entry.Name()
		b, err := resources.ReadFile(name)
		if err != nil {
			return fmt.Errorf("failed to read resource %s: %v", name, err)
		}

		scanner := bufio.NewScanner(bytes.NewReader(b))
		for scanner.Scan() {
			createIndex := scanner.Text()
			if _, err := ctx.tx.Exec(ctx.txCtx, createIndex); err != nil {
				return fmt.Errorf("failed to execute %s: %v", name, err)
			}
		}
	}

	commentOnTable := fmt.Sprintf("COMMENT ON TABLE task IS %s", quoteString(versions[len(versions)-1]))
	if _, err := ctx.tx.Exec(ctx.txCtx, commentOnTable); err != nil {
		return fmt.Errorf("failed to set schema version: %v", err)
	}

	return nil
}

func partitionSequence(table string, date time.Time) string {
	return fmt.Sprintf("%s_%s_seq", table, date.Format(tablePartitionLayout))
}

func partitionTable(table string, date time.Time) string {
	return fmt.Sprintf("%s_%s", table, date.Format(tablePartitionLayout))
}

// prepareDatabase
func prepareDatabase(ctx *pgContext) error {
	date := ctx.Date()

	createPartitionTasks := []createPartitionTask{
		{Partition: engine.Partition(date), initial: true},
		{Partition: engine.Partition(date.AddDate(0, 0, 1)), initial: true},
		{Partition: engine.Partition(date.AddDate(0, 0, 2))},
	}

	for i := range createPartitionTasks {
		if err := createPartitionTasks[i].Execute(ctx, nil); err != nil {
			return fmt.Errorf("failed to create partition %s: %v", createPartitionTasks[i].Partition, err)
		}
	}

	return nil
}

func selectSchemaVersion(ctx *pgContext) (string, error) {
	row := ctx.tx.QueryRow(ctx.txCtx, `
SELECT
	description
FROM
	pg_description
INNER JOIN
	pg_class
ON
	pg_description.objoid = pg_class.oid
INNER JOIN
	pg_namespace
ON
	pg_class.relnamespace = pg_namespace.oid
WHERE
	nspname = $1 AND
	relname = $2
`, ctx.options.databaseSchema, "task")

	var schemaVersion string
	if err := row.Scan(&schemaVersion); err != nil {
		if err != pgx.ErrNoRows {
			return "", fmt.Errorf("failed to select schema version: %v", err)
		}
	}

	return schemaVersion, nil
}

func selectTablePartitionExists(ctx *pgContext, table string, date time.Time) (bool, error) {
	checkTable := partitionTable(table, date)

	row := ctx.tx.QueryRow(
		ctx.txCtx,
		"SELECT EXISTS(SELECT 1 FROM pg_tables WHERE schemaname = $1 AND tablename = $2)",
		ctx.options.databaseSchema,
		checkTable,
	)

	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("failed to check if table %s exists: %v", checkTable, err)
	}

	return exists, nil
}

// createPartitionTask
type createPartitionTask struct {
	Partition engine.Partition

	initial bool
}

func (t createPartitionTask) Execute(ctx internal.Context, _ *internal.TaskEntity) error {
	pgCtx := ctx.(*pgContext)

	date := time.Time(t.Partition)

	partitionExists, err := selectTablePartitionExists(pgCtx, partitionedTables[0], date)
	if err != nil {
		return fmt.Errorf("failed to check if partition %s exists: %v", date.Format(tablePartitionLayout), err)
	}

	if partitionExists {
		return nil
	}

	for i := range partitionedTables {
		createTable := fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES IN ('%s')",
			partitionTable(partitionedTables[i], date),
			partitionedTables[i],
			date.Format(time.DateOnly),
		)

		if _, err := pgCtx.tx.Exec(pgCtx.txCtx, createTable); err != nil {
			return fmt.Errorf("failed to create table: %v\n%s", err, createTable)
		}

		if partitionedTables[i] == "process_instance_queue_element" {
			continue // no sequence required, since ID of process instance is used
		}

		createSequence := fmt.Sprintf(
			"CREATE SEQUENCE IF NOT EXISTS %s MAXVALUE 2147483647",
			partitionSequence(partitionedTables[i], date),
		)

		if _, err := pgCtx.tx.Exec(pgCtx.txCtx, createSequence); err != nil {
			return fmt.Errorf("failed to create sequence: %v\n%s", err, createSequence)
		}
	}

	if t.initial {
		return nil
	}

	createPartition := internal.TaskEntity{
		Partition: date.AddDate(0, 0, -1),

		CreatedAt: ctx.Time(),
		CreatedBy: ctx.Options().EngineId,
		DueAt:     date.AddDate(0, 0, -1),
		Type:      engine.TaskCreatePartition,

		Instance: createPartitionTask{
			Partition: engine.Partition(date.AddDate(0, 0, 1)),
		},
	}

	if err := ctx.Tasks().Insert(&createPartition); err != nil {
		return err
	}

	detachPartition := internal.TaskEntity{
		Partition: date,

		CreatedAt: ctx.Time(),
		CreatedBy: ctx.Options().EngineId,
		DueAt:     date.Add(5 * time.Minute),
		Type:      engine.TaskDetachPartition,

		Instance: detachPartitionTask{
			Partition: engine.Partition(date.AddDate(0, 0, -2)),
		},
	}

	if err := ctx.Tasks().Insert(&detachPartition); err != nil {
		return err
	}

	return nil
}

// detachPartitionTask
type detachPartitionTask struct {
	Partition engine.Partition
}

func (t detachPartitionTask) Execute(ctx internal.Context, _ *internal.TaskEntity) error {
	pgCtx := ctx.(*pgContext)

	date := time.Time(t.Partition)

	rows, err := pgCtx.tx.Query(pgCtx.txCtx, `
SELECT EXISTS(SELECT 1 FROM process_instance WHERE partition = $1 AND ended_at IS NULL)
UNION ALL
SELECT EXISTS(SELECT 1 FROM task WHERE partition = $1 AND completed_at IS NULL)
`, date)

	if err != nil {
		return err
	}

	var isNotEmpty bool
	for rows.Next() {
		if err := rows.Scan(&isNotEmpty); err != nil {
			rows.Close()
			return err
		}
		if isNotEmpty {
			break
		}
	}

	rows.Close() // must be closed right away to avoid "conn busy" error when inserting retry task

	if isNotEmpty {
		offsetInDays := int(ctx.Time().Sub(date).Hours() / 24)
		offset := 5*time.Minute + time.Minute*time.Duration(offsetInDays)

		retry := internal.TaskEntity{
			Partition: ctx.Date(),

			CreatedAt: ctx.Time(),
			CreatedBy: ctx.Options().EngineId,
			DueAt:     ctx.Date().AddDate(0, 0, 1).Add(offset),
			Type:      engine.TaskDetachPartition,

			Instance: detachPartitionTask{
				Partition: t.Partition,
			},
		}

		return ctx.Tasks().Insert(&retry)
	}

	for i := range partitionedTables {
		alterTable := fmt.Sprintf(
			"ALTER TABLE %s DETACH PARTITION %s",
			partitionedTables[i],
			partitionTable(partitionedTables[i], date),
		)

		if _, err := pgCtx.tx.Exec(pgCtx.txCtx, alterTable); err != nil {
			return fmt.Errorf("failed to alter table: %v\n%s", err, alterTable)
		}
	}

	if pgCtx.options.DropPartitionEnabled {
		dropPartition := internal.TaskEntity{
			Partition: ctx.Date(),

			CreatedAt: ctx.Time(),
			CreatedBy: ctx.Options().EngineId,
			DueAt:     ctx.Time(),
			Type:      engine.TaskDropPartition,

			Instance: dropPartitionTask{
				Partition: t.Partition,
			},
		}

		return ctx.Tasks().Insert(&dropPartition)
	}

	return nil
}

// dropPartitionTask
type dropPartitionTask struct {
	Partition engine.Partition
}

func (t dropPartitionTask) Execute(ctx internal.Context, _ *internal.TaskEntity) error {
	pgCtx := ctx.(*pgContext)

	date := time.Time(t.Partition)

	for i := range partitionedTables {
		dropTable := fmt.Sprintf(
			"DROP TABLE IF EXISTS %s",
			partitionTable(partitionedTables[i], date),
		)

		if _, err := pgCtx.tx.Exec(pgCtx.txCtx, dropTable); err != nil {
			return fmt.Errorf("failed to drop table partition: %v\n%s", err, dropTable)
		}

		if partitionedTables[i] == "process_instance_queue_element" {
			continue // no sequence exists, since ID of process instance is used
		}

		dropSequence := fmt.Sprintf(
			"DROP SEQUENCE IF EXISTS %s",
			partitionSequence(partitionedTables[i], date),
		)

		if _, err := pgCtx.tx.Exec(pgCtx.txCtx, dropSequence); err != nil {
			return fmt.Errorf("failed to drop sequence: %v\n%s", err, dropSequence)
		}
	}

	return nil
}
