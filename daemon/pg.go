package daemon

import (
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/pg"
	"github.com/gclaussn/go-bpmn/http/server"
)

func RunPg(args []string) int {
	engineOptions := pg.NewOptions()
	serverOptions := server.NewOptions()

	conf := newConf()

	pgDatabaseUrl := conf.addOption("PG_DATABASE_URL", "format: postgres://<username>:<password>@<host>:<port>/<database>?search_path=<schema>")
	pgDatabaseUrl.required = true

	conf.setEngineOptions(engineOptions.Common)
	conf.setServerOptions(serverOptions)

	flags := flag.NewFlagSet("go-bpmn-pgd", flag.ContinueOnError)
	flags.SetOutput(log.Writer())

	flags.Var(&conf.envFile.env, "env", "set environment variables")
	flags.Var(&conf.envFile, "env-file", "read in a file of environment variables")

	var doCreateEncryptionKey bool
	flags.BoolVar(&doCreateEncryptionKey, "create-encryption-key", false, "create a new encryption key - used for "+conf.opts[optEncryptionKeys].key)
	var doListConfOpts bool
	flags.BoolVar(&doListConfOpts, "list-conf-opts", false, "list configuration options")
	var doListConf bool
	flags.BoolVar(&doListConf, "list-conf", false, "list configuration")
	var doVersion bool
	flags.BoolVar(&doVersion, "version", false, "show version")

	var doCreateApiKey bool
	flags.BoolVar(&doCreateApiKey, "create-api-key", false, "create a new API key")
	var secretId string
	flags.StringVar(&secretId, "secret-id", "", "secret ID, required when creating a new API key")

	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		} else {
			return 1
		}
	}

	if doCreateEncryptionKey {
		return createEncryptionKey()
	}
	if doListConfOpts {
		return listConfOpts(conf)
	}
	if doListConf {
		return listConf(conf)
	}
	if doVersion {
		return showVersion()
	}

	conf.getEngineOptions(&engineOptions.Common)
	conf.getServerOptions(&serverOptions)

	if pgDatabaseUrl.value() == "" {
		pgDatabaseUrl.err = errors.New("is empty")
	}

	if code := listConfErrors(conf); code != 0 {
		return code
	}

	engineStartTime := time.Now()

	e, err := pg.New(pgDatabaseUrl.value(), func(o *pg.Options) {
		*o = engineOptions

		if doCreateApiKey {
			// disable task execution, when the engine is only started for API key creation
			o.Common.TaskExecutorEnabled = false
		}

		o.Common.OnTaskExecutionFailure = func(task engine.Task, err error) {
			log.Printf("failed to execute task %s: %v", task, err)
		}
	})
	if err != nil {
		log.Printf("failed to create pg engine: %v", err)
		return 1
	}

	defer e.Shutdown()

	if !doCreateApiKey { // must not appear in stdout when API key is created
		log.Printf("pg engine started in %dms", time.Since(engineStartTime).Milliseconds())
	}

	apiKeyManager := e.(pg.ApiKeyManager)
	if doCreateApiKey {
		_, authorization, err := apiKeyManager.CreateApiKey(secretId)
		if err != nil {
			log.Printf("failed to create API key: %v", err)
			return 1
		}

		log.SetFlags(0)
		log.Writer().Write([]byte(authorization))
		return 0
	}

	serverOptions.ApiKeyManager = apiKeyManager

	server, err := server.New(e, func(o *server.Options) {
		*o = serverOptions
	})
	if err != nil {
		log.Printf("failed to create HTTP server: %v", err)
		return 1
	}

	server.ListenAndServe()

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt, syscall.SIGTERM)

	<-signalC

	server.Shutdown()

	return 0
}
