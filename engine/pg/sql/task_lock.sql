SELECT
	id,

	partition,
	task_id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	due_at,
	retry_count,
  serialized_task,
	type
FROM
	task_queue
WHERE
	due_at <= $1
{{if not .Partition.IsZero}}
	AND partition = '{{.Partition}}'
{{end}}
{{if ne .Id 0}}
	AND task_id = {{.Id}}
{{end}}

{{if ne .ProcessInstanceId 0}}
	AND process_instance_id = {{.ProcessInstanceId}}
{{end}}
ORDER BY
	due_at
LIMIT
	{{.Limit}}
FOR UPDATE SKIP LOCKED
