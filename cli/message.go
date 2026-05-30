package cli

import (
	"context"
	"strconv"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newMessageCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "message",
		Short:       "Send messages, query messages and subscriptions",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newMessageSendCmd(cli))
	c.AddCommand(newMessageQueryCmd(cli))
	c.AddCommand(newMessageSubscriptionQueryCmd(cli))

	return &c
}

func newMessageSendCmd(cli *Cli) *cobra.Command {
	var (
		expirationTime         timeValue
		expirationTimeCycle    string
		expirationTimeDuration iso8601DurationValue

		// variables
		encodingMap  map[string]string
		encryptedMap map[string]string
		valueMap     map[string]string

		cmd engine.SendMessageCmd
	)

	c := cobra.Command{
		Use:   "send",
		Short: "Send a message",
		RunE: func(c *cobra.Command, _ []string) error {
			expirationTimer := engine.Timer{
				Time:         expirationTime.Time(),
				TimeCycle:    expirationTimeCycle,
				TimeDuration: engine.ISO8601Duration(expirationTimeDuration),
			}

			if expirationTimer.Time != nil || expirationTimer.TimeCycle != "" || !expirationTimer.TimeDuration.IsZero() {
				cmd.ExpirationTimer = &expirationTimer
			}

			variables, err := mapProcessVariables(encodingMap, encryptedMap, valueMap)
			if err != nil {
				return err
			}

			cmd.Variables = variables
			cmd.WorkerId = cli.workerId

			message, err := cli.e.SendMessage(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(message)
			return nil
		},
	}

	c.Flags().StringVar(&cmd.CorrelationKey, "correlation-key", "", "Key, used to correlate a message subscription with the message")
	c.Flags().Var(&expirationTime, "expiration-time", "A point in time, when the message expires")
	c.Flags().StringVar(&expirationTimeCycle, "expiration-time-cycle", "", "CRON expression that specifies a cycle after which the message expires")
	c.Flags().Var(&expirationTimeDuration, "expiration-time-duration", "Duration until the message expires")
	c.Flags().StringVar(&cmd.Name, "name", "", "Message name")
	c.Flags().StringVar(&cmd.UniqueKey, "unique-key", "", "Optional key that uniquely identifies the message")
	c.Flags().StringToStringVar(&encodingMap, "variable-encoding", nil, "Variable to set or delete at process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&encryptedMap, "variable-encrypted", nil, "Variable to set or delete at process instance scope\nDetermines if a value is encrypted before it is stored.")
	c.Flags().StringToStringVar(&valueMap, "variable-value", nil, "Variable to set or delete at process instance scope\nData value, encoded as a string")

	c.MarkFlagRequired("correlation-key")
	c.MarkFlagRequired("name")

	return &c
}

func newMessageQueryCmd(cli *Cli) *cobra.Command {
	var (
		criteria engine.MessageCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query messages",
		RunE: func(c *cobra.Command, _ []string) error {
			q := cli.e.CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryMessages(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := newTable([]string{
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
				table.addRow([]string{
					strconv.FormatInt(result.Id, 10),
					result.Name,
					result.CorrelationKey,
					result.UniqueKey,
					strconv.FormatBool(result.IsCorrelated),
					formatTime(result.CreatedAt),
					result.CreatedBy,
					formatTimeOrNil(result.ExpiresAt),
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Int64Var(&criteria.Id, "id", 0, "Message ID")

	c.Flags().BoolVar(&criteria.ExcludeExpired, "exclude-expired", false, "Determines if expired messages are returned")
	c.Flags().StringVar(&criteria.Name, "name", "", "Message name")

	flagQueryOptions(&c, &options)

	return &c
}

func newMessageSubscriptionQueryCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		criteria engine.MessageSubscriptionCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query-subscriptions",
		Short: "Query message subscriptions",
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)

			q := cli.e.CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryMessageSubscriptions(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := newTable([]string{
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
				table.addRow([]string{
					strconv.Itoa(int(result.Id)),
					result.Partition.String(),
					strconv.Itoa(int(result.ElementInstanceId)),
					strconv.Itoa(int(result.ProcessInstanceId)),
					result.BpmnElementId,
					result.CorrelationKey,
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

	c.Flags().StringVar(&criteria.CorrelationKey, "correlation-key", "", "Message correlation key")
	c.Flags().StringVar(&criteria.Name, "name", "", "Message name")

	flagQueryOptions(&c, &options)

	return &c
}
