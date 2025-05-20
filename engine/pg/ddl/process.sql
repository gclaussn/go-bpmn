CREATE TABLE IF NOT EXISTS process (
	id SERIAL PRIMARY KEY,

	bpmn_process_id VARCHAR NOT NULL,
	bpmn_xml VARCHAR NOT NULL,
	bpmn_xml_md5 VARCHAR(32) NOT NULL,
	created_at TIMESTAMP(3) NOT NULL,
	created_by VARCHAR NOT NULL,
	parallelism INTEGER NOT NULL,
	tags JSONB,
	version VARCHAR NOT NULL,

	UNIQUE (bpmn_process_id, version)
);
