CREATE INDEX IF NOT EXISTS job_pool_process_id_due_at_idx ON job_pool (process_id, due_at);
