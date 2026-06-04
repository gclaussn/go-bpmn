SELECT
	encoding,
	is_encrypted,
	name,
	value
FROM
	variable
WHERE
	partition = '{{.partition}}'
	AND element_instance_id = {{.elementInstanceId}}
{{if .names}}
	AND name IN ({{ .names | joinString }})
{{end}}
