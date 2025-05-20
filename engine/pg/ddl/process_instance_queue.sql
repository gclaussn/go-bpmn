CREATE TABLE IF NOT EXISTS process_instance_queue (
	bpmn_process_id VARCHAR PRIMARY KEY,

	parallelism INTEGER NOT NULL,
	active_count INTEGER NOT NULL,
	queued_count INTEGER NOT NULL,
	head_partition DATE,
	head_id INTEGER,
	tail_partition DATE,
	tail_id INTEGER
);
