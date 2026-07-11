package daemon

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/server"
	"github.com/spf13/cobra"
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

func NewConf() Conf {
	conf := Conf{
		Env: Env{},

		opts:    make(map[string]ConfOpt),
		optErrs: make(map[string]error),
	}

	// add engine options
	conf.AddEngineOption(
		optEncryptionKeys,
		"comma-separated list of encryption keys (from new to old)",
		func(o engine.Options) string {
			return ""
		},
		func(o *engine.Options, opt ConfOpt) error {
			encryptionKeys := opt.Value()
			if encryptionKeys == "" {
				return nil
			}

			encryption, err := engine.NewEncryption(encryptionKeys)
			o.Encryption = encryption
			return err
		},
	)

	conf.AddEngineOption(
		optEngineId,
		"ID of the engine",
		func(o engine.Options) string {
			return o.EngineId
		},
		func(o *engine.Options, opt ConfOpt) error {
			engineId := opt.Value()
			if engineId == "" {
				return errors.New("is empty")
			}

			o.EngineId = engineId
			return nil
		},
	)

	conf.AddEngineOption(
		optTaskExecutorEnabled,
		"enable or disable the engine's task executor",
		func(o engine.Options) string {
			return strconv.FormatBool(o.TaskExecutorEnabled)
		},
		func(o *engine.Options, opt ConfOpt) error {
			taskExecutorEnabled, err := strconv.ParseBool(opt.Value())
			o.TaskExecutorEnabled = taskExecutorEnabled
			return err
		},
	)
	conf.AddEngineOption(
		optTaskExecutorInterval,
		"interval between the execution of due tasks",
		func(o engine.Options) string {
			return o.TaskExecutorInterval.String()
		},
		func(o *engine.Options, opt ConfOpt) error {
			taskExecutorInterval, err := time.ParseDuration(opt.Value())
			o.TaskExecutorInterval = taskExecutorInterval
			return err
		},
	)
	conf.AddEngineOption(
		optTaskExecutorLimit,
		"maximum number of due tasks to lock and execute at once",
		func(o engine.Options) string {
			return strconv.Itoa(o.TaskExecutorLimit)
		},
		func(o *engine.Options, opt ConfOpt) error {
			taskExecutorLimit, err := strconv.ParseInt(opt.Value(), 10, 32)
			o.TaskExecutorLimit = int(taskExecutorLimit)
			return err
		},
	)

	conf.AddEngineOption(
		optTaskRetryLimit,
		"maximum number of task retries",
		func(o engine.Options) string {
			return strconv.Itoa(o.TaskRetryLimit)
		},
		func(o *engine.Options, opt ConfOpt) error {
			taskRetryLimit, err := strconv.ParseInt(opt.Value(), 10, 32)
			o.TaskRetryLimit = int(taskRetryLimit)
			return err
		},
	)

	// add server options
	conf.AddServerOption(
		optHttpBindAddress,
		"TCP address of the engine's HTTP API to listen on",
		func(o server.Options) string {
			return o.BindAddress
		},
		func(o *server.Options, opt ConfOpt) error {
			bindAddress := opt.Value()
			if bindAddress == "" {
				return errors.New("is empty")
			}

			o.BindAddress = bindAddress
			return nil
		},
	)

	conf.AddServerOption(
		optHttpReadTimeout,
		"maximum duration for reading the entire request - see http.Server#ReadTimeout",
		func(o server.Options) string {
			return o.ReadTimeout.String()
		},
		func(o *server.Options, opt ConfOpt) error {
			readTimeout, err := time.ParseDuration(opt.Value())
			o.ReadTimeout = readTimeout
			return err
		},
	)

	conf.AddServerOption(
		optHttpWriteTimeout,
		"maximum duration before timing out writing the response - see http.Server#WriteTimeout",
		func(o server.Options) string {
			return o.WriteTimeout.String()
		},
		func(o *server.Options, opt ConfOpt) error {
			writeTimeout, err := time.ParseDuration(opt.Value())
			o.WriteTimeout = writeTimeout
			return err
		},
	)

	conf.AddServerOption(
		optSetTimeEnabled,
		"enable or disable the setTime operation",
		func(o server.Options) string {
			return strconv.FormatBool(o.SetTimeEnabled)
		},
		func(o *server.Options, opt ConfOpt) error {
			setTimeEnabled, err := strconv.ParseBool(opt.Value())
			o.SetTimeEnabled = setTimeEnabled
			return err
		},
	)

	return conf
}

type Conf struct {
	Env Env

	opts    map[string]ConfOpt
	optErrs map[string]error
}

