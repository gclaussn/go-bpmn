package common

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestFormatter(t *testing.T) {
	assert := assert.New(t)

	buffer := bytes.NewBufferString("")

	cmd := cobra.Command{}
	cmd.SetOut(buffer)

	t.Run("format using default template", func(t *testing.T) {
		buffer.Reset()

		formatter := &Formatter{}
		assert.NoError(formatter.Format(&cmd, 1, "{{ . }}"))
		assert.Equal("1", buffer.String())
	})

	t.Run("format using JSON", func(t *testing.T) {
		buffer.Reset()

		formatter := &Formatter{}
		assert.NoError(formatter.Set(formatJson))
		assert.NoError(formatter.Format(&cmd, 1, ""))
		assert.Equal("1", buffer.String())
	})

	t.Run("format using template string", func(t *testing.T) {
		buffer.Reset()

		formatter := &Formatter{}
		assert.NoError(formatter.Set("{{ . }}"))
		assert.NoError(formatter.Format(&cmd, 1, ""))
		assert.Equal("1", buffer.String())
	})

	t.Run("format using template file", func(t *testing.T) {
		buffer.Reset()

		formatter := &Formatter{}
		assert.NoError(formatter.Set("file://../job/job.tpl"))
		assert.NoError(formatter.Format(&cmd, engine.Job{Id: 1}, ""))
		assert.Contains(buffer.String(), "Id: 1\n")
	})

	t.Run("set returns error, when template is invalid", func(t *testing.T) {
		formatter := &Formatter{}
		assert.Error(formatter.Set("{{}}"))
	})

	t.Run("set returns error, when template file not exists", func(t *testing.T) {
		formatter := &Formatter{}
		assert.Error(formatter.Set("file://./not-existing"))
	})
}

