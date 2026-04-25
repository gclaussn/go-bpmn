SELECT
	id,

	partition,

	element_id,
	element_instance_id,
	process_id,
	process_instance_id,

	bpmn_element_id,
	correlation_key,
	created_at,
	created_by,
	name
FROM
	message_subscription
WHERE
	true
{{if not .c.Partition.IsZero}}
	AND partition = '{{.c.Partition}}'
{{end}}

{{if ne .c.ProcessInstanceId 0}}
	AND process_instance_id = {{.c.ProcessInstanceId}}
{{end}}

{{if ne .c.CorrelationKey ""}}
	AND correlation_key = {{.c.CorrelationKey | quoteString}}
{{end}}
{{if ne .c.Name ""}}
	AND name = {{.c.Name | quoteString}}
{{end}}

ORDER BY
	id
{{if gt .o.Offset 0}}
OFFSET {{.o.Offset}}
{{end}}
LIMIT {{.o.Limit}}
