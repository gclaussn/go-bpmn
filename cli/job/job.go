package job

import (
	"context"
	_ "embed"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

//go:embed job.tpl
var jobTemplate string

const (
	allColumns     = "partition,id,elementId,elementInstanceId,processId,processInstanceId,bpmnElementId,bpmnElementType,completedAt,createdAt,createdBy,dueAt,hasError,error,lockedAt,lockedBy,retryCount,state,type"
	defaultColumns = "partition,id,processId,processInstanceId,createdAt,lockedAt,completedAt,type"

	allLockColumns     = "partition,id,elementId,elementInstanceId,processId,processInstanceId,bpmnElementId,bpmnElementType,correlationKey,dueAt,lockedAt,lockedBy,retryCount,type"
	defaultLockColumns = "partition,id,processId,processInstanceId,bpmnElementId,bpmnElementType,retryCount,type"
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

		formatter common.Formatter
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

			return formatter.Format(c, job, jobTemplate)
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringVar(&cmd.Error, "error", "", "Error string")
	c.Flags().IntVar(&cmd.RetryLimit, "retry-limit", 0, "Maximum number of retries")
	c.Flags().Var(&retryTimer, "retry-timer", "Duration until a retry job becomes due")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	formatter.Flag(&c)

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

	formatter := common.NewTableFormatter(allLockColumns, defaultLockColumns)

	c := cobra.Command{
		Use:         "lock",
		Short:       "Lock jobs",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.WorkerId = workerId

			lockedJobs, err := e.LockJobs(context.Background(), cmd)
			if err != nil {
				return err
			}

			if ok, err := formatter.Format(c, lockedJobs, common.Rows(lockedJobs)); ok || err != nil {
				return err
			}

			table := common.NewTable(allLockColumns)

			for _, lockedJob := range lockedJobs {
				table.AddRow([]string{
					lockedJob.Partition.String(),
					common.FormatId(lockedJob.Id),

					common.FormatId(lockedJob.ElementId),
					common.FormatId(lockedJob.ElementInstanceId),
					common.FormatId(lockedJob.ProcessId),
					common.FormatId(lockedJob.ProcessInstanceId),

					lockedJob.BpmnElementId,
					lockedJob.BpmnElementType.String(),
					lockedJob.CorrelationKey,
					common.FormatTime(lockedJob.DueAt),
					common.FormatTime(lockedJob.LockedAt),
					lockedJob.LockedBy,
					strconv.Itoa(lockedJob.RetryCount),
					lockedJob.Type.String(),
				})
			}

			table.Format(c, formatter.SelectedColumns())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition condition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job condition - must be used in combination with a partition")

	c.Flags().Int32SliceVar(&cmd.ProcessIds, "process-id", nil, "IDs of processes to include")
	c.Flags().Int32Var(&cmd.ProcessInstanceId, "process-instance-id", 0, "Process instance condition - must be used in combination with a partition")

	c.Flags().IntVar(&cmd.Limit, "limit", 1, "Maximum number of jobs to lock")

	formatter.Flag(&c)

	return &c
}

func newUnlockCmd() *cobra.Command {
	var (
		partition common.Partition

		cmd engine.UnlockJobsCmd

		formatter common.Formatter
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

			return formatter.Format(c, count, "Count: {{ . }}\n")
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition condition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job condition - must be used in combination with a partition")

	c.Flags().StringVar(&cmd.WorkerId, "worker-id", "", "Condition that restricts the jobs, to be locked by a specific worker")

	formatter.Flag(&c)

	c.MarkFlagRequired("worker-id")

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		partition common.Partition

		criteria engine.JobCriteria
		options  engine.QueryOptions
	)

	formatter := common.NewTableFormatter(allColumns, defaultColumns)

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

			if ok, err := formatter.Format(c, results, common.Rows(results)); ok || err != nil {
				return err
			}

			table := common.NewTable(allColumns)

			for _, result := range results {
				table.AddRow([]string{
					result.Partition.String(),
					common.FormatId(result.Id),

					common.FormatId(result.ElementId),
					common.FormatId(result.ElementInstanceId),
					common.FormatId(result.ProcessId),
					common.FormatId(result.ProcessInstanceId),

					result.BpmnElementId,
					result.BpmnElementType.String(),
					common.FormatTime(result.CompletedAt),
					common.FormatTime(result.CreatedAt),
					result.CreatedBy,
					common.FormatTime(result.DueAt),
					strconv.FormatBool(result.HasError()),
					result.Error,
					common.FormatTime(result.LockedAt),
					result.LockedBy,
					strconv.Itoa(result.RetryCount),
					result.State.String(),
					result.Type.String(),
				})
			}

			table.Format(c, formatter.SelectedColumns())
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

	formatter.Flag(&c)

	return &c
}