func TestTableFormatter(t *testing.T) {
	assert := assert.New(t)

	buffer := bytes.NewBufferString("")

	cmd := cobra.Command{}
	cmd.SetOut(buffer)

	t.Run("format using default columns", func(t *testing.T) {
		buffer.Reset()

		formatter := NewTableFormatter("a,b,c", "c,a")

		ok, err := formatter.Format(&cmd, nil, nil)
		assert.False(ok)
		assert.NoError(err)

		assert.Equal("", buffer.String())
		assert.Equal("c,a", formatter.SelectedColumns())
	})

	t.Run("format using JSON", func(t *testing.T) {
		buffer.Reset()

		formatter := NewTableFormatter("", "")
		assert.NoError(formatter.Set(formatJson))

		ok, err := formatter.Format(&cmd, []engine.Job{}, nil)
		assert.True(ok)
		assert.NoError(err)

		assert.Equal("[]", buffer.String())
	})

	t.Run("format using JSONL", func(t *testing.T) {
		buffer.Reset()

		formatter := NewTableFormatter("", "")
		assert.NoError(formatter.Set(formatJsonl))

		ok, err := formatter.Format(&cmd, nil, Rows([]engine.Data{
			{Encoding: "a", Value: "b"},
			{Encoding: "c", Value: "d"},
		}))
		assert.True(ok)
		assert.NoError(err)

		lines := strings.Split(buffer.String(), "\n")
		assert.Len(lines, 3)

		assert.Equal(`{"encoding":"a","value":"b"}`, lines[0])
		assert.Equal(`{"encoding":"c","value":"d"}`, lines[1])
		assert.Empty(lines[2])
	})

	t.Run("format rows using template string", func(t *testing.T) {
		buffer.Reset()

		formatter := NewTableFormatter("", "")
		assert.NoError(formatter.Set("{{ .Id }}"))

		ok, err := formatter.Format(&cmd, nil, Rows([]engine.Job{
			{Id: 1},
			{Id: 2},
			{Id: 3},
		}))
		assert.True(ok)
		assert.NoError(err)

		lines := strings.Split(buffer.String(), "\n")
		assert.Len(lines, 4)

		assert.Equal("1", lines[0])
		assert.Equal("2", lines[1])
		assert.Equal("3", lines[2])
		assert.Empty(lines[3])
	})

	t.Run("format rows using template file", func(t *testing.T) {
		f, err := os.CreateTemp("", "tpl-")
		if err != nil {
			t.Fatalf("failed to create temporary file: %v", err)
		}

		defer f.Close()
		defer os.Remove(f.Name())

		f.WriteString("{{ .Id }}")

		buffer.Reset()

		formatter := NewTableFormatter("", "")
		assert.NoError(formatter.Set("file://" + f.Name()))

		ok, err := formatter.Format(&cmd, nil, Rows([]engine.Job{
			{Id: 1},
			{Id: 2},
			{Id: 3},
		}))
		assert.True(ok)
		assert.NoError(err)

		lines := strings.Split(buffer.String(), "\n")
		assert.Len(lines, 4)

		assert.Equal("1", lines[0])
		assert.Equal("2", lines[1])
		assert.Equal("3", lines[2])
		assert.Empty(lines[3])
	})

	t.Run("format table using template string", func(t *testing.T) {
		buffer.Reset()

		formatter := NewTableFormatter("", "")
		assert.NoError(formatter.Set("table:{{ len . }}"))

		ok, err := formatter.Format(&cmd, []engine.Job{
			{Id: 1},
			{Id: 2},
			{Id: 3},
		}, nil)
		assert.True(ok)
		assert.NoError(err)

		assert.Equal("3", buffer.String())
	})

	t.Run("format table using template file", func(t *testing.T) {
		f, err := os.CreateTemp("", "tpl-")
		if err != nil {
			t.Fatalf("failed to create temporary file: %v", err)
		}

		defer f.Close()
		defer os.Remove(f.Name())

		f.WriteString("{{ len . }}")

		buffer.Reset()

		formatter := NewTableFormatter("", "")
		assert.NoError(formatter.Set("table:file://" + f.Name()))

		ok, err := formatter.Format(&cmd, []engine.Job{
			{Id: 1},
			{Id: 2},
			{Id: 3},
		}, nil)
		assert.True(ok)
		assert.NoError(err)

		assert.Equal("3", buffer.String())
	})

	t.Run("format using selected columns", func(t *testing.T) {
		buffer.Reset()

		formatter := NewTableFormatter("a,b,c", "c,a")
		assert.NoError(formatter.Set("columns:b,c"))

		ok, err := formatter.Format(&cmd, nil, nil)
		assert.False(ok)
		assert.NoError(err)

		assert.Equal("", buffer.String())
		assert.Equal("b,c", formatter.SelectedColumns())
	})
}

func TestTable(t *testing.T) {
	assert := assert.New(t)

	buffer := bytes.NewBufferString("")

	cmd := cobra.Command{}
	cmd.SetOut(buffer)

	t.Run("format all columns", func(t *testing.T) {
		buffer.Reset()

		table := NewTable("a,b,c,def")
		table.AddRow([]string{"a1", "b1", "c1", "d1"})
		table.AddRow([]string{"a2", "b2", "c2", "d2"})

		table.Format(&cmd, "a,b,c,def")

		lines := strings.Split(buffer.String(), "\n")
		assert.Len(lines, 5)

		assert.Equal("A    B    C    DEF", lines[0])
		assert.Equal("                  ", lines[1])
		assert.Equal("a1   b1   c1   d1 ", lines[2])
		assert.Equal("a2   b2   c2   d2 ", lines[3])
		assert.Empty(lines[4])
	})

	t.Run("format selected columns", func(t *testing.T) {
		buffer.Reset()

		table := NewTable("a,b,c,def")
		table.AddRow([]string{"a1", "b1", "c1", "d1"})
		table.AddRow([]string{"a2", "b2", "c2", "d2"})

		table.Format(&cmd, "def,a")

		lines := strings.Split(buffer.String(), "\n")
		assert.Len(lines, 5)

		assert.Equal("DEF   A ", lines[0])
		assert.Equal("        ", lines[1])
		assert.Equal("d1    a1", lines[2])
		assert.Equal("d2    a2", lines[3])
		assert.Empty(lines[4])
	})
}
