SELECT
	partition,
	id,

	revision,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	correlation_key,
	created_at,
	created_by,
	state,
	tags,
	updated_at,
	updated_by
FROM
	user_task
WHERE
	true
{{if not .c.Partition.IsZero}}
	AND partition = '{{.c.Partition}}'
{{end}}
{{if ne .c.Id 0}}
	AND id = {{.c.Id}}
{{end}}

{{if ne .c.ElementInstanceId 0}}
	AND element_instance_id = {{.c.ElementInstanceId}}
{{end}}
{{if ne .c.ProcessId 0}}
	AND process_id = {{.c.ProcessId}}
{{end}}
{{if ne .c.ProcessInstanceId 0}}
	AND process_instance_id = {{.c.ProcessInstanceId}}
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
