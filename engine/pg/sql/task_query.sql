SELECT
	task.partition,
	task.id,

	task.element_id,
	task.element_instance_id,
	task.process_id,
	task.process_instance_id,

	task.bpmn_element_id,
	task.bpmn_element_type,
	task.completed_at,
	task.created_at,
	task.created_by,
	task.due_at,
	task.error,
	CASE WHEN task.completed_at IS NULL THEN task_queue_lock.locked_at ELSE task.locked_at END AS locked_at,
	CASE WHEN task.completed_at IS NULL THEN task_queue_lock.locked_by ELSE task.locked_by END AS locked_by,
	task.retry_count,
	task.serialized_task,
	task.state,
	task.type
FROM
	task
LEFT JOIN
	task_queue_lock
ON
	task.completed_at IS NULL AND
	task_queue_lock.partition = task.partition AND
	task_queue_lock.task_id = task.id
WHERE
	true
{{if not .c.Partition.IsZero}}
	AND task.partition = '{{.c.Partition}}'
{{end}}
{{if ne .c.Id 0}}
	AND task.id = {{.c.Id}}
{{end}}

{{if ne .c.ElementId 0}}
	AND task.element_id = {{.c.ElementId}}
{{end}}
{{if ne .c.ElementInstanceId 0}}
	AND task.element_instance_id = {{.c.ElementInstanceId}}
{{end}}
{{if ne .c.ProcessId 0}}
	AND task.process_id = {{.c.ProcessId}}
{{end}}
{{if ne .c.ProcessInstanceId 0}}
	AND task.process_instance_id = {{.c.ProcessInstanceId}}
{{end}}

{{if ne .c.Type 0}}
	AND task.type = '{{.c.Type}}'
{{end}}

ORDER BY
	task.partition, task.id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
