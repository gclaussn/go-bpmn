CREATE INDEX IF NOT EXISTS process_bpmn_process_id_idx ON process (bpmn_process_id);
CREATE INDEX IF NOT EXISTS process_tags_idx ON process USING gin (tags) WHERE tags IS NOT NULL;
