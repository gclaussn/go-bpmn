DELETE FROM
	task_queue_lock
WHERE
	locked_by = {{.EngineId | quoteString}}
{{if not .Partition.IsZero}}
	AND partition = '{{.Partition}}'
{{end}}
{{if ne .Id 0}}
	AND task_id = {{.Id}}
{{end}}
RETURNING
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
