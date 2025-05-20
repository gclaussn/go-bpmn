CREATE TABLE IF NOT EXISTS incident (
	partition DATE NOT NULL,
	id INTEGER NOT NULL,

	element_id INTEGER,
	element_instance_id INTEGER,
	job_id INTEGER,
	process_id INTEGER,
	process_instance_id INTEGER,
	task_id INTEGER,

	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	resolved_at TIMESTAMP(3),
	resolved_by VARCHAR
) PARTITION BY LIST (partition);
