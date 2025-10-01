CREATE TABLE IF NOT EXISTS message_variable (
	id BIGSERIAL PRIMARY KEY,

	message_id BIGINT NOT NULL,

	encoding  VARCHAR,
	is_encrypted BOOLEAN,
	name VARCHAR NOT NULL,
	value VARCHAR
);
