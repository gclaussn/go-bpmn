CREATE TABLE IF NOT EXISTS event_definition (
	element_id INTEGER PRIMARY KEY,

	process_id INTEGER NOT NULL,

	bpmn_element_id VARCHAR NOT NULL,
	bpmn_element_type VARCHAR NOT NULL,
	bpmn_process_id VARCHAR NOT NULL,
	error_code VARCHAR,
	is_suspended BOOLEAN NOT NULL,
	message_name VARCHAR,
	signal_name VARCHAR,
	time TIMESTAMP(3),
	time_cycle VARCHAR,
	time_duration VARCHAR,
	version VARCHAR NOT NULL
);
