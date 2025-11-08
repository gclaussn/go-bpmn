UPDATE
	job
SET
	locked_at = $1,
	locked_by = {{.WorkerId | quoteString}}
FROM (
	SELECT
		partition,
		id
	FROM
		job
	WHERE
		locked_at IS NULL AND due_at <= $1
{{if not .Partition.IsZero}}
		AND partition = '{{.Partition}}'
{{end}}
{{if ne .Id 0}}
		AND id = {{.Id}}
{{end}}

{{if .ProcessIds}}
		AND process_id IN ({{.ProcessIds | joinInt32}})
{{end}}
{{if ne .ProcessInstanceId 0}}
		AND process_instance_id = {{.ProcessInstanceId}}
{{end}}
	ORDER BY
		partition,
		due_at
	LIMIT
		{{.Limit}}
	FOR UPDATE SKIP LOCKED
) AS locked
WHERE
	job.partition = locked.partition AND
	job.id = locked.id
RETURNING
	job.partition,
	job.id,

	job.element_id,
	job.element_instance_id,
	job.process_id,
	job.process_instance_id,

	job.bpmn_element_id,
	job.correlation_key,
	job.created_at,
	job.created_by,
	job.due_at,
	job.locked_at,
	job.locked_by,
	job.retry_count,
	job.type
