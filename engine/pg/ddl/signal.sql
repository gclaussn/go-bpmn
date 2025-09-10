CREATE TABLE IF NOT EXISTS signal (
	id BIGSERIAL PRIMARY KEY,

	active_subscriber_count INTEGER NOT NULL,
	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	name VARCHAR NOT NULL,
	subscriber_count INTEGER NOT NULL
);
