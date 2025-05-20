CREATE TABLE IF NOT EXISTS variable (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	element_id INTEGER,
	element_instance_id INTEGER,
	process_id INTEGER NOT NULL,
	process_instance_id INTEGER NOT NULL,

	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	encoding  VARCHAR NOT NULL,
	is_encrypted BOOLEAN NOT NULL,
	name VARCHAR NOT NULL,
	updated_at TIMESTAMP(3) NOT NULL,
	updated_by VARCHAR NOT NULL,
	value VARCHAR NOT NULL
) PARTITION BY LIST (partition);
