SELECT
	partition,
	id,

	parent_id,
	root_id,

	process_id,

	bpmn_process_id,
	correlation_key,
	created_at,
	created_by,
	ended_at,
	started_at,
	state,
	state_changed_by,
	tags,
	version
FROM
	process_instance
WHERE
	true
{{if not .c.Partition.IsZero}}
	AND partition = '{{.c.Partition}}'
{{end}}
{{if ne .c.Id 0}}
	AND id = {{.c.Id}}
{{end}}

{{if ne .c.ProcessId 0}}
	AND process_id = {{.c.ProcessId}}
{{end}}

{{range $name, $value := .c.Tags}}
	AND tags->>{{$name | quoteString}} = {{$value | quoteString}}
{{end}}

ORDER BY
	partition, id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
