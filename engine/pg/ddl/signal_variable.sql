CREATE TABLE IF NOT EXISTS signal_variable (
	id BIGSERIAL PRIMARY KEY,

	signal_id BIGINT NOT NULL,

	encoding  VARCHAR,
	is_encrypted BOOLEAN,
	name VARCHAR NOT NULL,
	value VARCHAR
);
