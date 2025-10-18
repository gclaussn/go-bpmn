CREATE TABLE IF NOT EXISTS element_instance (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	parent_id INTEGER,
	prev_element_id INTEGER,
	prev_id INTEGER,

	element_id INTEGER NOT NULL,
	process_id INTEGER NOT NULL,
	process_instance_id INTEGER NOT NULL,

	bpmn_element_id VARCHAR NOT NULL,
	bpmn_element_type VARCHAR NOT NULL,
	context VARCHAR,
	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	ended_at TIMESTAMP(3),
	execution_count INTEGER NOT NULL,
	is_multi_instance BOOLEAN NOT NULL,
	started_at TIMESTAMP(3),
	state VARCHAR NOT NULL
) PARTITION BY LIST (partition);
