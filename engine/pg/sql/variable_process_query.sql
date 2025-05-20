SELECT
	encoding,
	is_encrypted,
	name,
	value
FROM
	variable
WHERE
	partition = '{{.Partition}}'
	AND process_instance_id = {{.ProcessInstanceId}}
	AND element_instance_id IS NULL
{{if .Names}}
	AND name IN ({{ .Names | joinString }})
{{end}}
