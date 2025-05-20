SELECT
	partition,
	id,

	element_id,
	element_instance_id,
	job_id,
	process_id,
	process_instance_id,
	task_id,

	created_at,
	created_by,
	resolved_at,
	resolved_by
FROM
	incident
WHERE
	true
{{if not .c.Partition.IsZero}}
	AND partition = '{{.c.Partition}}'
{{end}}
{{if ne .c.Id 0}}
	AND id = {{.c.Id}}
{{end}}

{{if ne .c.JobId 0}}
	AND job_id = {{.c.JobId}}
{{end}}
{{if ne .c.ProcessInstanceId 0}}
	AND process_instance_id = {{.c.ProcessInstanceId}}
{{end}}
{{if ne .c.TaskId 0}}
	AND task_id = {{.c.TaskId}}
{{end}}

{{if or (gt .o.Offset 0) (gt .o.Limit 0)}}
ORDER BY
	partition, id
{{end}}
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
{{if gt .o.Limit 0}}
LIMIT {{.o.Limit}}
{{end}}
