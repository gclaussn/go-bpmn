package signal

import (
	"context"
	_ "embed"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

//go:embed signal.tpl
var signalTemplate string

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "signal",
		Short: "Send signals and query signal subscriptions",
		RunE:  common.Help,
	}

	c.AddCommand(
		newSendCmd(),
		newQuerySubscriptionsCmd(),
	)

	return &c
}

func newSendCmd() *cobra.Command {
	var (
		variables common.ProcessVariables

		cmd engine.SendSignalCmd

		formatter common.Formatter
	)

	c := cobra.Command{
		Use:         "send",
		Short:       "Send a signal",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			variables, err := variables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Variables = variables
			cmd.WorkerId = workerId

			signal, err := e.SendSignal(context.Background(), cmd)
			if err != nil {
				return err
			}

			return formatter.Format(c, signal, signalTemplate)
		},
	}

	c.Flags().StringVar(&cmd.Name, "name", "", "Signal name")
	c.Flags().StringToStringVar(&variables.EncodingMap, "variable-encoding", nil, "Variable to set or delete at process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&variables.EncryptedMap, "variable-encrypted", nil, "Variable to set or delete at process instance scope\nDetermines if a value is encrypted before it is stored")
	c.Flags().StringToStringVar(&variables.ValueMap, "variable", nil, "Variable to set or delete at process instance scope\nData value, encoded as a string")

	formatter.Flag(&c)

	c.MarkFlagRequired("name")

	return &c
}

func newQuerySubscriptionsCmd() *cobra.Command {
	var (
		partition common.Partition

		criteria engine.SignalSubscriptionCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:         "query-subscriptions",
		Short:       "Query signal subscriptions",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)

			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QuerySignalSubscriptions(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := common.NewTable([]string{
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
				table.AddRow([]string{
					strconv.Itoa(int(result.Id)),
					result.Partition.String(),
					strconv.Itoa(int(result.ElementInstanceId)),
					strconv.Itoa(int(result.ProcessInstanceId)),
					result.BpmnElementId,
					common.FormatTime(result.CreatedAt),
					result.CreatedBy,
					result.Name,
				})
			}

			c.Print(table.String())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition filter")

	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance filter")

	c.Flags().StringVar(&criteria.Name, "name", "", "Signal name")

	common.FlagQueryOptions(&c, &options)

	return &c
}
