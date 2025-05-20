package pg

import (
	"strconv"
	"strings"
	"text/template"

	"github.com/gclaussn/go-bpmn/engine"
)

var (
	sqlTemplateFunctions = template.FuncMap{
		"joinInstanceState": joinInstanceState,
		"joinInt32":         joinInt32,
		"joinString":        joinString,
		"quoteString":       quoteString,
	}

	sqlElementQuery         *template.Template = newSqlTemplate("element_query.sql")
	sqlElementInstanceQuery *template.Template = newSqlTemplate("element_instance_query.sql")
	sqlIncidentQuery        *template.Template = newSqlTemplate("incident_query.sql")
	sqlJobLock              *template.Template = newSqlTemplate("job_lock.sql")
	sqlJobQuery             *template.Template = newSqlTemplate("job_query.sql")
	sqlJobUnlock            *template.Template = newSqlTemplate("job_unlock.sql")
	sqlProcessInstanceQuery *template.Template = newSqlTemplate("process_instance_query.sql")
	sqlProcessQuery         *template.Template = newSqlTemplate("process_query.sql")
	sqlTaskLock             *template.Template = newSqlTemplate("task_lock.sql")
	sqlTaskQuery            *template.Template = newSqlTemplate("task_query.sql")
	sqlTaskUnlock           *template.Template = newSqlTemplate("task_unlock.sql")
	sqlVariableElementQuery *template.Template = newSqlTemplate("variable_element_query.sql")
	sqlVariableProcessQuery *template.Template = newSqlTemplate("variable_process_query.sql")
	sqlVariableQuery        *template.Template = newSqlTemplate("variable_query.sql")
)

func newSqlTemplate(name string) *template.Template {
	return template.Must(template.New(name).Funcs(sqlTemplateFunctions).ParseFS(resources, "sql/"+name))
}

func joinInstanceState(values []engine.InstanceState) string {
	s := make([]string, len(values))
	for i, v := range values {
		s[i] = quoteString(v.String())
	}
	return strings.Join(s, ",")
}

func joinInt32(values []int32) string {
	s := make([]string, len(values))
	for i, v := range values {
		s[i] = strconv.Itoa(int(v))
	}
	return strings.Join(s, ",")
}

func joinString(values []string) string {
	s := make([]string, len(values))
	for i, v := range values {
		s[i] = quoteString(v)
	}
	return strings.Join(s, ",")
}

// copied from https://github.com/jackc/pgx/blob/v5.5.0/internal/sanitize/sanitize.go#L90
func quoteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
