CREATE TABLE IF NOT EXISTS element (
	id SERIAL PRIMARY KEY,

	process_id INTEGER NOT NULL,

	bpmn_element_id VARCHAR NOT NULL,
	bpmn_element_name VARCHAR,
	bpmn_element_type VARCHAR NOT NULL,
	is_multi_instance BOOLEAN NOT NULL,
	parent_bpmn_element_id VARCHAR
);
