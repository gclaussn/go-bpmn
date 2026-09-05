package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"os"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

const (
	formatJson  = "json"
	formatJsonl = "jsonl"
	formatRow   = "row"
	formatTable = "table"

	prefixColumns = "columns:"
	prefixFile    = "file://"
	prefixTable   = "table:"
)

func FormatId(v int32) string {
	if v == 0 {
		return ""
	}
	return strconv.Itoa(int(v))
}

func FormatTime(v time.Time) string {
	if v.IsZero() {
		return ""
	}
	return v.Format(time.RFC3339)
}

func NewTable(columns string) *Table {
	split := strings.Split(columns, ",")

	rows := make([][]string, 2)
	rows[0] = split
	rows[1] = make([]string, len(split))

	return &Table{rows: rows}
}

func NewTableFormatter(allColumns string, defaultColumns string) TableFormatter {
	return TableFormatter{
		format: formatTable,

		allColumns:      allColumns,
		selectedColumns: defaultColumns,
	}
}

func Rows[T any](data []T) iter.Seq2[int, any] {
	return func(yield func(int, any) bool) {
		for i := range data {
			if !yield(i, data[i]) {
				break
			}
		}
	}
}

type Table struct {
	rows [][]string
}

func (t *Table) AddRow(row []string) {
	t.rows = append(t.rows, row)
}

func (t *Table) Format(cmd *cobra.Command, selectedColumns string) {
	rows := t.rows

	var idx []int
	for column := range strings.SplitSeq(selectedColumns, ",") {
		if i := slices.Index(rows[0], column); i >= 0 {
			idx = append(idx, i)
		}
	}

	for i, header := range rows[0] {
		var sb strings.Builder
		for _, r := range header {
			if unicode.IsUpper(r) {
				sb.WriteRune(' ')
				sb.WriteRune(r)
			} else {
				sb.WriteRune(unicode.ToUpper(r))
			}
		}
		rows[0][i] = sb.String()
	}

	columns := make([]int, len(rows[0]))
	for i := range rows {
		for j := range columns {
			l := utf8.RuneCountInString(rows[i][j])
			if columns[j] < l {
				columns[j] = l
			}
		}
	}

	var out strings.Builder
	for i := range rows {
		out.Reset()
		for j, index := range idx {
			if j != 0 {
				out.WriteString("   ")
			}

			value := rows[i][index]
			out.WriteString(value)

			l := utf8.RuneCountInString(value)
			for k := 0; k < columns[index]-l; k++ {
				out.WriteRune(' ')
			}
		}
		cmd.Println(out.String())
	}
}

// Formatter formats output in JSON format or using a Go template.
type Formatter struct {
	format   string // can be json
	template *template.Template
}

func (f *Formatter) Flag(cmd *cobra.Command) {
	cmd.Flags().Var(f, "format", `Format output using:
json      Print in JSON format
TEMPLATE  Print output using a Go template string
file://   Print output using a Go template file`)
}

func (f Formatter) Format(cmd *cobra.Command, data any, defaultFormat string) error {
	if f.format == "" && f.template == nil {
		if err := f.Set(defaultFormat); err != nil {
			return err
		}
	}

	if f.format == formatJson {
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to format data: %v", err)
		}

		cmd.Print(string(b))
	} else {
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

	var (
		t   *template.Template
		err error
	)
	if strings.HasPrefix(s, prefixFile) {
		s = s[len(prefixFile):]
		t, err = newTemplateFromFile(s)
	} else {
		t, err = newTemplate(s)
	}

	if err != nil {
		return err
	}

	f.template = t
	return nil
}

func (f Formatter) String() string {
	return f.format
}

func (f Formatter) Type() string {
	return "format"
}

type TableFormatter struct {
	format   string // can be json, jsonl, row or table
	template *template.Template

	allColumns      string
	selectedColumns string
}

func (f *TableFormatter) Flag(cmd *cobra.Command) {
	cmd.Flags().Var(f, "format", `Format table output using:
json            Print in JSON format
jsonl           Print in JSONL format
TEMPLATE        Print each row using a Go template string
file://         Print each row using a Go template file
table:TEMPLATE  Print table using a Go template string
table:file://   Print table using a Go template file
columns:        Print in table format, using selected columns`)
}

func (f TableFormatter) Format(cmd *cobra.Command, data any, rows iter.Seq2[int, any]) (bool, error) {
	switch f.format {
	case formatJson:
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return false, fmt.Errorf("failed to format data: %v", err)
		}

		cmd.Print(string(b))
	case formatJsonl:
		for i, row := range rows {
			b, err := json.Marshal(row)
			if err != nil {
				return false, fmt.Errorf("failed to format row %d: %v", i, err)
			}
			cmd.Println(string(b))
		}
	case formatRow:
		var out bytes.Buffer
		for i, row := range rows {
			out.Reset()
			if err := f.template.Execute(&out, row); err != nil {
				return false, fmt.Errorf("failed to execute Go template for row %d: %v", i, err)
			}
			cmd.Println(out.String())
		}
	case formatTable:
		if f.template == nil {
			return false, nil // print table
		}

		var out bytes.Buffer
		if err := f.template.Execute(&out, data); err != nil {
			return false, fmt.Errorf("failed to execute Go template: %v", err)
		}
		cmd.Print(out.String())
	}

	return true, nil
}

func (f TableFormatter) SelectedColumns() string {
	return f.selectedColumns
}

func (f *TableFormatter) Set(s string) error {
	if s == formatJson || s == formatJsonl {
		f.format = s
		return nil
	}

	var (
		t   *template.Template
		err error
	)
	switch {
	case strings.HasPrefix(s, prefixColumns):
		s = s[len(prefixColumns):]

		columnMap := make(map[string]bool)
		for column := range strings.SplitSeq(f.allColumns, ",") {
			columnMap[column] = true
		}

		var unknownColumns []string
		for column := range strings.SplitSeq(s, ",") {
			if _, ok := columnMap[column]; !ok {
				unknownColumns = append(unknownColumns, column)
			}
		}

		if len(unknownColumns) != 0 {
			return fmt.Errorf("unknown columns: %s (possible columns are: %s)", strings.Join(unknownColumns, ","), f.allColumns)
		}

		f.selectedColumns = s
		return nil
	case strings.HasPrefix(s, prefixTable+prefixFile):
		s = s[len(prefixTable)+len(prefixFile):]
		t, err = newTemplateFromFile(s)
	case strings.HasPrefix(s, prefixTable):
		s = s[len(prefixTable):]
		t, err = newTemplate(s)
	case strings.HasPrefix(s, prefixFile):
		s = s[len(prefixFile):]
		t, err = newTemplateFromFile(s)
		f.format = formatRow
	default:
		t, err = newTemplate(s)
		f.format = formatRow
	}

	if err != nil {
		return err
	}

	f.template = t
	return nil
}

func (f TableFormatter) String() string {
	return f.format
}

func (f TableFormatter) Type() string {
	return "format"
}

func newTemplate(text string) (*template.Template, error) {
	t, err := template.New("format").Funcs(newTemplateFuncs()).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Go template: %v", err)
	}
	return t, nil
}

func newTemplateFromFile(name string) (*template.Template, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("failed to open Go template file %s: %v", name, err)
	}

	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read Go template file %s: %v", name, err)
	}

	return newTemplate(string(b))
}

func newTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"formatId":   FormatId,
		"formatTime": FormatTime,
	}
}
