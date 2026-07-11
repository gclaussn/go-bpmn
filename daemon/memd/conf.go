package memd

import (
	"errors"

	"github.com/gclaussn/go-bpmn/daemon"
	"github.com/gclaussn/go-bpmn/engine/mem"
	"github.com/gclaussn/go-bpmn/http/server"
)

const (
	optHttpBasicAuthPassword = "HTTP_BASIC_AUTH_PASSWORD"
	optHttpBasicAuthUsername = "HTTP_BASIC_AUTH_USERNAME"
)

func NewConf(base daemon.Conf) Conf {
	conf := Conf{
		Base: base,

		opts: make(map[string]ConfOpt),
	}

	base.AddServerOption(
		optHttpBasicAuthUsername,
		"username for basic authentication",
		func(o server.Options) string {
			return ""
		},
		func(o *server.Options, opt daemon.ConfOpt) error {
			username := opt.Value()
			if username == "" {
				return errors.New("is empty")
			}

			o.BasicAuthUsername = username
			return nil
		},
	)
	base.AddServerOption(
		optHttpBasicAuthPassword,
		"password for basic authentication",
		func(o server.Options) string {
			return ""
		},
		func(o *server.Options, opt daemon.ConfOpt) error {
			password := opt.Value()
			if password == "" {
				return errors.New("is empty")
			}

			o.BasicAuthPassword = password
			return nil
		},
	)

	base.MarkRequired(optHttpBasicAuthUsername)
	base.MarkRequired(optHttpBasicAuthPassword)

	return conf
}

type Conf struct {
	Base daemon.Conf

	opts map[string]ConfOpt
}

func (c Conf) AddOption(
	key string,
	description string,
	getOption func(mem.Options) string,
	setOption func(*mem.Options, ConfOpt) error,
) {
	opt := ConfOpt{
		env:         c.Base.Env,
		key:         key,
		description: description,

		getEngineOption: getOption,
		setEngineOption: setOption,
	}

	c.opts[key] = opt
	c.Base.AddOption(key, description)
}

func (c Conf) GetOptions(engineOptions *mem.Options, serverOptions *server.Options) {
	c.Base.GetOptions(&engineOptions.Common, serverOptions)

	for _, opt := range c.opts {
		if opt.setEngineOption != nil {
			if err := opt.setEngineOption(engineOptions, opt); err != nil {
				c.Base.AddError(opt.key, err)
			}
		}
	}
}

func (c Conf) SetDefaults(engineOptions mem.Options, serverOptions server.Options) {
	c.Base.SetDefaults(engineOptions.Common, serverOptions)

	for key, opt := range c.opts {
		if opt.getEngineOption != nil {
			opt.defaultValue = opt.getEngineOption(engineOptions)
			c.opts[key] = opt
		}
	}
}

type ConfOpt struct {
	env          daemon.Env
	key          string
	description  string
	defaultValue string

	getEngineOption func(mem.Options) string
	setEngineOption func(*mem.Options, ConfOpt) error
}
