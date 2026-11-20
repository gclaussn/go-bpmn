SELECT
	id,

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
FROM
	job_pool
WHERE
	due_at <= $1
{{if not .Partition.IsZero}}
	AND partition = '{{.Partition}}'
{{end}}
{{if ne .Id 0}}
	AND job_id = {{.Id}}
{{end}}

{{if .ProcessIds}}
	AND process_id IN ({{.ProcessIds | joinInt32}})
{{end}}
{{if ne .ProcessInstanceId 0}}
	AND process_instance_id = {{.ProcessInstanceId}}
{{end}}
LIMIT
	{{.Limit}}
FOR UPDATE SKIP LOCKED
