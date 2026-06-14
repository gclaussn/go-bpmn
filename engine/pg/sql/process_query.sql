SELECT
	id,

	bpmn_collaboration_id,
	bpmn_participant_id,
	bpmn_participant_name,
	bpmn_process_id,
	created_at,
	created_by,
	parallelism,
	tags,
	version
FROM
	process
WHERE
	true
{{if ne .c.Id 0}}
	AND id = {{.c.Id}}
{{end}}

{{range $tag := .c.Tags}}
	{{- if $tag.Value}}
	AND tags->>{{$tag.Name | quoteString}} = {{$tag.Value | quoteString}}
	{{- else}}
	AND tags ? {{$tag.Name | quoteString}}
	{{- end}}
{{end}}

ORDER BY
	id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
