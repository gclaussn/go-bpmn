CREATE TABLE IF NOT EXISTS signal_subscription (
	id BIGSERIAL PRIMARY KEY,

	partition DATE NOT NULL,

	element_id INTEGER NOT NULL,
	element_instance_id INTEGER NOT NULL,
	process_id INTEGER NOT NULL,
	process_instance_id INTEGER NOT NULL,

	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	name VARCHAR NOT NULL
);
