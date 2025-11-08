package daemon

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/server"
)

const (
	envPrefix = "GO_BPMN_"

	optEncryptionKeys       = "ENCRYPTION_KEYS"
	optEngineId             = "ENGINE_ID"
	optTaskExecutorEnabled  = "TASK_EXECUTOR_ENABLED"
	optTaskExecutorInterval = "TASK_EXECUTOR_INTERVAL"
	optTaskExecutorLimit    = "TASK_EXECUTOR_LIMIT"
	optTaskRetryLimit       = "TASK_RETRY_LIMIT"

	optHttpBindAddress  = "HTTP_BIND_ADDRESS"
	optHttpReadTimeout  = "HTTP_READ_TIMEOUT"
	optHttpWriteTimeout = "HTTP_WRITE_TIMEOUT"
	optSetTimeEnabled   = "SET_TIME_ENABLED"
)

var (
	version = "unknown-version"
)

func newConf() *conf {
	env := env{}
	for _, value := range os.Environ() {
		env.Set(value)
	}

	conf := conf{
		envFile: envFile{env},
		opts:    make(map[string]*confOpt),
	}

	conf.addEngineOption(
		optEncryptionKeys,
		"comma-separated list of encryption keys (from new to old)",
		func(o engine.Options) string {
			return ""
		},
		func(o *engine.Options, co *confOpt) error {
			encryptionKeys := co.value()
			if encryptionKeys == "" {
				return nil
			}

			encryption, err := engine.NewEncryption(encryptionKeys)
			o.Encryption = encryption
			return err
		},
	)
	conf.addEngineOption(
		optEngineId,
		"ID of the engine",
		func(o engine.Options) string {
			return o.EngineId
		},
		func(o *engine.Options, co *confOpt) error {
			engineId := co.value()
			if engineId == "" {
				return errors.New("is empty")
			}

			o.EngineId = engineId
			return nil
		},
	)
	conf.addEngineOption(
		optTaskExecutorEnabled,
		"enable or disable the engine's task executor",
		func(o engine.Options) string {
			return strconv.FormatBool(o.TaskExecutorEnabled)
		},
		func(o *engine.Options, co *confOpt) error {
			taskExecutorEnabled, err := strconv.ParseBool(co.value())
			o.TaskExecutorEnabled = taskExecutorEnabled
			return err
		},
	)
	conf.addEngineOption(
		optTaskExecutorInterval,
		"interval between the execution of due tasks",
		func(o engine.Options) string {
			return o.TaskExecutorInterval.String()
		},
		func(o *engine.Options, co *confOpt) error {
			taskExecutorInterval, err := time.ParseDuration(co.value())
			o.TaskExecutorInterval = taskExecutorInterval
			return err
		},
	)
	conf.addEngineOption(
		optTaskExecutorLimit,
		"maximum number of due tasks to lock and execute at once",
		func(o engine.Options) string {
			return strconv.Itoa(o.TaskExecutorLimit)
		},
		func(o *engine.Options, co *confOpt) error {
			taskExecutorLimit, err := strconv.ParseInt(co.value(), 10, 32)
			o.TaskExecutorLimit = int(taskExecutorLimit)
			return err
		},
	)
	conf.addEngineOption(
		optTaskRetryLimit,
		"maximum number of task retries",
		func(o engine.Options) string {
			return strconv.Itoa(o.TaskRetryLimit)
		},
		func(o *engine.Options, co *confOpt) error {
			taskRetryLimit, err := strconv.ParseInt(co.value(), 10, 32)
			o.TaskRetryLimit = int(taskRetryLimit)
			return err
		},
	)

	conf.addServerOption(
		optHttpBindAddress,
		"TCP address of the engine's HTTP API to listen on",
		func(o server.Options) string {
			return o.BindAddress
		},
		func(o *server.Options, co *confOpt) error {
			bindAddress := co.value()
			if bindAddress == "" {
				return errors.New("is empty")
			}

			o.BindAddress = bindAddress
			return nil
		},
	)
	conf.addServerOption(
		optHttpReadTimeout,
		"maximum duration for reading the entire request - see http.Server#ReadTimeout",
		func(o server.Options) string {
			return o.ReadTimeout.String()
		},
		func(o *server.Options, co *confOpt) error {
			readTimeout, err := time.ParseDuration(co.value())
			o.ReadTimeout = readTimeout
			return err
		},
	)
	conf.addServerOption(
		optHttpWriteTimeout,
		"maximum duration before timing out writing the response - see http.Server#WriteTimeout",
		func(o server.Options) string {
			return o.WriteTimeout.String()
		},
		func(o *server.Options, co *confOpt) error {
			writeTimeout, err := time.ParseDuration(co.value())
			o.WriteTimeout = writeTimeout
			return err
		},
	)
	conf.addServerOption(
		optSetTimeEnabled,
		"enable or disable the setTime operation",
		func(o server.Options) string {
			return strconv.FormatBool(o.SetTimeEnabled)
		},
		func(o *server.Options, co *confOpt) error {
			setTimeEnabled, err := strconv.ParseBool(co.value())
			o.SetTimeEnabled = setTimeEnabled
			return err
		},
	)

	return &conf
}

func createEncryptionKey() int {
	encryptionKey, err := engine.NewEncryptionKey()
	if err != nil {
		log.Printf("failed to create encryption key: %v", err)
		return 1
	}

	log.SetFlags(0)
	log.Writer().Write([]byte(encryptionKey))
	return 0
}

