INSERT INTO job_pool (
	partition,

	element_id,
	element_instance_id,
	job_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	correlation_key,
	due_at,
	retry_count,
	type
)
SELECT
	partition,

	element_id,
	element_instance_id,
	id AS job_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	correlation_key,
	due_at,
	retry_count,
	type
FROM
	job
WHERE
	completed_at IS NULL
ORDER BY
	due_at
;
