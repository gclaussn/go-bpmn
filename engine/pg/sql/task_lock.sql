UPDATE
	task
SET
	locked_at = $1,
	locked_by = $2
FROM (
	SELECT
		partition,
		id
	FROM
		task
	WHERE
		locked_at IS NULL AND due_at <= $1
{{if not .Partition.IsZero}}
		AND partition = '{{.Partition}}'
{{end}}
{{if ne .Id 0}}
		AND id = {{.Id}}
{{end}}

{{if ne .ProcessId 0}}
		AND process_id = {{.ProcessId}}
{{end}}
{{if ne .ProcessInstanceId 0}}
		AND process_instance_id = {{.ProcessInstanceId}}
{{end}}
{{if ne .Type 0}}
		AND type = '{{.Type}}'
{{end}}
	ORDER BY
		partition,
		due_at
	LIMIT
		{{ .Limit }}
	FOR UPDATE SKIP LOCKED
) AS locked
WHERE
	task.partition = locked.partition AND
	task.id = locked.id
RETURNING
	task.partition,
	task.id,

	task.element_id,
	task.element_instance_id,
	task.process_id,
	task.process_instance_id,

	task.created_at,
	task.created_by,
	task.due_at,
	task.locked_at,
	task.locked_by,
	task.retry_count,
  task.serialized_task,
	task.type
