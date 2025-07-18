CREATE TABLE IF NOT EXISTS event_variable (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	event_id INTEGER NOT NULL,

	encoding  VARCHAR,
	is_encrypted BOOLEAN,
	name VARCHAR NOT NULL,
	value VARCHAR
) PARTITION BY LIST (partition);
