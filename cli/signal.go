package cli

import (
	"context"
	"strconv"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newSignalCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "signal",
		Short:       "Send signals and query signal subscriptions",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newSignalSendCmd(cli))
	c.AddCommand(newSignalSubscriptionQueryCmd(cli))

	return &c
}

func newSignalSendCmd(cli *Cli) *cobra.Command {
	var (
		// variables
		encodingMapV  map[string]string
		encryptedMapV map[string]string
		valueMapV     map[string]string

		cmd engine.SendSignalCmd
	)

	c := cobra.Command{
		Use:   "send",
		Short: "Send a signal",
		RunE: func(c *cobra.Command, _ []string) error {
			variables, err := mapProcessVariables(encodingMapV, encryptedMapV, valueMapV)
			if err != nil {
				return err
			}

			cmd.Variables = variables
			cmd.WorkerId = cli.workerId

			signal, err := cli.e.SendSignal(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(signal)
			return nil
		},
	}

	c.Flags().StringVar(&cmd.Name, "name", "", "Signal name")
	c.Flags().StringToStringVar(&encodingMapV, "variable-encoding", nil, "Variable to set or delete at process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&encryptedMapV, "variable-encrypted", nil, "Variable to set or delete at process instance scope\nDetermines if a value is encrypted before it is stored.")
	c.Flags().StringToStringVar(&valueMapV, "variable-value", nil, "Variable to set or delete at process instance scope\nData value, encoded as a string")

	c.MarkFlagRequired("name")

	return &c
}

func newSignalSubscriptionQueryCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		criteria engine.SignalSubscriptionCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query-subscriptions",
		Short: "Query signal subscriptions",
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)

			q := cli.e.CreateQuery()
			q.SetOptions(options)

			results, err := q.QuerySignalSubscriptions(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := newTable([]string{
				"ID",
				"PARTITION",
				"ELEMENT_INSTANCE_ID",
				"PROCESS INSTANCE ID",
				"BPMN ELEMENT ID",
				"CREATED AT",
				"CREATED BY",
				"NAME",
			})

			for _, result := range results {
				table.addRow([]string{
					strconv.Itoa(int(result.Id)),
					result.Partition.String(),
					strconv.Itoa(int(result.ElementInstanceId)),
					strconv.Itoa(int(result.ProcessInstanceId)),
					result.BpmnElementId,
					formatTime(result.CreatedAt),
					result.CreatedBy,
					result.Name,
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Partition filter")

	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance filter")

	c.Flags().StringVar(&criteria.Name, "name", "", "Signal name")

	flagQueryOptions(&c, &options)

	return &c
}
