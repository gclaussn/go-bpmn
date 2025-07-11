CREATE TABLE IF NOT EXISTS event (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	signal_name VARCHAR,
	time TIMESTAMP(3),
	time_cycle VARCHAR,
	time_duration VARCHAR,
) PARTITION BY LIST (partition);
