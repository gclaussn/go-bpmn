package user_task

import (
	"context"
	_ "embed"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

//go:embed user_task.tpl
var userTaskTemplate string

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "user-task",
		Short: "Update and query user tasks",
		RunE:  common.Help,
	}

	c.AddCommand(
		newUpdateCmd(),
		newQueryCmd(),
	)

	return &c
}

func newUpdateCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		processVariables common.ProcessVariables
		tagMap           map[string]string

		cmd engine.UpdateUserTaskCmd

		formatter common.Formatter
	)

	c := cobra.Command{
		Use:         "update",
		Short:       "Update a user task",
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
			cmd.Tags = common.Tags(tagMap)
			cmd.WorkerId = workerId

			userTask, err := e.UpdateUserTask(context.Background(), cmd)
			if err != nil {
				return err
			}

			return formatter.Format(c, userTask, userTaskTemplate)
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "User task partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "User task ID")

	c.Flags().Int32VarP(&cmd.Revision, "revision", "r", 0, "User task revision that should be updated")

	c.Flags().StringToStringVar(&elementVariables.BpmnElementIdMap, "ev-bpmn-element-id", nil, "Variable to set or delete at element instance scope\nBPMN element ID to determine the variable's scope")
	c.Flags().StringToStringVar(&elementVariables.EncodingMap, "ev-encoding", nil, "Variable to set or delete at element instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&elementVariables.EncryptedMap, "ev-encrypted", nil, "Variable to set or delete at element instance scope\nDetermines if a value is encrypted before it is stored")
	c.Flags().StringToStringVar(&elementVariables.ValueMap, "ev", nil, "Variable to set or delete at element instance scope\nData value, encoded as a string")
	c.Flags().StringVar(&cmd.ErrorCode, "error-code", "", "Code of a BPMN error, used to throw an error")
	c.Flags().StringVar(&cmd.EscalationCode, "escalation-code", "", "Code of a BPMN escalation, used to escalate a user task")
	c.Flags().BoolVar(&cmd.IsCompleted, "completed", false, "Determines if a user task is completed and the execution is continued")
	c.Flags().StringToStringVar(&processVariables.EncodingMap, "pv-encoding", nil, "Variable to set or delete at process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&processVariables.EncryptedMap, "pv-encrypted", nil, "Variable to set or delete at process instance scope\nDetermines if a value is encrypted before it is stored")
	c.Flags().StringToStringVar(&processVariables.ValueMap, "pv", nil, "Variable to set or delete at process instance scope\nData value, encoded as a string")
	c.Flags().StringToStringVarP(&tagMap, "tag", "t", nil, "Tags - for a tag deletion, no value must be provided")

	formatter.Flag(&c)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("revision")

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		partition common.Partition
		tagMap    map[string]string

		criteria engine.UserTaskCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:         "query",
		Short:       "Query user tasks",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)
			criteria.Tags = common.Tags(tagMap)

			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryUserTasks(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := common.NewTable([]string{
				"PARTITION",
				"ID",
				"REVISION",
				"PROCESS ID",
				"PROCESS INSTANCE ID",
				"CREATED AT",
				"STATE",
				"UPDATED_AT",
			})

			for _, result := range results {
				table.AddRow([]string{
					result.Partition.String(),
					strconv.Itoa(int(result.Id)),
					strconv.Itoa(int(result.Revision)),
					strconv.Itoa(int(result.ProcessId)),
					strconv.Itoa(int(result.ProcessInstanceId)),
					common.FormatTime(result.CreatedAt),
					result.State.String(),
					common.FormatTime(result.UpdatedAt),
				})
			}

			c.Print(table.String())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition filter")
	c.Flags().Int32VarP(&criteria.Id, "id", "i", 0, "User task filter")

	c.Flags().Int32Var(&criteria.ElementInstanceId, "element-instance-id", 0, "Element instance filter")
	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process filter")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance filter")

	c.Flags().StringToStringVarP(&tagMap, "tag", "t", nil, "Tags, a user task must have, to be included")

	common.FlagQueryOptions(&c, &options)

	return &c
}
