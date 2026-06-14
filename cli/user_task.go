package cli

import (
	"context"
	"strconv"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newUserTaskCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "user-task",
		Short:       "Update and query user tasks",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newUserTaskUpdateCmd(cli))
	c.AddCommand(newUserTaskQueryCmd(cli))

	return &c
}

func newUserTaskUpdateCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue
		tagsV     map[string]string

		// element variables
		elementEncodingMap  map[string]string
		elementEncryptedMap map[string]string
		elementIdMap        map[string]string
		elementValueMap     map[string]string

		// process variables
		processEncodingMap  map[string]string
		processEncryptedMap map[string]string
		processValueMap     map[string]string

		cmd engine.UpdateUserTaskCmd
	)

	c := cobra.Command{
		Use:   "update",
		Short: "Update a user task",
		RunE: func(c *cobra.Command, _ []string) error {
			tags := make([]engine.Tag, 0, len(tagsV))
			for name, value := range tagsV {
				tags = append(tags, engine.Tag{
					Name:  name,
					Value: value,
				})
			}

			elementVariables, err := mapElementVariables(elementIdMap, elementEncodingMap, elementEncryptedMap, elementValueMap)
			if err != nil {
				return err
			}

			processVariables, err := mapProcessVariables(processEncodingMap, processEncryptedMap, processValueMap)
			if err != nil {
				return err
			}

			cmd.Partition = engine.Partition(partition)

			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.Tags = tags
			cmd.WorkerId = cli.workerId

			userTask, err := cli.e.UpdateUserTask(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(userTask)
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "User task partition")
	c.Flags().Int32Var(&cmd.Id, "id", 0, "User task ID")

	c.Flags().Int32Var(&cmd.Revision, "revision", 0, "User task revision that should be updated")

	c.Flags().StringToStringVar(&elementEncodingMap, "element-variable-encoding", nil, "Variable to set or delete at element instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&elementEncryptedMap, "element-variable-encrypted", nil, "Variable to set or delete at element instance scope\nDetermines if a value is encrypted before it is stored.")
	c.Flags().StringToStringVar(&elementIdMap, "element-variable-id", nil, "Variable to set or delete at element instance scope\nBPMN element ID to determine the variable's scope")
	c.Flags().StringToStringVar(&elementValueMap, "element-variable-value", nil, "Variable to set or delete at element instance scope\nData value, encoded as a string")
	c.Flags().StringVar(&cmd.ErrorCode, "error-code", "", "Code of a BPMN error, used to throw an error")
	c.Flags().StringVar(&cmd.EscalationCode, "escalation-code", "", "Code of a BPMN escalation, used to escalate a user task")
	c.Flags().BoolVar(&cmd.IsCompleted, "completed", false, "Determines if a user task is completed and the execution is continued")
	c.Flags().StringToStringVar(&processEncodingMap, "process-variable-encoding", nil, "Variable to set or delete at process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&processEncryptedMap, "process-variable-encrypted", nil, "Variable to set or delete at process instance scope\nDetermines if a value is encrypted before it is stored.")
	c.Flags().StringToStringVar(&processValueMap, "process-variable-value", nil, "Variable to set or delete at process instance scope\nData value, encoded as a string")
	c.Flags().StringToStringVar(&tagsV, "tag", nil, "Tag, consisting of name and value")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("revision")

	return &c
}

func newUserTaskQueryCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue
		tagsV     map[string]string

		criteria engine.UserTaskCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query user tasks",
		RunE: func(c *cobra.Command, _ []string) error {
			tags := make([]engine.Tag, 0, len(tagsV))
			for name, value := range tagsV {
				tags = append(tags, engine.Tag{
					Name:  name,
					Value: value,
				})
			}

			criteria.Partition = engine.Partition(partition)
			criteria.Tags = tags

			q := cli.e.CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryUserTasks(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := newTable([]string{
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
				table.addRow([]string{
					result.Partition.String(),
					strconv.Itoa(int(result.Id)),
					strconv.Itoa(int(result.Revision)),
					strconv.Itoa(int(result.ProcessId)),
					strconv.Itoa(int(result.ProcessInstanceId)),
					formatTime(result.CreatedAt),
					result.State.String(),
					formatTime(result.UpdatedAt),
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

	c.Flags().StringToStringVar(&tagsV, "tag", nil, "Tag, consisting of name and value")

	flagQueryOptions(&c, &options)

	return &c
}
