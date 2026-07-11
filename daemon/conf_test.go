package daemon

import (
	"strings"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/server"
	"github.com/stretchr/testify/assert"
)

func TestConf(t *testing.T) {
	assert := assert.New(t)

	t.Run("get options", func(t *testing.T) {
		encryptionKey, _ := engine.NewEncryptionKey()

		conf := NewConf()

		conf.Env[envPrefix+optEncryptionKeys] = encryptionKey
		conf.Env[envPrefix+optEngineId] = "engine-id"
		conf.Env[envPrefix+optTaskExecutorEnabled] = "true"
		conf.Env[envPrefix+optTaskExecutorInterval] = (30 * time.Second).String()
		conf.Env[envPrefix+optTaskExecutorLimit] = "100"
		conf.Env[envPrefix+optTaskRetryLimit] = "3"

		conf.Env[envPrefix+optHttpBindAddress] = "192.168.0.10:8080"
		conf.Env[envPrefix+optHttpReadTimeout] = "5s"
		conf.Env[envPrefix+optHttpWriteTimeout] = "35s"
		conf.Env[envPrefix+optSetTimeEnabled] = "true"

		var (
			engineOptions engine.Options
			serverOptions server.Options
		)
		conf.GetOptions(&engineOptions, &serverOptions)

		assert.False(engineOptions.Encryption.IsZero())
		assert.Equal("engine-id", engineOptions.EngineId)
		assert.True(engineOptions.TaskExecutorEnabled)
		assert.Equal("30s", engineOptions.TaskExecutorInterval.String())
		assert.Equal(100, engineOptions.TaskExecutorLimit)
		assert.Equal(3, engineOptions.TaskRetryLimit)

		assert.Equal("192.168.0.10:8080", serverOptions.BindAddress)
		assert.True(serverOptions.SetTimeEnabled)

		assert.Empty(conf.Errors())
	})

	t.Run("get options when values are invalid", func(t *testing.T) {
		conf := NewConf()

		conf.Env[envPrefix+optEncryptionKeys] = "invalid-encryption-key"
		conf.Env[envPrefix+optEngineId] = ""
		conf.Env[envPrefix+optTaskExecutorEnabled] = "invalid-task-executor-enabled"
		conf.Env[envPrefix+optTaskExecutorInterval] = "invalid-task-executor-interval"
		conf.Env[envPrefix+optTaskExecutorLimit] = "invalid-task-executor-limit"
		conf.Env[envPrefix+optTaskRetryLimit] = "invalid-task-retry-limit"

		conf.Env[envPrefix+optHttpBindAddress] = ""
		conf.Env[envPrefix+optSetTimeEnabled] = "invalid-set-time-enabled"

		var (
			engineOptions engine.Options
			serverOptions server.Options
		)
		conf.GetOptions(&engineOptions, &serverOptions)

		assert.NotNil(conf.optErrs[envPrefix+optEncryptionKeys])
		assert.NotNil(conf.optErrs[envPrefix+optEngineId])
		assert.NotNil(conf.optErrs[envPrefix+optTaskExecutorEnabled])
		assert.NotNil(conf.optErrs[envPrefix+optTaskExecutorInterval])
		assert.NotNil(conf.optErrs[envPrefix+optTaskExecutorLimit])
		assert.NotNil(conf.optErrs[envPrefix+optTaskRetryLimit])

		assert.NotNil(conf.optErrs[envPrefix+optHttpBindAddress])
		assert.NotNil(conf.optErrs[envPrefix+optSetTimeEnabled])
		assert.NotEmpty(conf.Errors())

		errors := strings.Join(conf.Errors(), "\n")

		assert.Contains(errors, "GO_BPMN_ENCRYPTION_KEYS=invalid-encryption-key: ")
		assert.Contains(errors, "GO_BPMN_ENGINE_ID: ")
		assert.Contains(errors, "GO_BPMN_TASK_EXECUTOR_ENABLED=invalid-task-executor-enabled: ")
		assert.Contains(errors, "GO_BPMN_TASK_EXECUTOR_INTERVAL=invalid-task-executor-interval: ")
		assert.Contains(errors, "GO_BPMN_TASK_EXECUTOR_LIMIT=invalid-task-executor-limit: ")
		assert.Contains(errors, "GO_BPMN_TASK_RETRY_LIMIT=invalid-task-retry-limit: ")

		assert.Contains(errors, "GO_BPMN_HTTP_BIND_ADDRESS: ")
		assert.Contains(errors, "GO_BPMN_HTTP_READ_TIMEOUT: ")
		assert.Contains(errors, "GO_BPMN_HTTP_WRITE_TIMEOUT: ")
		assert.Contains(errors, "GO_BPMN_SET_TIME_ENABLED=invalid-set-time-enabled: ")
	})
}
