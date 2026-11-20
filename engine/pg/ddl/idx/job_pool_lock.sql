CREATE INDEX IF NOT EXISTS job_pool_lock_partition_job_id_idx ON job_pool_lock (partition, job_id);
CREATE INDEX IF NOT EXISTS job_pool_lock_locked_by_idx ON job_pool_lock (locked_by);
