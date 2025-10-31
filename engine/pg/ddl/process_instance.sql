CREATE TABLE IF NOT EXISTS process_instance (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	parent_id INTEGER,
	root_id INTEGER,

	message_id BIGINT,
	process_id INTEGER NOT NULL,

	bpmn_process_id VARCHAR NOT NULL,
	correlation_key VARCHAR,
	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	ended_at TIMESTAMP(3),
	started_at TIMESTAMP(3),
	state VARCHAR NOT NULL,
	tags JSONB,
	version VARCHAR NOT NULL
) PARTITION BY LIST (partition);
