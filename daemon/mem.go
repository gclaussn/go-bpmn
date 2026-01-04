package daemon

import (
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/mem"
	"github.com/gclaussn/go-bpmn/http/server"
)

func RunMem(args []string) int {
	engineOptions := mem.NewOptions()
	serverOptions := server.NewOptions()

	conf := newConf()

	httpBasicAuthUsername := conf.addServerOption(
		"HTTP_BASIC_AUTH_USERNAME",
		"username for basic authentication",
		func(o server.Options) string {
			return ""
		},
		func(o *server.Options, co *confOpt) error {
			username := co.value()
			if username == "" {
				return errors.New("is empty")
			}

			o.BasicAuthUsername = username
			return nil
		},
	)
	httpBasicAuthUsername.required = true

	httpBasicAuthPassword := conf.addServerOption(
		"HTTP_BASIC_AUTH_PASSWORD",
		"password for basic authentication",
		func(o server.Options) string {
			return ""
		},
		func(o *server.Options, co *confOpt) error {
			password := co.value()
			if password == "" {
				return errors.New("is empty")
			}

			o.BasicAuthPassword = password
			return nil
		},
	)
	httpBasicAuthPassword.required = true

	conf.setEngineOptions(engineOptions.Common)
	conf.setServerOptions(serverOptions)

	flags := flag.NewFlagSet("go-bpmn-memd", flag.ContinueOnError)
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

	if code := listConfErrors(conf); code != 0 {
		return code
	}

	e, err := mem.New(func(o *mem.Options) {
		*o = engineOptions

		o.Common.OnTaskExecutionFailure = func(task engine.Task, err error) {
			log.Printf("failed to execute task %s: %v", task, err)
		}
	})
	if err != nil {
		log.Printf("failed to create mem engine: %v", err)
		return 1
	}

	defer e.Shutdown()

	s, err := server.New(e, func(o *server.Options) {
		*o = serverOptions
	})
	if err != nil {
		log.Printf("failed to create HTTP server: %v", err)
		return 1
	}

	s.ListenAndServe()

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt, syscall.SIGTERM)

	<-signalC

	s.Shutdown()
	log.Println("server shut down")
	e.Shutdown()
	log.Println("engine shut down")

	return 0
}
