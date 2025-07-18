CREATE INDEX IF NOT EXISTS variable_element_instance_id_idx ON variable (element_instance_id) WHERE element_instance_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS variable_process_instance_id_idx ON variable (process_instance_id);
