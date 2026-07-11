package pgd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecute(t *testing.T) {
	assert := assert.New(t)

	execute := func(args ...string) (string, error) {
		buffer := bytes.NewBufferString("")

		d := New("test")

		rootCmd := d.rootCmd
		rootCmd.SetArgs(args)
		rootCmd.SetErr(buffer)
		rootCmd.SetOut(buffer)

		err := rootCmd.Execute()

		return buffer.String(), err
	}

	t.Run("no arguments", func(t *testing.T) {
		out, err := execute("-h")
		assert.True(strings.HasPrefix(out, "Usage:"))
		assert.Nil(err)
	})

	t.Run("help command", func(t *testing.T) {
		out, err := execute("help")
		assert.Contains(out, `Error: unknown command "help"`)
		assert.NotNil(err)
	})

	t.Run("help flag", func(t *testing.T) {
		out, err := execute("-h")
		assert.True(strings.HasPrefix(out, "Usage:"))
		assert.Nil(err)
	})

	t.Run("create-encryption-key", func(t *testing.T) {
		out, err := execute("create-encryption-key")
		assert.Len(out, 44)
		assert.Nil(err)
	})

	t.Run("list-conf", func(t *testing.T) {
		out, err := execute("list-conf")
		assert.Contains(out, "GO_BPMN_ENGINE_ID=default-engine")
		assert.Contains(out, "GO_BPMN_PG_DATABASE_URL=")
		assert.Contains(out, "GO_BPMN_HTTP_BIND_ADDRESS=127.0.0.1:8080")
		assert.Nil(err)
	})

	t.Run("list-conf with env", func(t *testing.T) {
		out, err := execute("list-conf", "--env", "GO_BPMN_ENGINE_ID=engine-id", "-e", "GO_BPMN_PG_DATABASE_URL=pg-database-url")
		assert.Contains(out, "GO_BPMN_ENGINE_ID=engine-id", "should override default value")
		assert.Contains(out, "GO_BPMN_PG_DATABASE_URL=pg-database-url", "should set value")
		assert.Nil(err)
	})

	t.Run("returns error when env is invalid", func(t *testing.T) {
		out, err := execute("list-conf", "--env", "X")
		assert.Contains(out, "Error: --env X: invalid value X: required format KEY=VALUE")
		assert.NotNil(err)
	})

	t.Run("list-conf with env-file", func(t *testing.T) {
		f, err := os.CreateTemp("", "env-")
		if err != nil {
			t.Fatalf("failed to create temporary file: %v", err)
		}

		defer f.Close()
		defer os.Remove(f.Name())

		f.WriteString("GO_BPMN_ENGINE_ID=engine-id\n")
		f.WriteString("GO_BPMN_PG_DATABASE_URL=pg-database-url\n")
		f.WriteString("\n") // empty lines should be ignored

		out, err := execute("list-conf", "--env-file", f.Name())
		assert.Contains(out, "GO_BPMN_ENGINE_ID=engine-id", "should override default value")
		assert.Contains(out, "GO_BPMN_PG_DATABASE_URL=pg-database-url")
		assert.Nil(err)
	})

	t.Run("returns error when env-file not exists", func(t *testing.T) {
		out, err := execute("list-conf", "--env-file", "/tmp/go-bpmn/not-existing")
		assert.Contains(out, "--env-file /tmp/go-bpmn/not-existing:")
		assert.NotNil(err)
	})

	t.Run("returns error when env-file is invalid", func(t *testing.T) {
		f, err := os.CreateTemp("", "env-")
		if err != nil {
			t.Fatalf("failed to create temporary file: %v", err)
		}

		defer f.Close()
		defer os.Remove(f.Name())

		f.WriteString("X\n")

		out, err := execute("list-conf", "--env-file", f.Name())
		assert.Contains(out, "--env-file ")
		assert.Contains(out, "line 1: invalid value X: required format KEY=VALUE")
		assert.NotNil(err)
	})

	t.Run("list-conf-opts", func(t *testing.T) {
		out, err := execute("list-conf-opts")
		assert.Contains(out, "GO_BPMN_ENGINE_ID")
		assert.Contains(out, "GO_BPMN_PG_DATABASE_URL*")
		assert.Contains(out, "GO_BPMN_HTTP_BIND_ADDRESS")
		assert.Nil(err)
	})

	t.Run("version", func(t *testing.T) {
		out, err := execute("version")
		assert.Contains(out, "test")
		assert.Nil(err)
	})

	t.Run("api key", func(t *testing.T) {
		t.Run("no arguments", func(t *testing.T) {
			out, err := execute("api-key", "-h")
			assert.Contains(out, "Usage:")
			assert.Nil(err)
		})

		t.Run("help command", func(t *testing.T) {
			out, err := execute("api-key", "help")
			assert.Contains(out, "Usage:")
			assert.Nil(err)
		})

		t.Run("help flag", func(t *testing.T) {
			out, err := execute("api-key", "-h")
			assert.Contains(out, "Usage:")
			assert.Nil(err)
		})
	})
}
