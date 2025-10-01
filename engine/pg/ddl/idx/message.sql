CREATE INDEX IF NOT EXISTS message_name_correlation_key_idx ON message (name, correlation_key);
CREATE INDEX IF NOT EXISTS message_purge_idx ON message (expires_at) INCLUDE (id) WHERE expires_at IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS message_unique_key_idx ON message (name, correlation_key, unique_key) WHERE unique_key IS NOT NULL AND (is_correlated IS FALSE OR expires_at IS NULL);
