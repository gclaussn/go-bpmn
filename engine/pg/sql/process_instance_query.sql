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

{{if ne .c.ParentId 0}}
	AND parent_id = {{.c.ParentId}}
{{end}}
{{if ne .c.RootId 0}}
	AND (root_id = {{.c.RootId}} OR id = {{.c.RootId}})
{{end}}

{{if ne .c.ProcessId 0}}
	AND process_id = {{.c.ProcessId}}
{{end}}

{{range $tag := .c.Tags}}
	{{- if $tag.Value}}
	AND tags->>{{$tag.Name | quoteString}} = {{$tag.Value | quoteString}}
	{{- else}}
	AND tags ? {{$tag.Name | quoteString}}
	{{- end}}
{{end}}

ORDER BY
	partition, id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
