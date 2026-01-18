UPDATE
	task
SET
	locked_at = null,
	locked_by = null,
	state = 'CREATED'
WHERE
	completed_at IS NULL
	AND locked_by = {{.EngineId | quoteString}}
{{if not .Partition.IsZero}}
	AND partition = '{{.Partition}}'
{{end}}
{{if ne .Id 0}}
	AND id = {{.Id}}
{{end}}
