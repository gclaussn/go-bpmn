package pgd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gclaussn/go-bpmn/daemon"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/pg"
	"github.com/gclaussn/go-bpmn/http/server"
	"github.com/spf13/cobra"
)

func New(version string) *Daemon {
	base := daemon.NewConf()

	d := &Daemon{
		version: version,

		Conf: NewConf(base),

		engineOptions: pg.NewOptions(),
		serverOptions: server.NewOptions(),
	}

	rootCmd := newRootCmd(d)

	var (
		envFiles  []string
		envValues []string
	)

	rootCmd.PersistentFlags().StringSliceVar(&envFiles, "env-file", nil, "read in a file of environment variables")
	rootCmd.PersistentFlags().StringSliceVarP(&envValues, "env", "e", nil, "set environment variable")

	rootCmd.MarkFlagFilename("env-file")

	rootCmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		if _, ok := c.Annotations[daemon.AnnotationConf]; !ok {
			return nil
		}

		c.SilenceUsage = true

		conf := d.Conf

		conf.SetDefaults(d.engineOptions, d.serverOptions)

		base := conf.Base

		base.Env.SetEnvironment(os.Environ())

		for _, file := range envFiles {
			if err := base.Env.SetFile(file); err != nil {
				return fmt.Errorf("--env-file %s: %v", file, err)
			}
		}

		for _, value := range envValues {
			if err := base.Env.Set(value); err != nil {
				return fmt.Errorf("--env %s: %v", value, err)
			}
		}

		conf.GetOptions(&d.engineOptions, &d.serverOptions)

		if _, ok := c.Annotations[daemon.AnnotationValidConf]; !ok {
			return nil
		}

		return base.PrintErrors(c)
	}

	d.rootCmd = rootCmd

	return d
}

type Daemon struct {
	version string

	Conf Conf

	engineOptions pg.Options
	serverOptions server.Options

	rootCmd *cobra.Command
}

func (d *Daemon) EngineOptions() *pg.Options {
	return &d.engineOptions
}

func (d *Daemon) NewEngine() (engine.Engine, error) {
	pgDatabaseUrl, _ := d.Conf.Base.Opt(optPgDatabaseUrl)

	e, err := pg.New(pgDatabaseUrl.Value(), func(o *pg.Options) {
		*o = d.engineOptions

		o.Common.OnTaskExecutionFailure = func(task engine.Task, err error) {
			log.Printf("failed to execute task %s: %v", task, err)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create pg engine: %v", err)
	}

	return e, nil
}

func (d *Daemon) RootCmd() *cobra.Command {
	return d.rootCmd
}

func (d *Daemon) ServerOptions() *server.Options {
	return &d.serverOptions
}

func newRootCmd(d *Daemon) *cobra.Command {
	c := cobra.Command{
		Use:  "go-bpmn-pgd",
		RunE: daemon.Help,
	}

	c.AddCommand(newApiKeyCmd(d))
	c.AddCommand(daemon.NewCreateEncryptionKeyCmd())
	c.AddCommand(daemon.NewListConfCmd(d.Conf.Base))
	c.AddCommand(daemon.NewListConfOptsCmd(d.Conf.Base))
	c.AddCommand(newRunCmd(d))
	c.AddCommand(daemon.NewVersionCmd(d.version))

	c.SetHelpCommand(&cobra.Command{
		Use:    "no-help",
		Hidden: true,
	})

	c.CompletionOptions.HiddenDefaultCmd = true

	return &c
}

func newRunCmd(d *Daemon) *cobra.Command {
	c := cobra.Command{
		Use:         "run",
		Short:       "Run pg engine daemon",
		Annotations: map[string]string{daemon.AnnotationConf: "", daemon.AnnotationValidConf: ""},
		RunE: func(c *cobra.Command, _ []string) error {
			log.SetOutput(os.Stdout)

			startTime := time.Now()

			e, err := d.NewEngine()
			if err != nil {
				return err
			}

			log.Printf("pg engine started in %dms", time.Since(startTime).Milliseconds())

			apiKeyManager := e.(pg.ApiKeyManager)

			d.serverOptions.ApiKeyManager = apiKeyManager

			s, err := server.New(e, func(o *server.Options) {
				*o = d.serverOptions
			})
			if err != nil {
				e.Shutdown()
				return fmt.Errorf("failed to create HTTP server: %v", err)
			}

			s.ListenAndServe()

			signalC := make(chan os.Signal, 1)
			signal.Notify(signalC, os.Interrupt, syscall.SIGTERM)

			<-signalC

			s.Shutdown()
			log.Println("server shut down")
			e.Shutdown()
			log.Println("engine shut down")

			return nil
		},
	}

	return &c
}
