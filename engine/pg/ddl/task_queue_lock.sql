CREATE TABLE IF NOT EXISTS task_queue_lock (
	id BIGINT PRIMARY KEY,

	partition DATE NOT NULL,
	task_id INTEGER NOT NULL,

	element_id INTEGER,
	element_instance_id INTEGER,
	process_id INTEGER,
	process_instance_id INTEGER,

	bpmn_element_id VARCHAR,
	bpmn_element_type VARCHAR,
	due_at TIMESTAMP(3) NOT NULL,
	locked_at TIMESTAMP(3),
	locked_by VARCHAR,
	retry_count INTEGER NOT NULL,
	serialized_task VARCHAR,
	type VARCHAR NOT NULL
);
