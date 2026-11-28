DELETE FROM
	job_pool_lock
WHERE
	locked_by = {{.WorkerId | quoteString}}
{{if not .Partition.IsZero}}
	AND partition = '{{.Partition}}'
{{end}}
{{if ne .Id 0}}
	AND job_id = {{.Id}}
{{end}}
RETURNING
	partition,
	job_id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	correlation_key,
	due_at,
	retry_count,
	type
