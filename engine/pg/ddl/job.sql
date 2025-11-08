CREATE TABLE IF NOT EXISTS job (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	element_id INTEGER NOT NULL,
	element_instance_id INTEGER NOT NULL,
	process_id INTEGER NOT NULL,
	process_instance_id INTEGER NOT NULL,

	bpmn_element_id VARCHAR NOT NULL,
	completed_at TIMESTAMP(3),
	correlation_key VARCHAR,
	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	due_at TIMESTAMP(3) NOT NULL,
	error VARCHAR,
	locked_at TIMESTAMP(3),
	locked_by VARCHAR,
	retry_count INTEGER NOT NULL,
	type VARCHAR NOT NULL
) PARTITION BY LIST (partition);
