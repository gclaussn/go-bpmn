CREATE INDEX IF NOT EXISTS element_instance_id_idx ON element_instance (id);
CREATE INDEX IF NOT EXISTS element_instance_process_id_idx ON element_instance (process_id);
CREATE INDEX IF NOT EXISTS element_instance_process_instance_id_element_id_idx ON element_instance (process_instance_id, element_id);
CREATE INDEX IF NOT EXISTS element_instance_process_instance_id_state_idx ON element_instance (process_instance_id, state);
