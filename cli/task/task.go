package task

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

const (
	allColumns            = "partition,id,elementId,elementInstanceId,processId,processInstanceId,bpmnElementId,completedAt,createdAt,createdBy,dueAt,hasError,error,lockedAt,lockedBy,retryCount,serializedTask,state,type"
	defaultColumns        = "partition,id,processId,processInstanceId,createdAt,lockedAt,completedAt,type"
	defaultExecuteColumns = "partition,id,processId,processInstanceId,elementInstanceId,type,hasError"
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

	formatter := common.NewTableFormatter(allColumns, defaultExecuteColumns)

	c := cobra.Command{
		Use:         "execute",
		Short:       "Execute tasks",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			cmd.Partition = engine.Partition(partition)
			cmd.Type = engine.TaskType(taskType)

			completedTasks, failedTasks, err := common.GetEngine(c).ExecuteTasks(context.Background(), cmd)
			if err != nil {
				return err
			}

			tasks := slices.Concat(completedTasks, failedTasks)

			if ok, err := formatter.Format(c, tasks, common.Rows(tasks)); ok || err != nil {
				return err
			}

			table := common.NewTable(allColumns)
			addRows(table, tasks)
			table.Format(c, formatter.SelectedColumns())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition condition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Task condition - must be used in combination with a partition")

	c.Flags().Int32Var(&cmd.ProcessId, "process-id", 0, "Process condition")
	c.Flags().Int32Var(&cmd.ProcessInstanceId, "process-instance-id", 0, "Process instance condition - must be used in combination with a partition")
	c.Flags().Var(&taskType, "type", "Task type condition")

	c.Flags().IntVar(&cmd.Limit, "limit", 1, "Maximum number of tasks to lock and execute")

	formatter.Flag(&c)

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

	formatter := common.NewTableFormatter(allColumns, defaultColumns)

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

			if ok, err := formatter.Format(c, results, common.Rows(results)); ok || err != nil {
				return err
			}

			table := common.NewTable(allColumns)
			addRows(table, results)
			table.Format(c, formatter.SelectedColumns())
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

	formatter.Flag(&c)

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

func addRows(table *common.Table, tasks []engine.Task) {
	for _, task := range tasks {
		table.AddRow([]string{
			task.Partition.String(),
			common.FormatId(task.Id),

			common.FormatId(task.ElementId),
			common.FormatId(task.ElementInstanceId),
			common.FormatId(task.ProcessId),
			common.FormatId(task.ProcessInstanceId),

			task.BpmnElementId,
			common.FormatTime(task.CompletedAt),
			common.FormatTime(task.CreatedAt),
			task.CreatedBy,
			common.FormatTime(task.DueAt),
			strconv.FormatBool(task.HasError()),
			task.Error,
			common.FormatTime(task.LockedAt),
			task.LockedBy,
			strconv.Itoa(task.RetryCount),
			task.SerializedTask,
			task.State.String(),
			task.Type.String(),
		})
	}
}
