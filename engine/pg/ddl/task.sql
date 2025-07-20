CREATE TABLE IF NOT EXISTS task (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	element_id INTEGER,
	element_instance_id INTEGER,
	event_id INTEGER,
	process_id INTEGER,
	process_instance_id INTEGER,

	completed_at TIMESTAMP(3),
	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	due_at TIMESTAMP(3) NOT NULL,
	error VARCHAR,
	locked_at TIMESTAMP(3),
	locked_by VARCHAR,
	retry_count INTEGER NOT NULL,
	retry_timer VARCHAR,
	serialized_task VARCHAR,
	type VARCHAR NOT NULL
) PARTITION BY LIST (partition);
