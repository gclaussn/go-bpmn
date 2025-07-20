SELECT
	id,

	process_id,

	bpmn_element_id,
	bpmn_element_name,
	bpmn_element_type,
	is_multi_instance
FROM
	element
WHERE
	true
{{if ne .c.ProcessId 0}}
	AND process_id = {{.c.ProcessId}}
{{end}}

ORDER BY
	id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
