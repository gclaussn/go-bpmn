CREATE TABLE IF NOT EXISTS event (
	partition DATE NOT NULL,

	element_instance_id INTEGER NOT NULL,

	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	error_code VARCHAR,
	escalation_code VARCHAR,
	message_correlation_key VARCHAR,
	message_name VARCHAR,
	signal_name VARCHAR,
	time TIMESTAMP(3),
	time_cycle VARCHAR,
	time_duration VARCHAR
) PARTITION BY LIST (partition);
