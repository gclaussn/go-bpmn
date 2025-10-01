CREATE TABLE IF NOT EXISTS message (
	id BIGSERIAL PRIMARY KEY,

	correlation_key VARCHAR NOT NULL,
	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	expires_at TIMESTAMP(3),
	is_conflict BOOLEAN NOT NUll,
	is_correlated BOOLEAN NOT NULL,
	name VARCHAR NOT NULL,
	unique_key VARCHAR
);
