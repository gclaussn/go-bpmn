package job

import (
	"context"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "job",
		Short: "Manage and query jobs",
		RunE:  common.Help,
	}

	c.AddCommand(
		newCompleteCmd(),
		newFailCmd(),
		newLockCmd(),
		newUnlockCmd(),
		newQueryCmd(),
	)

	return &c
}

func newCompleteCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "complete",
		Short: "Complete a job",
		RunE:  common.Help,
	}

	c.AddCommand(
		newCallProcessCmd(),
		newEvaluateExclusiveGatewayCmd(),
		newEvaluateInclusiveGatewayCmd(),
		newExecuteCmd(),
		newPassVariablesCmd(),
		newSetErrorCodeCmd(),
		newSetEscalationCodeCmd(),
		newSetMessageCorrelationKeyCmd(),
		newSetSignalNameCmd(),
		newSetTimerCmd(),
		newSubscribeMessageCmd(),
		newSubscribeSignalCmd(),
	)

	return &c
}

func newFailCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		processVariables common.ProcessVariables
		retryTimer       common.ISO8601Duration

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "fail",
		Short:       "Complete a job with an error due to a technical problem",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.RetryTimer = engine.ISO8601Duration(retryTimer)
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringVar(&cmd.Error, "error", "", "Error string")
	c.Flags().IntVar(&cmd.RetryLimit, "retry-limit", 0, "Maximum number of retries")
	c.Flags().Var(&retryTimer, "retry-timer", "Duration until a retry job becomes due")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("error")

	return &c
}

func newLockCmd() *cobra.Command {
	var (
		partition common.Partition

		cmd engine.LockJobsCmd
	)

	c := cobra.Command{
		Use:         "lock",
		Short:       "Lock jobs",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.WorkerId = workerId

			jobs, err := e.LockJobs(context.Background(), cmd)
			if err != nil {
				return err
			}

			table := common.NewTable([]string{
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

				table.AddRow([]string{
					job.Partition.String(),
					strconv.Itoa(int(job.Id)),
					strconv.Itoa(int(job.ProcessId)),
					strconv.Itoa(int(job.ProcessInstanceId)),
					common.FormatTime(job.CreatedAt),
					common.FormatTimeOrNil(job.LockedAt),
					job.Type.String(),
				})
			}

			c.Print(table.String())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition condition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job condition - must be used in combination with a partition")

	c.Flags().Int32SliceVar(&cmd.ProcessIds, "process-id", nil, "IDs of processes to include")
	c.Flags().Int32Var(&cmd.ProcessInstanceId, "process-instance-id", 0, "Process instance condition - must be used in combination with a partition")

	c.Flags().IntVar(&cmd.Limit, "limit", 1, "Maximum number of jobs to lock")

	return &c
}

func newUnlockCmd() *cobra.Command {
	var (
		partition common.Partition

		cmd engine.UnlockJobsCmd
	)

	c := cobra.Command{
		Use:         "unlock",
		Short:       "Unlock jobs",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			e := common.GetEngine(c)

			cmd.Partition = engine.Partition(partition)

			count, err := e.UnlockJobs(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Printf("Number of unlocked jobs: %d\n", count)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition condition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job condition - must be used in combination with a partition")

	c.Flags().StringVar(&cmd.WorkerId, "worker-id", "", "Condition that restricts the jobs, to be locked by a specific worker")

	c.MarkFlagRequired("worker-id")

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		partition common.Partition

		criteria engine.JobCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:         "query",
		Short:       "Query jobs",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)

			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryJobs(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := common.NewTable([]string{
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
				table.AddRow([]string{
					result.Partition.String(),
					strconv.Itoa(int(result.Id)),
					strconv.Itoa(int(result.ProcessId)),
					strconv.Itoa(int(result.ProcessInstanceId)),
					common.FormatTime(result.CreatedAt),
					common.FormatTimeOrNil(result.LockedAt),
					common.FormatTimeOrNil(result.CompletedAt),
					result.Type.String(),
				})
			}

			c.Print(table.String())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition filter")
	c.Flags().Int32VarP(&criteria.Id, "id", "i", 0, "Job filter")

	c.Flags().Int32Var(&criteria.ElementId, "element-id", 0, "Element filter")
	c.Flags().Int32Var(&criteria.ElementInstanceId, "element-instance-id", 0, "Element instance filter")
	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process filter")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance filter")

	common.FlagQueryOptions(&c, &options)

	return &c
}
