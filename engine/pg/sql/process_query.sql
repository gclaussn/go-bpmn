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

ORDER BY
	id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
