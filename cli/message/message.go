package message

import (
	"context"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "message",
		Short: "Send messages, query messages and subscriptions",
		RunE:  common.Help,
	}

	c.AddCommand(
		newSendCmd(),
		newQueryCmd(),
		newQuerySubscriptionsCmd(),
	)

	return &c
}

func newSendCmd() *cobra.Command {
	var (
		expirationTimer common.Timer
		variables       common.ProcessVariables

		cmd engine.SendMessageCmd
	)

	c := cobra.Command{
		Use:         "send",
		Short:       "Send a message",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			variables, err := variables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.ExpirationTimer = expirationTimer.Timer()
			cmd.Variables = variables
			cmd.WorkerId = workerId

			message, err := e.SendMessage(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(message)
			return nil
		},
	}

	c.Flags().StringVar(&cmd.CorrelationKey, "correlation-key", "", "Key, used to correlate a message subscription with the message")
	c.Flags().Var(&expirationTimer.Time, "expiration-time", "A point in time, when the message expires")
	c.Flags().StringVar(&expirationTimer.TimeCycle, "expiration-time-cycle", "", "CRON expression that specifies a cycle after which the message expires")
	c.Flags().Var(&expirationTimer.TimeDuration, "expiration-time-duration", "Duration until the message expires")
	c.Flags().StringVar(&cmd.Name, "name", "", "Message name")
	c.Flags().StringVar(&cmd.UniqueKey, "unique-key", "", "Optional key that uniquely identifies the message")
	c.Flags().StringToStringVar(&variables.EncodingMap, "variable-encoding", nil, "Variable to set or delete at process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&variables.EncryptedMap, "variable-encrypted", nil, "Variable to set or delete at process instance scope\nDetermines if a value is encrypted before it is stored")
	c.Flags().StringToStringVar(&variables.ValueMap, "variable", nil, "Variable to set or delete at process instance scope\nData value, encoded as a string")

	c.MarkFlagRequired("correlation-key")
	c.MarkFlagRequired("name")

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		criteria engine.MessageCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:         "query",
		Short:       "Query messages",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryMessages(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := common.NewTable([]string{
				"ID",
				"NAME",
				"CORRELATION KEY",
				"UNIQUE KEY",
				"CORRELATED",
				"CREATED AT",
				"CREATED BY",
				"EXPIRES AT",
			})

			for _, result := range results {
				table.AddRow([]string{
					strconv.FormatInt(result.Id, 10),
					result.Name,
					result.CorrelationKey,
					result.UniqueKey,
					strconv.FormatBool(result.IsCorrelated),
					common.FormatTime(result.CreatedAt),
					result.CreatedBy,
					common.FormatTimeOrNil(result.ExpiresAt),
				})
			}

			c.Print(table.String())
			return nil
		},
	}

	c.Flags().Int64VarP(&criteria.Id, "id", "i", 0, "Message filter")

	c.Flags().BoolVar(&criteria.ExcludeExpired, "exclude-expired", false, "Determines if expired messages are returned")
	c.Flags().StringVar(&criteria.Name, "name", "", "Message name filter")

	common.FlagQueryOptions(&c, &options)

	return &c
}

func newQuerySubscriptionsCmd() *cobra.Command {
	var (
		partition common.Partition

		criteria engine.MessageSubscriptionCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:         "query-subscriptions",
		Short:       "Query message subscriptions",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)

			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryMessageSubscriptions(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := common.NewTable([]string{
				"ID",
				"PARTITION",
				"ELEMENT_INSTANCE_ID",
				"PROCESS INSTANCE ID",
				"BPMN ELEMENT ID",
				"CORRELATION KEY",
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
					result.CorrelationKey,
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

	c.Flags().StringVar(&criteria.CorrelationKey, "correlation-key", "", "Message correlation key")
	c.Flags().StringVar(&criteria.Name, "name", "", "Message name")

	common.FlagQueryOptions(&c, &options)

	return &c
}
