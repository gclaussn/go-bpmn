package incident

import (
	"context"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

const (
	allColumns     = "partition,id,elementId,elementInstanceId,jobId,processId,processInstanceId,taskId,createdAt,createdBy,resolvedAt,resolvedBy"
	defaultColumns = "partition,id,processId,processInstanceId,jobId,taskId,resolvedAt,resolvedBy"
)

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "incident",
		Short: "Resolve and query incidents",
		RunE:  common.Help,
	}

	c.AddCommand(
		newResolveCmd(),
		newQueryCmd(),
	)

	return &c
}

func newResolveCmd() *cobra.Command {
	var (
		partition common.Partition

		retryTimer common.ISO8601Duration

		cmd engine.ResolveIncidentCmd
	)

	c := cobra.Command{
		Use:         "resolve",
		Short:       "Resolve an incident",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, args []string) error {
			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.RetryTimer = engine.ISO8601Duration(retryTimer)
			cmd.WorkerId = workerId

			return e.ResolveIncident(context.Background(), cmd)
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Incident partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Incident ID")

	c.Flags().Var(&retryTimer, "retry-timer", "Duration until the retry job or task becomes due")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		partition common.Partition

		criteria engine.IncidentCriteria
		options  engine.QueryOptions
	)

	formatter := common.NewTableFormatter(allColumns, defaultColumns)

	c := cobra.Command{
		Use:         "query",
		Short:       "Query incidents",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)

			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryIncidents(context.Background(), criteria)
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
					common.FormatId(result.JobId),
					common.FormatId(result.ProcessId),
					common.FormatId(result.ProcessInstanceId),
					common.FormatId(result.TaskId),

					common.FormatTime(result.CreatedAt),
					result.CreatedBy,
					common.FormatTime(result.ResolvedAt),
					result.ResolvedBy,
				})
			}

			table.Format(c, formatter.SelectedColumns())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition filter")
	c.Flags().Int32VarP(&criteria.Id, "id", "i", 0, "Incident filter")

	c.Flags().Int32Var(&criteria.JobId, "job-id", 0, "Job filter")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance filter")
	c.Flags().Int32Var(&criteria.TaskId, "task-id", 0, "Task filter")

	common.FlagQueryOptions(&c, &options)

	formatter.Flag(&c)

	return &c
}
