package cli

import (
	"encoding/json"
	"fmt"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newEventCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "event",
		Short:       "Create and query events",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newSendSignalCmd(cli))

	return &c
}

func newSendSignalCmd(cli *Cli) *cobra.Command {
	var (
		variablesV map[string]string

		cmd engine.SendSignalCmd
	)

	c := cobra.Command{
		Use:   "send-signal",
		Short: "Send a signal",
		RunE: func(c *cobra.Command, _ []string) error {
			variables := make(map[string]*engine.Data)
			for variableName, dataJson := range variablesV {
				if dataJson == "" || dataJson == "null" {
					variables[variableName] = nil
					continue
				}

				var data engine.Data
				if err := json.Unmarshal([]byte(dataJson), &data); err != nil {
					return fmt.Errorf("failed to unmarshal variable %s: %v", variableName, err)
				}
				variables[variableName] = &data
			}

			cmd.Variables = variables
			cmd.WorkerId = cli.workerId

			signalEvent, err := cli.engine.SendSignal(cmd)
			if err != nil {
				return err
			}

			c.Print(signalEvent)
			return nil
		},
	}

	c.Flags().StringVar(&cmd.Name, "name", "", "Signal name")
	c.Flags().StringToStringVar(&variablesV, "variable", nil, "Variable to set or delete at process instance scope")

	c.MarkFlagRequired("name")

	return &c
}
