SELECT
	element.id,

	element.process_id,

	element.bpmn_element_id,
	element.bpmn_element_name,
	element.bpmn_element_type,
	element.is_multi_instance,

	event_definition.is_suspended,
	event_definition.message_name,
	event_definition.signal_name,
	event_definition.time,
	event_definition.time_cycle,
	event_definition.time_duration
FROM
	element
LEFT JOIN
	event_definition
ON
	event_definition.element_id = element.id
WHERE
	true
{{if ne .c.ProcessId 0}}
	AND element.process_id = {{.c.ProcessId}}
{{end}}

{{if ne .c.BpmnElementId ""}}
	AND element.bpmn_element_id = {{.c.BpmnElementId | quoteString}}
{{end}}

ORDER BY
	element.id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
