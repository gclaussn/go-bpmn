SELECT
	partition,
	id,

	parent_id,

	element_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	bpmn_element_type,
	created_at,
	created_by,
	ended_at,
	is_multi_instance,
	started_at,
	state,
	state_changed_by
FROM
	element_instance
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
{{if ne .c.ProcessInstanceId 0}}
	AND process_instance_id = {{.c.ProcessInstanceId}}
{{end}}

{{if ne .c.BpmnElementId ""}}
	AND bpmn_element_id = {{.c.BpmnElementId | quoteString}}
{{end}}
{{if .c.States}}
	AND state IN ({{.c.States | joinInstanceState}})
{{end}}

ORDER BY
	partition, id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
