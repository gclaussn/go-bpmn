SELECT
	id,

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

{{range $name, $value := .c.Tags}}
	AND tags->>{{$name | quoteString}} = {{$value | quoteString}}
{{end}}

{{if or (gt .o.Offset 0) (gt .o.Limit 0)}}
ORDER BY
	id
{{end}}
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
{{if gt .o.Limit 0}}
LIMIT {{.o.Limit}}
{{end}}
