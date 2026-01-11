CREATE TABLE IF NOT EXISTS message_subscription (
	id BIGSERIAL PRIMARY KEY,

	partition DATE NOT NULL,

	element_id INTEGER NOT NULL,
	element_instance_id INTEGER NOT NULL,
	process_id INTEGER NOT NULL,
	process_instance_id INTEGER NOT NULL,

	bpmn_element_id VARCHAR NOT NULL,
	correlation_key VARCHAR NOT NULL,
	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	name VARCHAR NOT NULL
);
