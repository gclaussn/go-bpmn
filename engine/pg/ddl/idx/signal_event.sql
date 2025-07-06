CREATE INDEX IF NOT EXISTS signal_event_bpmn_process_id_idx ON signal_event (bpmn_process_id);
CREATE INDEX IF NOT EXISTS signal_event_name_idx ON signal_event (name) WHERE is_suspended IS FALSE;
