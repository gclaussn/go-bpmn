package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

const (
	formatJson     = "json"
	formatTemplate = "template"
)

func FormatTime(v time.Time) string {
	if v.IsZero() {
		return ""
	}
	return v.Format(time.RFC3339)
}

func FormatTimeOrNil(v *time.Time) string {
	if v == nil {
		return ""
	}
	return FormatTime(*v)
}

func NewTable(headers []string) *Table {
	rows := make([][]string, 2)
	rows[0] = headers
	rows[1] = make([]string, len(headers))

	return &Table{rows: rows}
}

type Table struct {
	rows [][]string
}

func (t *Table) AddRow(row []string) {
	t.rows = append(t.rows, row)
}

func (t *Table) String() string {
	rows := t.rows

	columns := make([]int, len(rows[0]))
	for i := range rows {
		for j := 0; j < len(columns); j++ {
			l := utf8.RuneCountInString(rows[i][j])
			if columns[j] < l {
				columns[j] = l
			}
		}
	}

	var sb strings.Builder
	for i := range rows {
		for j := range columns {
			if j != 0 {
				sb.WriteString("   ")
			}

			value := rows[i][j]
			sb.WriteString(value)

			l := utf8.RuneCountInString(value)
			for k := 0; k < columns[j]-l; k++ {
				sb.WriteRune(' ')
			}
		}
		sb.WriteRune('\n')
	}

	return sb.String()
}

// Formatter formats output in JSON format or using a Go template.
type Formatter struct {
	format   string
	template *template.Template
}

func (f *Formatter) Flag(cmd *cobra.Command) {
	cmd.Flags().Var(f, "format", `Format output using:
json      Print in JSON format
TEMPLATE  Print output using a Go template string
file://   Print output using a Go template file`)
}

func (f Formatter) Format(cmd *cobra.Command, data any, defaultFormat string) error {
	if f.format == "" {
		if err := f.Set(defaultFormat); err != nil {
			return err
		}
	}

	switch f.format {
	case formatJson:
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format data: %v", err)
		}

		cmd.Print(string(b))
	case formatTemplate:
		var out bytes.Buffer
		if err := f.template.Execute(&out, data); err != nil {
			return fmt.Errorf("failed to execute Go template: %v", err)
		}

		cmd.Print(out.String())
	}

	return nil
}

func (f *Formatter) Set(s string) error {
	if s == formatJson {
		f.format = formatJson
		return nil
	}

	var text string
	if strings.HasPrefix(s, "file://") {
		templateName := s[7:]

		templateFile, err := os.Open(templateName)
		if err != nil {
			return fmt.Errorf("failed to open Go template file %s: %v", templateName, err)
		}

		defer templateFile.Close()

		b, err := io.ReadAll(templateFile)
		if err != nil {
			return fmt.Errorf("failed to read Go template file %s: %v", templateName, err)
		}

		text = string(b)
	} else {
		text = s
	}

	t, err := template.New("format").Funcs(newTemplateFuncs()).Parse(text)
	if err != nil {
		return fmt.Errorf("failed to parse Go template: %v", err)
	}

	f.format = formatTemplate
	f.template = t
	return nil
}

func (f Formatter) String() string {
	return f.format
}

func (f Formatter) Type() string {
	return "format"
}

func newTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatTime":      FormatTime,
		"formatTimeOrNil": FormatTimeOrNil,
	}
}
