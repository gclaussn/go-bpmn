package task

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "task",
		Short: "Manage and query tasks",
		RunE:  common.Help,
	}

	c.AddCommand(
		newExecuteCmd(),
		newUnlockCmd(),
		newQueryCmd(),
	)

	return &c
}

func newExecuteCmd() *cobra.Command {
	var (
		partition common.Partition

		taskType taskType

		cmd engine.ExecuteTasksCmd
	)

	c := cobra.Command{
		Use:         "execute",
		Short:       "Execute tasks",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			cmd.Partition = engine.Partition(partition)
			cmd.Type = engine.TaskType(taskType)

			completedTask, failedTasks, err := common.GetEngine(c).ExecuteTasks(context.Background(), cmd)
			if err != nil {
				return err
			}

			completedTable := common.NewTable([]string{
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

				completedTable.AddRow([]string{
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

			c.Println("Completed")
			c.Print(completedTable.String())

			if len(failedTasks) != 0 {
				failedTable := common.NewTable([]string{
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

					failedTable.AddRow([]string{
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
				c.Print(failedTable.String())
			}

			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition condition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Task condition - must be used in combination with a partition")

	c.Flags().Int32Var(&cmd.ProcessId, "process-id", 0, "Process condition")
	c.Flags().Int32Var(&cmd.ProcessInstanceId, "process-instance-id", 0, "Process instance condition - must be used in combination with a partition")
	c.Flags().Var(&taskType, "type", "Task type condition")

	c.Flags().IntVar(&cmd.Limit, "limit", 1, "Maximum number of tasks to lock and execute")

	return &c
}

func newUnlockCmd() *cobra.Command {
	var (
		partition common.Partition

		cmd engine.UnlockTasksCmd

		formatter common.Formatter
	)

	c := cobra.Command{
		Use:         "unlock",
		Short:       "Unlock tasks",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			cmd.Partition = engine.Partition(partition)

			count, err := common.GetEngine(c).UnlockTasks(context.Background(), cmd)
			if err != nil {
				return err
			}

			return formatter.Format(c, count, "Count: {{ . }}\n")
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition condition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Task condition - must be used in combination with a partition")

	c.Flags().StringVar(&cmd.EngineId, "engine-id", "", "Condition that restricts the tasks, to be locked by a specific engine")

	formatter.Flag(&c)

	c.MarkFlagRequired("engine-id")

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		partition common.Partition

		taskType taskType

		criteria engine.TaskCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:         "query",
		Short:       "Query tasks",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)
			criteria.Type = engine.TaskType(taskType)

			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryTasks(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := common.NewTable([]string{
				"PARTITION",
				"ID",
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
	c.Flags().Int32VarP(&criteria.Id, "id", "i", 0, "Task filter")

	c.Flags().Int32Var(&criteria.ElementId, "element-id", 0, "Element filter")
	c.Flags().Int32Var(&criteria.ElementInstanceId, "element-instance-id", 0, "Element instance filter")
	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process filter")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance filter")

	c.Flags().Var(&taskType, "type", "Task type")

	common.FlagQueryOptions(&c, &options)

	return &c
}

// taskType is a flag value for an engine task type.
type taskType engine.TaskType

func (v *taskType) Set(s string) error {
	value := engine.MapTaskType(s)
	if value == 0 {
		return fmt.Errorf("invalid task type %s", s)
	}

	*v = taskType(value)
	return nil
}

func (v taskType) String() string {
	return engine.TaskType(v).String()
}

func (v taskType) Type() string {
	return "taskType"
}
