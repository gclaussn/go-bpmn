CREATE TABLE IF NOT EXISTS timer_event (
	id SERIAL PRIMARY KEY,

	element_id INTEGER NOT NULL,
	process_id INTEGER NOT NULL,

	bpmn_process_id VARCHAR NOT NULL,
	is_suspended BOOLEAN NOT NULL,
	time TIMESTAMP(3),
	time_cycle VARCHAR,
	time_duration VARCHAR,
	version VARCHAR NOT NULL
);
