CREATE TABLE IF NOT EXISTS job_pool_lock (
	id BIGINT PRIMARY KEY,

	partition DATE NOT NULL,

	element_id INTEGER NOT NULL,
	element_instance_id INTEGER NOT NULL,
	job_id INTEGER NOT NULL,
	process_id INTEGER NOT NULL,
	process_instance_id INTEGER NOT NULL,

	bpmn_element_id VARCHAR NOT NULL,
	bpmn_element_type VARCHAR NOT NULL,
	correlation_key VARCHAR,
	due_at TIMESTAMP(3) NOT NULL,
	locked_at TIMESTAMP(3),
	locked_by VARCHAR,
	retry_count INTEGER NOT NULL,
	type VARCHAR NOT NULL
);
