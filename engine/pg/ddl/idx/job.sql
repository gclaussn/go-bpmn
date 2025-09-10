CREATE INDEX IF NOT EXISTS job_id_idx ON job (id);
CREATE INDEX IF NOT EXISTS job_due_at_idx ON job (process_id, due_at) WHERE locked_by IS NULL;
CREATE INDEX IF NOT EXISTS job_process_id_idx ON job (process_id);
CREATE INDEX IF NOT EXISTS job_process_instance_id_idx ON job (process_instance_id);
