CREATE TABLE IF NOT EXISTS process_instance_queue_element (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	process_id INTEGER NOT NULL,

	next_partition DATE,
	next_id INTEGER,
	prev_partition DATE,
	prev_id INTEGER
) PARTITION BY LIST (partition);
