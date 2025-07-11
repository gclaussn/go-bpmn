CREATE TABLE IF NOT EXISTS variable (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	element_id INTEGER,
	element_instance_id INTEGER,
	event_id INTEGER,
	process_id INTEGER,
	process_instance_id INTEGER,

	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	encoding  VARCHAR,
	is_encrypted BOOLEAN,
	name VARCHAR NOT NULL,
	updated_at TIMESTAMP(3) NOT NULL,
	updated_by VARCHAR NOT NULL,
	value VARCHAR
) PARTITION BY LIST (partition);
