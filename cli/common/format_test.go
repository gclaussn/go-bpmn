package common

import (
	"bytes"
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

		formatter.Format(&cmd, 1, "{{ . }}")
		assert.Equal("1", buffer.String())
	})

	t.Run("format using JSON", func(t *testing.T) {
		buffer.Reset()

		formatter := &Formatter{}
		assert.NoError(formatter.Set(formatJson))

		formatter.Format(&cmd, 1, "")
		assert.Equal("1", buffer.String())
	})

	t.Run("format using template string", func(t *testing.T) {
		buffer.Reset()

		formatter := &Formatter{}
		assert.NoError(formatter.Set("{{ . }}"))

		formatter.Format(&cmd, 1, "")
		assert.Equal("1", buffer.String())
	})

	t.Run("format using template file", func(t *testing.T) {
		buffer.Reset()

		formatter := &Formatter{}
		assert.NoError(formatter.Set("file://../job/job.tpl"))

		formatter.Format(&cmd, engine.Job{Id: 1}, "")
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
