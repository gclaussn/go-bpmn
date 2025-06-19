package cli

import (
	"strconv"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newIncidentCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "incident",
		Short:       "Resolve and query incidents",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newIncidentResolveCmd(cli))
	c.AddCommand(newIncidentQueryCmd(cli))

	return &c
}

func newIncidentResolveCmd(cli *Cli) *cobra.Command {
	var (
		partition  partitionValue
		retryTimer iso8601DurationValue

		cmd engine.ResolveIncidentCmd
	)

	c := cobra.Command{
		Use:   "resolve",
		Short: "Resolve an incident",
		RunE: func(c *cobra.Command, args []string) error {
			cmd.Partition = engine.Partition(partition)
			cmd.RetryTimer = engine.ISO8601Duration(retryTimer)
			cmd.WorkerId = cli.workerId

			return cli.engine.ResolveIncident(cmd)
		},
	}

	c.Flags().Var(&partition, "partition", "Incident partition")
	c.Flags().Int32Var(&cmd.Id, "id", 0, "Incident ID")

	c.Flags().IntVar(&cmd.RetryCount, "retry-count", 0, "Number of retries the newly created job or task has left")
	c.Flags().Var(&retryTimer, "retry-timer", "Duration until the retry job or task becomes due")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newIncidentQueryCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		criteria engine.IncidentCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query incidents",
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
				"JOB ID",
				"TASK ID",
				"RESOLVED AT",
				"RESOLVED BY",
			})

			for i := range results {
				incident := results[i].(engine.Incident)

				table.addRow([]string{
					incident.Partition.String(),
					strconv.Itoa(int(incident.Id)),
					strconv.Itoa(int(incident.ProcessId)),
					strconv.Itoa(int(incident.ProcessInstanceId)),
					strconv.Itoa(int(incident.JobId)),
					strconv.Itoa(int(incident.TaskId)),
					formatTimeOrNil(incident.ResolvedAt),
					incident.ResolvedBy,
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Incident partition")
	c.Flags().Int32Var(&criteria.Id, "id", 0, "Incident ID")

	c.Flags().Int32Var(&criteria.JobId, "job-id", 0, "Job ID")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance ID")
	c.Flags().Int32Var(&criteria.TaskId, "task-id", 0, "Task ID")

	flagQueryOptions(&c, &options)

	return &c
}
