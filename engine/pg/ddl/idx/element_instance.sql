CREATE INDEX IF NOT EXISTS element_instance_id_idx ON element_instance (id);
CREATE INDEX IF NOT EXISTS element_instance_parent_id_idx ON element_instance (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS element_instance_prev_id_idx ON element_instance (prev_id) WHERE prev_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS element_instance_process_id_idx ON element_instance (process_id);
CREATE INDEX IF NOT EXISTS element_instance_process_instance_id_state_idx ON element_instance (process_instance_id, state);
