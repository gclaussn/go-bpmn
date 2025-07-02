CREATE TABLE IF NOT EXISTS signal_event (
	element_id INTEGER PRIMARY KEY,

	process_id INTEGER NOT NULL,

	bpmn_element_id VARCHAR NOT NULL,
	bpmn_process_id VARCHAR NOT NULL,
	is_suspended BOOLEAN NOT NULL,
	name VARCHAR,
	version VARCHAR NOT NULL
);
