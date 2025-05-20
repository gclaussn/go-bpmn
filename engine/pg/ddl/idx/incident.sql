CREATE INDEX IF NOT EXISTS incident_id_idx ON incident (id);
CREATE INDEX IF NOT EXISTS incident_job_id_idx ON incident (job_id) WHERE job_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS incident_process_instance_id_idx ON incident (process_instance_id) WHERE process_instance_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS incident_task_id_idx ON incident (task_id) WHERE task_id IS NOT NULL;
