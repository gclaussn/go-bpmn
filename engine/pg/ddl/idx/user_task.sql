CREATE INDEX IF NOT EXISTS user_task_id_idx ON user_task (id);
CREATE INDEX IF NOT EXISTS user_task_process_id_idx ON job (process_id);
CREATE INDEX IF NOT EXISTS user_task_process_instance_id_idx ON job (process_instance_id);
CREATE INDEX IF NOT EXISTS user_task_tags_idx ON user_task USING gin (tags) WHERE tags IS NOT NULL;
