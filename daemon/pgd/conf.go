package pgd

import (
	"github.com/gclaussn/go-bpmn/daemon"
	"github.com/gclaussn/go-bpmn/engine/pg"
	"github.com/gclaussn/go-bpmn/http/server"
)

const (
	optPgDatabaseUrl = "PG_DATABASE_URL"
)

func NewConf(base daemon.Conf) Conf {
	conf := Conf{
		Base: base,

		opts: make(map[string]ConfOpt),
	}

	base.AddOption(
		optPgDatabaseUrl,
		"pgx connection string - format: postgres://USERNAME:PASSWORD@HOST:PORT/DATABASE?search_path=SCHEMA",
	)

	base.MarkRequired(optPgDatabaseUrl)

	return conf
}

type Conf struct {
	Base daemon.Conf

	opts map[string]ConfOpt
}

func (c Conf) AddOption(
	key string,
	description string,
	getOption func(pg.Options) string,
	setOption func(*pg.Options, ConfOpt) error,
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

func (c Conf) GetOptions(engineOptions *pg.Options, serverOptions *server.Options) {
	c.Base.GetOptions(&engineOptions.Common, serverOptions)

	for _, opt := range c.opts {
		if opt.setEngineOption != nil {
			if err := opt.setEngineOption(engineOptions, opt); err != nil {
				c.Base.AddError(opt.key, err)
			}
		}
	}
}

func (c Conf) SetDefaults(engineOptions pg.Options, serverOptions server.Options) {
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

	getEngineOption func(pg.Options) string
	setEngineOption func(*pg.Options, ConfOpt) error
}
