package cli

import (
	"encoding/json"
	"fmt"
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
		partition         partitionValue
		completion        engine.JobCompletion
		elementVariablesV map[string]string
		processVariablesV map[string]string
		retryTimer        iso8601DurationValue

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:   "complete",
		Short: "Complete a job",
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables := make(map[string]*engine.Data)
			for variableName, dataJson := range elementVariablesV {
				if dataJson == "" || dataJson == "null" {
					elementVariables[variableName] = nil
					continue
				}

				var data engine.Data
				if err := json.Unmarshal([]byte(dataJson), &data); err != nil {
					return fmt.Errorf("failed to unmarshal element variable %s: %v", variableName, err)
				}
				elementVariables[variableName] = &data
			}

			processVariables := make(map[string]*engine.Data)
			for variableName, dataJson := range processVariablesV {
				if dataJson == "" || dataJson == "null" {
					processVariables[variableName] = nil
					continue
				}

				var data engine.Data
				if err := json.Unmarshal([]byte(dataJson), &data); err != nil {
					return fmt.Errorf("failed to unmarshal process variable %s: %v", variableName, err)
				}
				processVariables[variableName] = &data
			}

			cmd.Partition = engine.Partition(partition)

			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.RetryTimer = engine.ISO8601Duration(retryTimer)
			cmd.WorkerId = cli.workerId

			job, err := cli.engine.CompleteJob(cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Job partition")
	c.Flags().Int32Var(&cmd.Id, "id", 0, "Job ID")

	c.Flags().StringToStringVar(&elementVariablesV, "element-variable", nil, "Variable to set or delete at element instance scope")
	c.Flags().StringVar(&cmd.Error, "error", "", "Optional error string, used to fail a job due to a technical problem")
	c.Flags().StringToStringVar(&processVariablesV, "process-variable", nil, "Variable to set or delete at process instance scope")
	c.Flags().IntVar(&cmd.RetryCount, "retry-count", 0, "Number of retries left")
	c.Flags().Var(&retryTimer, "retry-timer", "Duration until the retry job becomes due")

	c.Flags().StringVar(&completion.ExclusiveGatewayDecision, "exclusive-gateway-decision", "", "Evaluated BPMN element ID to continue with after the exclusive gateway")
	c.Flags().StringSliceVar(&completion.InclusiveGatewayDecision, "inclusive-gateway-decision", nil, "Evaluated BPMN element ID to continue with after the inclusive gateway")

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

			jobs, err := cli.engine.LockJobs(cmd)
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

			for i := 0; i < len(jobs); i++ {
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

	c.Flags().StringSliceVar(&cmd.BpmnElementIds, "bpmn-element-id", nil, "ID of a BPMN element to include")
	c.Flags().Int32Var(&cmd.ElementInstanceId, "element-instance-id", 0, "Element instance ID")
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

			count, err := cli.engine.UnlockJobs(cmd)
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

			results, err := cli.engine.QueryWithOptions(criteria, options)
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

			for i := 0; i < len(results); i++ {
				job := results[i].(engine.Job)

				table.addRow([]string{
					job.Partition.String(),
					strconv.Itoa(int(job.Id)),
					strconv.Itoa(int(job.ProcessId)),
					strconv.Itoa(int(job.ProcessInstanceId)),
					formatTime(job.CreatedAt),
					formatTimeOrNil(job.LockedAt),
					formatTimeOrNil(job.CompletedAt),
					job.Type.String(),
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
