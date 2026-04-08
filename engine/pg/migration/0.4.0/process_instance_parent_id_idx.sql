CREATE INDEX IF NOT EXISTS process_instance_parent_id_idx ON process_instance (parent_id) WHERE parent_id IS NOT NULL;
