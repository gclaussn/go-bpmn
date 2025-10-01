SELECT
	id,

	correlation_key,
	created_at,
	created_by,
	expires_at,
	is_correlated,
	name,
	unique_key
FROM
	message
WHERE
	true
{{if ne .c.Id 0}}
	AND id = {{.c.Id}}
{{end}}

{{if .c.ExcludeExpired}}
	AND (expires_at IS NULL OR expires_at > $1)
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
