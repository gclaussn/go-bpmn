package daemon

import (
	"bytes"
	"log"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/server"
	"github.com/stretchr/testify/assert"
)

func TestConf(t *testing.T) {
	assert := assert.New(t)

	t.Run("get engine options", func(t *testing.T) {
		encryptionKey, _ := engine.NewEncryptionKey()

		conf := newConf()
		conf.opts[optEncryptionKeys].defaultValue = encryptionKey
		conf.opts[optEngineId].defaultValue = "engine-id"
		conf.opts[optJobRetryCount].defaultValue = "3"
		conf.opts[optJobRetryTimer].defaultValue = "PT1M"
		conf.opts[optTaskExecutorEnabled].defaultValue = "true"
		conf.opts[optTaskExecutorInterval].defaultValue = (30 * time.Second).String()
		conf.opts[optTaskExecutorLimit].defaultValue = "100"

		var options engine.Options
		conf.getEngineOptions(&options)

		assert.NotEqual(engine.Encryption{}, options.Encryption)
		assert.Equal("engine-id", options.EngineId)
		assert.Equal(3, options.JobRetryCount)
		assert.Equal("PT1M", options.JobRetryTimer.String())
		assert.True(options.TaskExecutorEnabled)
		assert.Equal("30s", options.TaskExecutorInterval.String())
		assert.Equal(100, options.TaskExecutorLimit)

		assert.Equal(0, listConfErrors(conf))
	})

	t.Run("get engine options when values are invalid", func(t *testing.T) {
		conf := newConf()
		conf.opts[optEncryptionKeys].defaultValue = "invalid-encryption-key"
		conf.opts[optEngineId].defaultValue = ""
		conf.opts[optJobRetryCount].defaultValue = "invalid-job-retry-count"
		conf.opts[optJobRetryTimer].defaultValue = "invalid-job-retry-timer"
		conf.opts[optTaskExecutorEnabled].defaultValue = "invalid-task-executor-enabled"
		conf.opts[optTaskExecutorInterval].defaultValue = "invalid-task-executor-interval"
		conf.opts[optTaskExecutorLimit].defaultValue = "invalid-task-executor-limit"

		conf.getEngineOptions(&engine.Options{})

		assert.NotNil(conf.opts[optEncryptionKeys].err)
		assert.NotNil(conf.opts[optEngineId].err)
		assert.NotNil(conf.opts[optJobRetryCount].err)
		assert.NotNil(conf.opts[optJobRetryTimer].err)
		assert.NotNil(conf.opts[optTaskExecutorEnabled].err)
		assert.NotNil(conf.opts[optTaskExecutorInterval].err)
		assert.NotNil(conf.opts[optTaskExecutorLimit].err)

		buffer := bytes.NewBufferString("")
		log.SetOutput(buffer)

		assert.Equal(1, listConfErrors(conf))

		assert.Contains(buffer.String(), "GO_BPMN_ENCRYPTION_KEYS=invalid-encryption-key: ")
		assert.Contains(buffer.String(), "GO_BPMN_ENGINE_ID: ")
		assert.Contains(buffer.String(), "GO_BPMN_JOB_RETRY_COUNT=invalid-job-retry-count: ")
		assert.Contains(buffer.String(), "GO_BPMN_JOB_RETRY_TIMER=invalid-job-retry-timer: ")
		assert.Contains(buffer.String(), "GO_BPMN_TASK_EXECUTOR_ENABLED=invalid-task-executor-enabled: ")
		assert.Contains(buffer.String(), "GO_BPMN_TASK_EXECUTOR_INTERVAL=invalid-task-executor-interval: ")
		assert.Contains(buffer.String(), "GO_BPMN_TASK_EXECUTOR_LIMIT=invalid-task-executor-limit: ")
	})

	t.Run("get server options", func(t *testing.T) {
		conf := newConf()
		conf.opts[optHttpBindAddress].defaultValue = "192.168.0.10:8080"
		conf.opts[optHttpReadTimeout].defaultValue = "5s"
		conf.opts[optHttpWriteTimeout].defaultValue = "35s"
		conf.opts[optSetTimeEnabled].defaultValue = "true"

		var options server.Options
		conf.getServerOptions(&options)

		assert.Equal("192.168.0.10:8080", options.BindAddress)
		assert.True(options.SetTimeEnabled)

		assert.Equal(0, listConfErrors(conf))
	})

	t.Run("get server options when values are invalid", func(t *testing.T) {
		conf := newConf()
		conf.opts[optHttpBindAddress].defaultValue = ""
		conf.opts[optSetTimeEnabled].defaultValue = "invalid"

		conf.getServerOptions(&server.Options{})

		assert.NotNil(conf.opts[optHttpBindAddress].err)
		assert.NotNil(conf.opts[optSetTimeEnabled].err)

		buffer := bytes.NewBufferString("")
		log.SetOutput(buffer)

		assert.Equal(1, listConfErrors(conf))

		assert.Contains(buffer.String(), "GO_BPMN_HTTP_BIND_ADDRESS: ")
		assert.Contains(buffer.String(), "GO_BPMN_HTTP_READ_TIMEOUT: ")
		assert.Contains(buffer.String(), "GO_BPMN_HTTP_WRITE_TIMEOUT: ")
		assert.Contains(buffer.String(), "GO_BPMN_SET_TIME_ENABLED=invalid: ")
	})
}
