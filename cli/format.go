package cli

import (
	"strings"
	"time"
	"unicode/utf8"
)

func formatTime(v time.Time) string {
	if v.IsZero() {
		return ""
	}
	return v.Format(time.RFC3339)
}

func formatTimeOrNil(v *time.Time) string {
	if v == nil {
		return ""
	}
	return formatTime(*v)
}

func newTable(headers []string) table {
	rows := make([][]string, 2)
	rows[0] = headers
	rows[1] = make([]string, len(headers))

	return table{rows: rows}
}

type table struct {
	rows [][]string
}

func (t *table) addRow(row []string) {
	t.rows = append(t.rows, row)
}

func (t *table) format() string {
	rows := t.rows

	columns := make([]int, len(rows[0]))
	for i := 0; i < len(rows); i++ {
		for j := 0; j < len(columns); j++ {
			l := utf8.RuneCountInString(rows[i][j])
			if columns[j] < l {
				columns[j] = l
			}
		}
	}

	var sb strings.Builder
	for i := 0; i < len(rows); i++ {
		for j := 0; j < len(columns); j++ {
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
