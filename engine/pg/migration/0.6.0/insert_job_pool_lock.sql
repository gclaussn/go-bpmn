INSERT INTO job_pool_lock
SELECT
	job_pool.id,

	job_pool.partition,

	job_pool.element_id,
	job_pool.element_instance_id,
	job_pool.job_id,
	job_pool.process_id,
	job_pool.process_instance_id,

	job_pool.bpmn_element_id,
	job_pool.bpmn_element_type,
	job_pool.correlation_key,
	job_pool.due_at,
	job.locked_at,
	job.locked_by,
	job_pool.retry_count,
	job_pool.type
FROM
	job_pool
INNER JOIN
	job
ON
	job.partition = job_pool.partition AND
	job.id = job_pool.job_id AND
	job.state = 'LOCKED'
ORDER BY
	job_pool.due_at
;
