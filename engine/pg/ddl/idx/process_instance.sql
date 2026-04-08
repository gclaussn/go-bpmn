CREATE INDEX IF NOT EXISTS process_instance_id_idx ON process_instance (id);
CREATE INDEX IF NOT EXISTS process_instance_root_id_idx ON process_instance (root_id) WHERE root_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS process_instance_parent_id_idx ON process_instance (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS process_instance_process_id_idx ON process_instance (process_id);
CREATE INDEX IF NOT EXISTS process_instance_tags_idx ON process_instance USING gin (tags) WHERE tags IS NOT NULL;