func (c Conf) AddEngineOption(
	key string,
	description string,
	getOption func(engine.Options) string,
	setOption func(*engine.Options, ConfOpt) error,
) {
	opt := ConfOpt{
		env:         c.Env,
		key:         envPrefix + key,
		description: description,

		getEngineOption: getOption,
		setEngineOption: setOption,
	}

	c.opts[key] = opt
}

func (c Conf) AddError(key string, err error) {
	c.optErrs[envPrefix+key] = err
}

func (c Conf) AddOption(key string, description string) {
	opt := ConfOpt{
		env:         c.Env,
		key:         envPrefix + key,
		description: description,
	}

	c.opts[key] = opt
}

func (c Conf) AddServerOption(
	key string,
	description string,
	getOption func(server.Options) string,
	setOption func(*server.Options, ConfOpt) error,
) {
	opt := ConfOpt{
		env:         c.Env,
		key:         envPrefix + key,
		description: description,

		getServerOption: getOption,
		setServerOption: setOption,
	}

	c.opts[key] = opt
}

func (c Conf) Errors() []string {
	var errs []string

	for _, opt := range c.opts {
		err := c.optErrs[opt.key]
		if err == nil && !opt.required {
			continue
		}

		value := opt.Value()

		switch {
		case err != nil && value != "":
			errs = append(errs, fmt.Sprintf("%s=%s: %v", opt.key, value, err))
		case err != nil && value == "":
			errs = append(errs, fmt.Sprintf("%s: %v", opt.key, err))
		case value == "":
			errs = append(errs, fmt.Sprintf("%s is required", opt.key))
		}
	}

	slices.SortFunc(errs, func(a string, b string) int {
		return strings.Compare(a, b)
	})

	return errs
}

func (c Conf) GetOptions(engineOptions *engine.Options, serverOptions *server.Options) {
	for _, opt := range c.opts {
		if opt.setEngineOption != nil {
			if err := opt.setEngineOption(engineOptions, opt); err != nil {
				c.optErrs[opt.key] = err
			}
		}

		if opt.setServerOption != nil {
			if err := opt.setServerOption(serverOptions, opt); err != nil {
				c.optErrs[opt.key] = err
			}
		}
	}
}

func (c Conf) MarkRequired(key string) {
	opt, ok := c.opts[key]
	if !ok {
		return
	}

	opt.required = true
	c.opts[key] = opt
}

func (c Conf) Opt(key string) (ConfOpt, bool) {
	opt, ok := c.opts[key]
	return opt, ok
}

func (c Conf) PrintErrors(cmd *cobra.Command) error {
	errs := c.Errors()
	if len(errs) == 0 {
		return nil
	}

	for _, err := range errs {
		cmd.Println(err)
	}
	cmd.Println()
	return fmt.Errorf("configuration has %d error(s)", len(errs))
}

func (c *Conf) SetDefaults(engineOptions engine.Options, serverOptions server.Options) {
	for key, opt := range c.opts {
		if opt.getEngineOption != nil {
			opt.defaultValue = opt.getEngineOption(engineOptions)
			c.opts[key] = opt
		}

		if opt.getServerOption != nil {
			opt.defaultValue = opt.getServerOption(serverOptions)
			c.opts[key] = opt
		}
	}
}

type ConfOpt struct {
	env          Env
	key          string
	description  string
	required     bool
	defaultValue string

	getEngineOption func(engine.Options) string
	getServerOption func(server.Options) string
	setEngineOption func(*engine.Options, ConfOpt) error
	setServerOption func(*server.Options, ConfOpt) error
}

func (o ConfOpt) Value() string {
	value := o.env[o.key]
	if value != "" {
		return value
	} else {
		return o.defaultValue
	}
}

type Env map[string]string

func (v Env) Set(value string) error {
	s := strings.SplitN(value, "=", 2)
	if len(s) != 2 {
		return fmt.Errorf("invalid value %s: required format KEY=VALUE", value)
	}
	v[s[0]] = s[1]
	return nil
}

func (v Env) SetEnvironment(environment []string) error {
	for _, value := range environment {
		if !strings.HasPrefix(value, envPrefix) {
			continue
		}

		if err := v.Set(value); err != nil {
			return err
		}
	}
	return nil
}

func (v Env) SetFile(name string) error {
	file, err := os.Open(name)
	if err != nil {
		return err
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	i := 0
	for scanner.Scan() {
		i++

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to scan line %d: %v", i, err)
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		if err := v.Set(line); err != nil {
			return fmt.Errorf("line %d: %v", i, err)
		}
	}

	return nil
}
