CREATE INDEX IF NOT EXISTS task_queue_lock_partition_task_id_idx ON task_queue_lock (partition, task_id);
CREATE INDEX IF NOT EXISTS task_queue_lock_locked_by_idx ON task_queue_lock (locked_by);
