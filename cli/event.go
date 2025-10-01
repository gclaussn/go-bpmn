package cli

import (
	"context"
	"encoding/json"
	"fmt"
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
		expirationTimeV         timeValue
		expirationTimeCycleV    string
		expirationTimeDurationV iso8601DurationValue
		variablesV              map[string]string

		cmd engine.SendMessageCmd
	)

	c := cobra.Command{
		Use:   "send-message",
		Short: "Send a message",
		RunE: func(c *cobra.Command, _ []string) error {
			expirationTimer := engine.Timer{
				Time:         time.Time(expirationTimeV),
				TimeCycle:    expirationTimeCycleV,
				TimeDuration: engine.ISO8601Duration(expirationTimeDurationV),
			}

			if !expirationTimer.Time.IsZero() || expirationTimer.TimeCycle != "" || !expirationTimer.TimeDuration.IsZero() {
				cmd.ExpirationTimer = &expirationTimer
			}

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

			message, err := cli.e.SendMessage(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(message)
			return nil
		},
	}

	c.Flags().StringVar(&cmd.CorrelationKey, "correlation-key", "", "Key, used to correlate a message subscription with the message")
	c.Flags().Var(&expirationTimeV, "expiration-time", "A point in time, when the message expires")
	c.Flags().StringVar(&expirationTimeCycleV, "expiration-time-cycle", "", "CRON expression that specifies a cyclic after which the message expires")
	c.Flags().Var(&expirationTimeDurationV, "expiration-time-duration", "Duration until the message expires")
	c.Flags().StringVar(&cmd.Name, "name", "", "Message name")
	c.Flags().StringVar(&cmd.UniqueKey, "unique-key", "", "Optional key that uniquely identifies the message")
	c.Flags().StringToStringVar(&variablesV, "variable", nil, "Variable to set or delete at process instance scope")

	c.MarkFlagRequired("correlation-key")
	c.MarkFlagRequired("name")

	return &c
}

func newEventSendSignalCmd(cli *Cli) *cobra.Command {
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

			signal, err := cli.e.SendSignal(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(signal)
			return nil
		},
	}

	c.Flags().StringVar(&cmd.Name, "name", "", "Signal name")
	c.Flags().StringToStringVar(&variablesV, "variable", nil, "Variable to set or delete at process instance scope")

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
