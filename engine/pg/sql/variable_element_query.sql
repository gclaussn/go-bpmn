SELECT
	encoding,
	is_encrypted,
	name,
	value
FROM
	variable
WHERE
	partition = '{{.Partition}}'
	AND element_instance_id = {{.ElementInstanceId}}
{{if .Names}}
	AND name IN ({{ .Names | joinString }})
{{end}}
