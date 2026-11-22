DELETE FROM
	job_pool
USING
	job
WHERE
	job.partition = job_pool.partition AND
	job.id = job_pool.job_id AND
	job.state = 'LOCKED'
;