func listConf(conf *conf) int {
	opts := make([]*confOpt, len(conf.opts))

	i := 0
	for _, opt := range conf.opts {
		opts[i] = opt
		i++
	}

	slices.SortFunc(opts, func(a *confOpt, b *confOpt) int {
		return strings.Compare(a.key, b.key)
	})

	log.SetFlags(0)
	for _, opt := range opts {
		log.Printf("%s=%s", opt.key, opt.value())
	}

	return 0
}

func listConfErrors(conf *conf) int {
	var opts []*confOpt

	for _, opt := range conf.opts {
		if opt.err != nil {
			opts = append(opts, opt)
		}
	}

	if len(opts) == 0 {
		return 0
	}

	slices.SortFunc(opts, func(a *confOpt, b *confOpt) int {
		return strings.Compare(a.key, b.key)
	})

	log.SetFlags(0)
	for _, opt := range opts {
		value := opt.value()
		if value == "" {
			log.Printf("%s: %v", opt.key, opt.err)
		} else {
			log.Printf("%s=%s: %v", opt.key, value, opt.err)
		}
	}

	return 1
}

func listConfOpts(conf *conf) int {
	opts := make([]*confOpt, len(conf.opts))

	i := 0
	for _, opt := range conf.opts {
		opts[i] = opt
		i++
	}

	slices.SortFunc(opts, func(a *confOpt, b *confOpt) int {
		return strings.Compare(a.key, b.key)
	})

	maxKeyLength := 0
	for _, opt := range opts {
		keyLength := len(opt.key)
		if opt.required {
			keyLength++
		}

		if keyLength > maxKeyLength {
			maxKeyLength = keyLength
		}
	}

	var sb strings.Builder
	for _, opt := range opts {
		sb.WriteString(opt.key)

		l := len(opt.key)
		if opt.required {
			sb.WriteRune('*')
			l++
		}

		sb.WriteString(strings.Repeat(" ", maxKeyLength-l))
		sb.WriteString("   ")
		sb.WriteString(opt.description)

		if opt.defaultValue != "" {
			sb.WriteString(fmt.Sprintf(" - default: %s", opt.defaultValue))
		}

		sb.WriteRune('\n')
	}

	log.SetFlags(0)
	log.Print(sb.String())

	return 0
}

func showVersion() int {
	log.Println(version)
	return 0
}

type conf struct {
	envFile envFile
	opts    map[string]*confOpt
}

func (c *conf) addEngineOption(
	key string,
	description string,
	getOption func(engine.Options) string,
	setOption func(*engine.Options, *confOpt) error,
) *confOpt {
	co := confOpt{
		env:         c.envFile.env,
		key:         envPrefix + key,
		description: description,

		getEngineOption: getOption,
		setEngineOption: setOption,
	}

	c.opts[key] = &co
	return &co
}

func (c *conf) addOption(key string, description string) *confOpt {
	co := confOpt{
		env:         c.envFile.env,
		key:         envPrefix + key,
		description: description,
	}

	c.opts[key] = &co
	return &co
}

func (c *conf) addServerOption(
	key string,
	description string,
	getOption func(server.Options) string,
	setOption func(*server.Options, *confOpt) error,
) *confOpt {
	co := confOpt{
		env:         c.envFile.env,
		key:         envPrefix + key,
		description: description,

		getServerOption: getOption,
		setServerOption: setOption,
	}

	c.opts[key] = &co
	return &co
}

func (c *conf) getEngineOptions(options *engine.Options) {
	for _, opt := range c.opts {
		if opt.setEngineOption != nil {
			if err := opt.setEngineOption(options, opt); err != nil {
				opt.err = err
			}
		}
	}
}

func (c *conf) getServerOptions(options *server.Options) {
	for _, opt := range c.opts {
		if opt.setServerOption != nil {
			if err := opt.setServerOption(options, opt); err != nil {
				opt.err = err
			}
		}
	}
}

func (c *conf) setEngineOptions(options engine.Options) {
	for _, opt := range c.opts {
		if opt.getEngineOption != nil {
			opt.defaultValue = opt.getEngineOption(options)
		}
	}
}

func (c *conf) setServerOptions(options server.Options) {
	for _, opt := range c.opts {
		if opt.getServerOption != nil {
			opt.defaultValue = opt.getServerOption(options)
		}
	}
}

type confOpt struct {
	env env

	key          string
	description  string
	required     bool
	defaultValue string

	getEngineOption func(engine.Options) string
	getServerOption func(server.Options) string
	setEngineOption func(*engine.Options, *confOpt) error
	setServerOption func(*server.Options, *confOpt) error

	err error
}

func (o *confOpt) value() string {
	value := o.env[o.key]
	if value != "" {
		return value
	} else {
		return o.defaultValue
	}
}

type env map[string]string

func (v env) Set(value string) error {
	s := strings.SplitN(value, "=", 2)
	if len(s) != 2 {
		return fmt.Errorf("required format %s", v)
	}
	v[s[0]] = s[1]
	return nil
}

func (v env) String() string {
	return "<key>=<value>"
}

type envFile struct {
	env env
}

func (v envFile) Set(value string) error {
	file, err := os.Open(value)
	if err != nil {
		return err
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	i := 0
	for scanner.Scan() {
		i++
		line := scanner.Text()
		if err := v.env.Set(line); err != nil {
			return fmt.Errorf("wrong format in line %d: required format %s", i, v.env)
		}
	}

	return nil
}

func (v envFile) String() string {
	return "<file>"
}
