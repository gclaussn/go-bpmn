// Package pg implements a PostgreSQL based process engine.
/*
pg provides a full implementation of the [engine.Engine] interface, using [github.com/jackc/pgx/v5].

Create an Engine

A pg engine requires a PostgreSQL URL (connection string).
The database URL is parsed via [pgxpool.ParseConfig] - use variable "search_path" to specify a database schema.

	e, err := pg.New("postgres://postgres:postgres@localhost:5432/my_database?search_path=my_schema", func(o *pg.Options) {
		o.Common.EngineId = "my-pg-engine"
	})
	if err != nil {
		log.Fatalf("failed to create pg engine: %v", err)
	}

	defer e.Shutdown()
*/
package pg
