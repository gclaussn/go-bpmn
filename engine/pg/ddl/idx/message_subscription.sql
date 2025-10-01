CREATE INDEX IF NOT EXISTS message_subscription_name_correlation_key_idx ON message_subscription (name, correlation_key);
