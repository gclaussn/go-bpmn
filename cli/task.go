package cli

import (
	"strconv"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newTaskCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "task",
		Short:       "Manage and query tasks",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newTaskExecuteCmd(cli))
	c.AddCommand(newTaskUnlockCmd(cli))
	c.AddCommand(newTaskQueryCmd(cli))

	return &c
}

func newTaskExecuteCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue
		taskType  taskTypeValue

		cmd engine.ExecuteTasksCmd
	)

	c := cobra.Command{
		Use:   "execute",
		Short: "Execute tasks",
		RunE: func(c *cobra.Command, _ []string) error {
			cmd.Partition = engine.Partition(partition)
			cmd.Type = engine.TaskType(taskType)

			completedTask, failedTasks, err := cli.engine.ExecuteTasks(cmd)
			if err != nil {
				return err
			}

			completedTable := newTable([]string{
				"PARTITION",
				"ID",
				"PROCESS ID",
				"PROCESS INSTANCE ID",
				"ELEMENT ID",
				"ELEMENT INSTANCE ID",
				"HAS ERROR",
				"TYPE",
			})

			for i := range completedTask {
				task := completedTask[i]

				completedTable.addRow([]string{
					task.Partition.String(),
					strconv.Itoa(int(task.Id)),
					strconv.Itoa(int(task.ProcessId)),
					strconv.Itoa(int(task.ProcessInstanceId)),
					strconv.Itoa(int(task.ElementId)),
					strconv.Itoa(int(task.ElementInstanceId)),
					strconv.FormatBool(task.HasError()),
					task.Type.String(),
				})
			}

			if len(failedTasks) != 0 {
				failedTable := newTable([]string{
					"PARTITION",
					"ID",
					"PROCESS ID",
					"PROCESS INSTANCE ID",
					"ELEMENT ID",
					"ELEMENT INSTANCE ID",
					"TYPE",
				})

				for i := range failedTasks {
					task := failedTasks[i]

					failedTable.addRow([]string{
						task.Partition.String(),
						strconv.Itoa(int(task.Id)),
						strconv.Itoa(int(task.ProcessId)),
						strconv.Itoa(int(task.ProcessInstanceId)),
						strconv.Itoa(int(task.ElementId)),
						strconv.Itoa(int(task.ElementInstanceId)),
						task.Type.String(),
					})
				}

				c.Println("Failed")
				c.Print(failedTable.format())
			}

			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Task partition")
	c.Flags().Int32Var(&cmd.Id, "id", 0, "Task ID")

	c.Flags().Int32Var(&cmd.ElementInstanceId, "element-instance-id", 0, "Element instance ID")
	c.Flags().Int32Var(&cmd.ProcessInstanceId, "process-instance-id", 0, "Process instance ID")
	c.Flags().Var(&taskType, "type", "Task type")

	c.Flags().IntVar(&cmd.Limit, "limit", 1, "Maximum number of tasks to lock and execute")

	return &c
}

func newTaskUnlockCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		cmd engine.UnlockTasksCmd
	)

	c := cobra.Command{
		Use:   "unlock",
		Short: "Unlock tasks",
		RunE: func(c *cobra.Command, _ []string) error {
			cmd.Partition = engine.Partition(partition)

			count, err := cli.engine.UnlockTasks(cmd)
			if err != nil {
				return err
			}

			c.Printf("Number of unlocked tasks: %d\n", count)
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Task partition")
	c.Flags().Int32Var(&cmd.Id, "id", 0, "Task ID")

	c.Flags().StringVar(&cmd.EngineId, "engine-id", "", "Condition that restricts the tasks, to be locked by a specific engine")

	c.MarkFlagRequired("engine-id")

	return &c
}

func newTaskQueryCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue
		taskType  taskTypeValue

		criteria engine.TaskCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query tasks",
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)
			criteria.Type = engine.TaskType(taskType)

			results, err := cli.engine.QueryWithOptions(criteria, options)
			if err != nil {
				return err
			}

			table := newTable([]string{
				"PARTITION",
				"ID",
				"PROCESS INSTANCE ID",
				"CREATED AT",
				"LOCKED_AT",
				"COMPLETED AT",
				"TYPE",
			})

			for i := range results {
				task := results[i].(engine.Task)

				table.addRow([]string{
					task.Partition.String(),
					strconv.Itoa(int(task.Id)),
					strconv.Itoa(int(task.ProcessInstanceId)),
					formatTime(task.CreatedAt),
					formatTimeOrNil(task.LockedAt),
					formatTimeOrNil(task.CompletedAt),
					task.Type.String(),
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Task partition")
	c.Flags().Int32Var(&criteria.Id, "id", 0, "Task ID")

	c.Flags().Int32Var(&criteria.ElementInstanceId, "element-instance-id", 0, "Element instance ID")
	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance ID")

	c.Flags().Var(&taskType, "type", "Task type")

	flagQueryOptions(&c, &options)

	return &c
}
