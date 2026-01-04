package cli

import (
	"context"
	"strconv"
	"time"

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

	c.AddCommand(newEventSendMessageCmd(cli))
	c.AddCommand(newEventSendSignalCmd(cli))
	c.AddCommand(newEventMessageQueryCmd(cli))

	return &c
}

func newEventSendMessageCmd(cli *Cli) *cobra.Command {
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
		Use:   "send-message",
		Short: "Send a message",
		RunE: func(c *cobra.Command, _ []string) error {
			expirationTimer := engine.Timer{
				Time:         time.Time(expirationTime),
				TimeCycle:    expirationTimeCycle,
				TimeDuration: engine.ISO8601Duration(expirationTimeDuration),
			}

			if !expirationTimer.Time.IsZero() || expirationTimer.TimeCycle != "" || !expirationTimer.TimeDuration.IsZero() {
				cmd.ExpirationTimer = &expirationTimer
			}

			variables, err := mapVariables(encodingMap, encryptedMap, valueMap)
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

func newEventSendSignalCmd(cli *Cli) *cobra.Command {
	var (
		// variables
		encodingMapV  map[string]string
		encryptedMapV map[string]string
		valueMapV     map[string]string

		cmd engine.SendSignalCmd
	)

	c := cobra.Command{
		Use:   "send-signal",
		Short: "Send a signal",
		RunE: func(c *cobra.Command, _ []string) error {
			variables, err := mapVariables(encodingMapV, encryptedMapV, valueMapV)
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

func newEventMessageQueryCmd(cli *Cli) *cobra.Command {
	var (
		criteria engine.MessageCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query-messages",
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
