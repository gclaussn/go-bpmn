CREATE INDEX IF NOT EXISTS process_instance_root_id_idx ON process_instance (root_id) WHERE root_id IS NOT NULL;
