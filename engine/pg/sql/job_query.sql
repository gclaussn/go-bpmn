SELECT
	job.partition,
	job.id,

	job.element_id,
	job.element_instance_id,
	job.process_id,
	job.process_instance_id,

	job.bpmn_element_id,
	job.bpmn_element_type,
	job.completed_at,
	job.created_at,
	job.created_by,
	job.due_at,
	job.error,
	CASE WHEN job.completed_at IS NULL THEN job_pool_lock.locked_at ELSE job.locked_at END AS locked_at,
	CASE WHEN job.completed_at IS NULL THEN job_pool_lock.locked_by ELSE job.locked_by END AS locked_by,
	job.retry_count,
	job.state,
	job.type
FROM
	job
LEFT JOIN
	job_pool_lock
ON
	job.completed_at IS NULL AND
	job_pool_lock.partition = job.partition AND
	job_pool_lock.job_id = job.id
WHERE
	true
{{if not .c.Partition.IsZero}}
	AND job.partition = '{{.c.Partition}}'
{{end}}
{{if ne .c.Id 0}}
	AND job.id = {{.c.Id}}
{{end}}

{{if ne .c.ElementId 0}}
	AND job.element_id = {{.c.ElementId}}
{{end}}
{{if ne .c.ElementInstanceId 0}}
	AND job.element_instance_id = {{.c.ElementInstanceId}}
{{end}}
{{if ne .c.ProcessId 0}}
	AND job.process_id = {{.c.ProcessId}}
{{end}}
{{if ne .c.ProcessInstanceId 0}}
	AND job.process_instance_id = {{.c.ProcessInstanceId}}
{{end}}

ORDER BY
	job.partition, job.id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
