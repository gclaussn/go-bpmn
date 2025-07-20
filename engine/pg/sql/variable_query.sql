SELECT
	partition,
	id,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	created_at,
	created_by,
	encoding,
	is_encrypted,
	name,
	updated_at,
	updated_by
FROM
	variable
WHERE
	true
{{if not .c.Partition.IsZero}}
	AND partition = '{{.c.Partition}}'
{{end}}

{{if ne .c.ElementInstanceId 0}}
	AND element_instance_id = {{.c.ElementInstanceId}}
{{end}}
{{if ne .c.ProcessInstanceId 0}}
	AND process_instance_id = {{.c.ProcessInstanceId}}
{{end}}

{{if .c.Names}}
		AND name IN ({{.c.Names | joinString}})
{{end}}

ORDER BY
	partition, id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
