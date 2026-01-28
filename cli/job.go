package cli

import (
	"context"
	"strconv"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newJobCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "job",
		Short:       "Manage and query jobs",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newJobCompleteCmd(cli))
	c.AddCommand(newJobLockCmd(cli))
	c.AddCommand(newJobUnlockCmd(cli))
	c.AddCommand(newJobQueryCmd(cli))

	return &c
}

func newJobCompleteCmd(cli *Cli) *cobra.Command {
	var (
		partition    partitionValue
		completion   engine.JobCompletion
		retryTimer   iso8601DurationValue
		timeV        timeValue
		timeCycle    string
		timeDuration iso8601DurationValue

		// element variables
		elementEncodingMap  map[string]string
		elementEncryptedMap map[string]string
		elementValueMap     map[string]string

		// process variables
		processEncodingMap  map[string]string
		processEncryptedMap map[string]string
		processValueMap     map[string]string

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:   "complete",
		Short: "Complete a job",
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := mapVariables(elementEncodingMap, elementEncryptedMap, elementValueMap)
			if err != nil {
				return err
			}

			processVariables, err := mapVariables(processEncodingMap, processEncryptedMap, processValueMap)
			if err != nil {
				return err
			}

			timer := engine.Timer{
				Time:         timeV.Time(),
				TimeCycle:    timeCycle,
				TimeDuration: engine.ISO8601Duration(timeDuration),
			}

			if timer.Time != nil || timer.TimeCycle != "" || !timer.TimeDuration.IsZero() {
				completion.Timer = &timer
			}

			cmd.Partition = engine.Partition(partition)

			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.RetryTimer = engine.ISO8601Duration(retryTimer)
			cmd.WorkerId = cli.workerId

			job, err := cli.e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Job partition")
	c.Flags().Int32Var(&cmd.Id, "id", 0, "Job ID")

	c.Flags().StringToStringVar(&elementEncodingMap, "element-variable-encoding", nil, "Variable to set or delete at element instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&elementEncryptedMap, "element-variable-encrypted", nil, "Variable to set or delete at element instance scope\nDetermines if a value is encrypted before it is stored.")
	c.Flags().StringToStringVar(&elementValueMap, "element-variable-value", nil, "Variable to set or delete at element instance scope\nData value, encoded as a string")
	c.Flags().StringVar(&cmd.Error, "error", "", "Optional error string, used to fail a job due to a technical problem")
	c.Flags().StringToStringVar(&processEncodingMap, "process-variable-encoding", nil, "Variable to set or delete at process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&processEncryptedMap, "process-variable-encrypted", nil, "Variable to set or delete at process instance scope\nDetermines if a value is encrypted before it is stored.")
	c.Flags().StringToStringVar(&processValueMap, "process-variable-value", nil, "Variable to set or delete at process instance scope\nData value, encoded as a string")
	c.Flags().IntVar(&cmd.RetryLimit, "retry-limit", 0, "Maximum number of retries")
	c.Flags().Var(&retryTimer, "retry-timer", "Duration until the retry job becomes due")

	c.Flags().StringVar(&completion.ErrorCode, "error-code", "", "Code of a BPMN error, used to specify or to trigger a BPMN error")
	c.Flags().StringVar(&completion.EscalationCode, "escalation-code", "", "Code of a BPMN escalation, used to specify or to trigger a BPMN escalation")
	c.Flags().StringVar(&completion.ExclusiveGatewayDecision, "exclusive-gateway-decision", "", "Evaluated BPMN element ID to continue with after the exclusive gateway")
	c.Flags().StringSliceVar(&completion.InclusiveGatewayDecision, "inclusive-gateway-decision", nil, "Evaluated BPMN element ID to continue with after the inclusive gateway")
	c.Flags().StringVar(&completion.MessageCorrelationKey, "message-correlation-key", "", "Key, used to correlate a message subscription with a message")
	c.Flags().StringVar(&completion.MessageName, "message-name", "", "Name of the message to subscribe to")
	c.Flags().StringVar(&completion.SignalName, "signal-name", "", "Name of the signal to subscribe to")
	c.Flags().Var(&timeV, "time", "A point in time, when the timer event is triggered")
	c.Flags().StringVar(&timeCycle, "time-cycle", "", "CRON expression that specifies a cyclic trigger")
	c.Flags().Var(&timeDuration, "time-duration", "Duration until the timer event is triggered")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newJobLockCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		cmd engine.LockJobsCmd
	)

	c := cobra.Command{
		Use:   "lock",
		Short: "Lock jobs",
		RunE: func(c *cobra.Command, _ []string) error {
			cmd.Partition = engine.Partition(partition)
			cmd.WorkerId = cli.workerId

			jobs, err := cli.e.LockJobs(context.Background(), cmd)
			if err != nil {
				return err
			}

			table := newTable([]string{
				"PARTITION",
				"ID",
				"PROCESS ID",
				"PROCESS INSTANCE ID",
				"CREATED AT",
				"LOCKED_AT",
				"TYPE",
			})

			for i := range jobs {
				job := jobs[i]

				table.addRow([]string{
					job.Partition.String(),
					strconv.Itoa(int(job.Id)),
					strconv.Itoa(int(job.ProcessId)),
					strconv.Itoa(int(job.ProcessInstanceId)),
					formatTime(job.CreatedAt),
					formatTimeOrNil(job.LockedAt),
					job.Type.String(),
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Job partition")
	c.Flags().Int32Var(&cmd.Id, "id", 0, "Job ID")

	c.Flags().Int32SliceVar(&cmd.ProcessIds, "process-id", nil, "IDs of processes to include")
	c.Flags().Int32Var(&cmd.ProcessInstanceId, "process-instance-id", 0, "Process instance ID")

	c.Flags().IntVar(&cmd.Limit, "limit", 1, "Maximum number of jobs to lock")

	return &c
}

func newJobUnlockCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		cmd engine.UnlockJobsCmd
	)

	c := cobra.Command{
		Use:   "unlock",
		Short: "Unlock jobs",
		RunE: func(c *cobra.Command, _ []string) error {
			cmd.Partition = engine.Partition(partition)

			count, err := cli.e.UnlockJobs(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Printf("Number of unlocked jobs: %d\n", count)
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Job partition")
	c.Flags().Int32Var(&cmd.Id, "id", 0, "Job ID")

	c.Flags().StringVar(&cmd.WorkerId, "worker-id", "", "Condition that restricts the jobs, to be locked by a specific worker")

	c.MarkFlagRequired("worker-id")

	return &c
}

func newJobQueryCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		criteria engine.JobCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query jobs",
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)

			q := cli.e.CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryJobs(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := newTable([]string{
				"PARTITION",
				"ID",
				"PROCESS ID",
				"PROCESS INSTANCE ID",
				"CREATED AT",
				"LOCKED_AT",
				"COMPLETED AT",
				"TYPE",
			})

			for _, result := range results {
				table.addRow([]string{
					result.Partition.String(),
					strconv.Itoa(int(result.Id)),
					strconv.Itoa(int(result.ProcessId)),
					strconv.Itoa(int(result.ProcessInstanceId)),
					formatTime(result.CreatedAt),
					formatTimeOrNil(result.LockedAt),
					formatTimeOrNil(result.CompletedAt),
					result.Type.String(),
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Job partition")
	c.Flags().Int32Var(&criteria.Id, "id", 0, "Job ID")

	c.Flags().Int32Var(&criteria.ElementInstanceId, "element-instance-id", 0, "Element instance ID")
	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance ID")

	flagQueryOptions(&c, &options)

	return &c
}
