package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecute(t *testing.T) {
	assert := assert.New(t)

	execute := func(args ...string) (string, error) {
		buffer := bytes.NewBufferString("")

		cli := New("test")

		rootCmd := cli.RootCmd()
		rootCmd.SetArgs(args)
		rootCmd.SetErr(buffer)
		rootCmd.SetOut(buffer)

		err := rootCmd.Execute()

		return buffer.String(), err
	}

	t.Run("no arguments", func(t *testing.T) {
		out, err := execute()
		assert.Contains(out, "Usage:")
		assert.Nil(err)
	})

	t.Run("help command", func(t *testing.T) {
		out, err := execute("help")
		assert.Contains(out, `Error: unknown command "help"`)
		assert.NotNil(err)
	})

	t.Run("help flag", func(t *testing.T) {
		out, err := execute("-h")
		assert.Contains(out, "Usage:")
		assert.Nil(err)
	})

	subCommands := []string{
		"element",
		"element-instance",
		"incident",
		"job",
		"message",
		"process",
		"process-instance",
		"signal",
		"task",
		"user-task",
		"variable",
	}

	for _, subCommand := range subCommands {
		t.Run(subCommand+" help", func(t *testing.T) {
			out, err := execute(subCommand)
			assert.Contains(out, "Usage:")
			assert.Nil(err)
		})
	}

	t.Run("set-time help", func(t *testing.T) {
		out, err := execute("set-time", "-h")
		assert.Contains(out, "Usage:")
		assert.Nil(err)
	})

	t.Run("version", func(t *testing.T) {
		out, err := execute("version")
		assert.Contains(out, "test")
		assert.Nil(err)
	})
}
