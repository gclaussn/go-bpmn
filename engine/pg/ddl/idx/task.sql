CREATE INDEX IF NOT EXISTS task_id_idx ON task (id);
CREATE INDEX IF NOT EXISTS task_lock_idx ON task (due_at) WHERE locked_by IS NULL;
CREATE INDEX IF NOT EXISTS task_process_id_idx ON task (process_id);
CREATE INDEX IF NOT EXISTS task_process_instance_id_idx ON task (process_instance_id);
CREATE INDEX IF NOT EXISTS task_type_idx ON task (type);
