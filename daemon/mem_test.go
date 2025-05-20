package daemon

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunMem(t *testing.T) {
	assert := assert.New(t)

	buffer := bytes.NewBufferString("")
	log.SetOutput(buffer)

	t.Run("help", func(t *testing.T) {
		assert.Equal(0, RunMem([]string{"-h"}))
	})

	t.Run("create-encryption-key", func(t *testing.T) {
		buffer.Reset()
		assert.Equal(0, RunMem([]string{"-create-encryption-key"}))

		assert.Len(buffer.String(), 44)
	})

	t.Run("list-conf-opts", func(t *testing.T) {
		buffer.Reset()
		assert.Equal(0, RunMem([]string{"-list-conf-opts"}))

		assert.Contains(buffer.String(), "GO_BPMN_ENGINE_ID")
		assert.Contains(buffer.String(), "GO_BPMN_HTTP_BASIC_AUTH_USERNAME*")
		assert.Contains(buffer.String(), "GO_BPMN_HTTP_BASIC_AUTH_PASSWORD*")
		assert.Contains(buffer.String(), "GO_BPMN_HTTP_BIND_ADDRESS")
	})

	t.Run("list-conf", func(t *testing.T) {
		buffer.Reset()
		assert.Equal(0, RunMem([]string{"-list-conf"}))

		assert.Contains(buffer.String(), "GO_BPMN_ENGINE_ID=default-engine")
		assert.Contains(buffer.String(), "GO_BPMN_HTTP_BASIC_AUTH_USERNAME=")
		assert.Contains(buffer.String(), "GO_BPMN_HTTP_BASIC_AUTH_PASSWORD=")
		assert.Contains(buffer.String(), "GO_BPMN_HTTP_BIND_ADDRESS=127.0.0.1:8080")
	})

	t.Run("list-conf with env", func(t *testing.T) {
		buffer.Reset()
		assert.Equal(0, RunMem([]string{"-env", "GO_BPMN_ENGINE_ID=test-engine", "-env", "GO_BPMN_HTTP_BASIC_AUTH_PASSWORD=test-password", "-list-conf"}))

		assert.Contains(buffer.String(), "GO_BPMN_ENGINE_ID=test-engine", "should override default value")
		assert.Contains(buffer.String(), "GO_BPMN_HTTP_BASIC_AUTH_PASSWORD=test-password", "should set value")
	})

	t.Run("returns 1 when env is invalid", func(t *testing.T) {
		buffer.Reset()
		assert.Equal(1, RunMem([]string{"-env", "X"}))

		assert.Contains(buffer.String(), `invalid value "X" for flag -env: required format <key>=<value>`)
	})

	t.Run("list-conf with env-file", func(t *testing.T) {
		f, err := os.CreateTemp("", "env-")
		if err != nil {
			t.Fatalf("failed to create temporary file: %v", err)
		}

		defer f.Close()
		defer os.Remove(f.Name())

		f.WriteString("GO_BPMN_ENGINE_ID=test-engine\n")
		f.WriteString("GO_BPMN_HTTP_BASIC_AUTH_PASSWORD=test-password\n")

		buffer.Reset()
		assert.Equal(0, RunMem([]string{"-env-file", f.Name(), "-list-conf"}))

		assert.Contains(buffer.String(), "GO_BPMN_ENGINE_ID=test-engine", "should override default value")
		assert.Contains(buffer.String(), "GO_BPMN_HTTP_BASIC_AUTH_PASSWORD=test-password")
	})

	t.Run("returns 1 when env-file not exists", func(t *testing.T) {
		buffer.Reset()
		assert.Equal(1, RunMem([]string{"-env-file", "/tmp/go-bpmn/not-existing"}))

		assert.Contains(buffer.String(), `invalid value "/tmp/go-bpmn/not-existing" for flag -env-file`)
	})

	t.Run("returns 1 when env-file is invalid", func(t *testing.T) {
		f, err := os.CreateTemp("", "env-")
		if err != nil {
			t.Fatalf("failed to create temporary file: %v", err)
		}

		defer f.Close()
		defer os.Remove(f.Name())

		f.WriteString("X\n")

		buffer.Reset()
		assert.Equal(1, RunMem([]string{"-env-file", f.Name()}))

		assert.Contains(buffer.String(), "for flag -env-file: wrong format in line 1: required format <key>=<value>")
	})

	t.Run("version", func(t *testing.T) {
		buffer.Reset()
		assert.Equal(0, RunMem([]string{"-version"}))

		assert.Contains(buffer.String(), version)
	})
}
