CREATE TABLE IF NOT EXISTS user_task (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	revision INTEGER NOT NULL,

	element_id INTEGER NOT NULL,
	element_instance_id INTEGER NOT NULL,
	process_id INTEGER NOT NULL,
	process_instance_id INTEGER NOT NULL,

	bpmn_element_id VARCHAR NOT NULL,
	correlation_key VARCHAR,
	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	state VARCHAR NOT NULL,
	tags JSONB,
	updated_at TIMESTAMP(3) NOT NULL,
	updated_by VARCHAR NOT NULL
) PARTITION BY LIST (partition);
