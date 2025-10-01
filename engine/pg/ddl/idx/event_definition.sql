CREATE INDEX IF NOT EXISTS event_definition_bpmn_process_id_idx ON event_definition (bpmn_process_id);
CREATE INDEX IF NOT EXISTS event_definition_message_name_idx ON event_definition (message_name) WHERE message_name IS NOT NULL AND is_suspended IS FALSE;
CREATE INDEX IF NOT EXISTS event_definition_signal_name_idx ON event_definition (signal_name) WHERE signal_name IS NOT NULL AND is_suspended IS FALSE;
