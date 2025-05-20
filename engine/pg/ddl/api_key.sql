CREATE TABLE IF NOT EXISTS api_key (
	id SERIAL PRIMARY KEY,

	created_at TIMESTAMP(3) NOT NULL,
	secret_hash VARCHAR NOT NULL,
	secret_id VARCHAR NOT NULL,

	UNIQUE(secret_id)
);
