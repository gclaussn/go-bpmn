package pgd

import (
	"context"
	"fmt"

	"github.com/gclaussn/go-bpmn/daemon"
	"github.com/gclaussn/go-bpmn/engine/pg"
	"github.com/spf13/cobra"
)

func newApiKeyCmd(d *Daemon) *cobra.Command {
	c := cobra.Command{
		Use:   "api-key",
		Short: "Manage API keys",
		RunE:  daemon.Help,
	}

	c.AddCommand(newApiKeyCreateCmd(d))

	return &c
}

func newApiKeyCreateCmd(d *Daemon) *cobra.Command {
	var secretId string

	c := cobra.Command{
		Use:         "create",
		Short:       "Create a new API key",
		Annotations: map[string]string{daemon.AnnotationConf: "", daemon.AnnotationValidConf: ""},
		RunE: func(c *cobra.Command, _ []string) error {
			// ensure task executor is disabled, when engine is started for API key management
			options := &d.engineOptions.Common
			options.TaskExecutorEnabled = false

			e, err := d.NewEngine()
			if err != nil {
				return err
			}

			defer e.Shutdown()

			_, authorization, err := e.(pg.ApiKeyManager).CreateApiKey(context.Background(), secretId)
			if err != nil {
				return fmt.Errorf("failed to create API key: %v", err)
			}

			c.Print(authorization)
			return nil
		},
	}

	c.Flags().StringVar(&secretId, "secret-id", "", "secret ID")

	c.MarkFlagRequired("secret-id")

	return &c
}
