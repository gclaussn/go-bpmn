CREATE INDEX IF NOT EXISTS signal_active_subscriber_count_idx ON signal (active_subscriber_count) INCLUDE (id) WHERE active_subscriber_count = 0;
