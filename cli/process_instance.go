package cli

import (
	"context"
	"strconv"
	"strings"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newProcessInstanceCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "process-instance",
		Short:       "Manage and query process instances",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newProcessInstanceCreateCmd(cli))
	c.AddCommand(newProcessInstanceGetVariablesCmd(cli))
	c.AddCommand(newProcessInstanceResumeCmd(cli))
	c.AddCommand(newProcessInstanceSetVariablesCmd(cli))
	c.AddCommand(newProcessInstanceSuspendCmd(cli))
	c.AddCommand(newProcessInstanceQueryCmd(cli))

	return &c
}

func newProcessInstanceCreateCmd(cli *Cli) *cobra.Command {
	var (
		tagsV map[string]string

		// variables
		encodingMap  map[string]string
		encryptedMap map[string]string
		valueMap     map[string]string

		cmd engine.CreateProcessInstanceCmd
	)

	c := cobra.Command{
		Use:   "create",
		Short: "Create a process instance",
		RunE: func(c *cobra.Command, _ []string) error {
			tags := make([]engine.Tag, 0, len(tagsV))
			for name, value := range tagsV {
				tags = append(tags, engine.Tag{
					Name:  name,
					Value: value,
				})
			}

			variables, err := mapVariables(encodingMap, encryptedMap, valueMap)
			if err != nil {
				return err
			}

			cmd.Tags = tags
			cmd.Variables = variables
			cmd.WorkerId = cli.workerId

			processInstance, err := cli.e.CreateProcessInstance(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Println(processInstance.Id)
			return nil
		},
	}

	c.Flags().StringVar(&cmd.BpmnProcessId, "bpmn-process-id", "", "BPMN ID of an existing process")
	c.Flags().StringVar(&cmd.CorrelationKey, "correlation-key", "", "Optional key, used to correlate a process instance with a business entity")
	c.Flags().StringToStringVar(&tagsV, "tag", nil, "Tag, consisting of name and value")
	c.Flags().StringToStringVar(&encodingMap, "variable-encoding", nil, "Variable to set at process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&encryptedMap, "variable-encrypted", nil, "Variable to set at process instance scope\nDetermines if a value is encrypted before it is stored.")
	c.Flags().StringToStringVar(&valueMap, "variable-value", nil, "Variable to set at process instance scope\nData value, encoded as a string")
	c.Flags().StringVar(&cmd.Version, "version", "", "Version of an existing process")

	c.MarkFlagRequired("bpmn-process-id")
	c.MarkFlagRequired("version")

	return &c
}

func newProcessInstanceGetVariablesCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		cmd engine.GetProcessVariablesCmd
	)

	c := cobra.Command{
		Use:   "get-variables",
		Short: "Get process variables",
		RunE: func(c *cobra.Command, args []string) error {
			cmd.Partition = engine.Partition(partition)

			variables, err := cli.e.GetProcessVariables(context.Background(), cmd)
			if err != nil {
				return err
			}

			var sb strings.Builder
			for i, variable := range variables {
				if i != 0 {
					sb.WriteRune('\n')
				}

				sb.WriteString(variable.Name)
				sb.WriteString(" (encoding: ")
				sb.WriteString(variable.Data.Encoding)
				sb.WriteString(", encrypted: ")
				sb.WriteString(strconv.FormatBool(variable.Data.IsEncrypted))
				sb.WriteString(")\n")
				sb.WriteString(variable.Data.Value)
				sb.WriteRune('\n')
			}

			c.Print(sb.String())
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Process instance partition")
	c.Flags().Int32Var(&cmd.ProcessInstanceId, "id", 0, "Process instance ID")

	c.Flags().StringSliceVarP(&cmd.Names, "name", "n", nil, "Names of process variables to get")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newProcessInstanceResumeCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		cmd engine.ResumeProcessInstanceCmd
	)

	c := cobra.Command{
		Use:   "resume",
		Short: "Resume a process instance",
		RunE: func(c *cobra.Command, args []string) error {
			cmd.Partition = engine.Partition(partition)
			cmd.WorkerId = cli.workerId

			return cli.e.ResumeProcessInstance(context.Background(), cmd)
		},
	}

	c.Flags().Var(&partition, "partition", "Process instance partition")
	c.Flags().Int32Var(&cmd.Id, "id", 0, "Process instance ID")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newProcessInstanceSetVariablesCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		// variables
		encodingMap  map[string]string
		encryptedMap map[string]string
		valueMap     map[string]string

		cmd engine.SetProcessVariablesCmd
	)

	c := cobra.Command{
		Use:   "set-variables",
		Short: "Get process variables",
		RunE: func(c *cobra.Command, args []string) error {
			variables, err := mapVariables(encodingMap, encryptedMap, valueMap)
			if err != nil {
				return err
			}

			cmd.Partition = engine.Partition(partition)
			cmd.Variables = variables
			cmd.WorkerId = cli.workerId

			return cli.e.SetProcessVariables(context.Background(), cmd)
		},
	}

	c.Flags().Var(&partition, "partition", "Process instance partition")
	c.Flags().Int32Var(&cmd.ProcessInstanceId, "id", 0, "Process instance ID")

	c.Flags().StringToStringVar(&encodingMap, "variable-encoding", nil, "Variable to set or delete\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&encryptedMap, "variable-encrypted", nil, "Variable to set or delete\nDetermines if a value is encrypted before it is stored.")
	c.Flags().StringToStringVar(&valueMap, "variable-value", nil, "Variable to set or delete\nData value, encoded as a string")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newProcessInstanceSuspendCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		cmd engine.SuspendProcessInstanceCmd
	)

	c := cobra.Command{
		Use:   "suspend",
		Short: "Suspend a process instance",
		RunE: func(c *cobra.Command, args []string) error {
			cmd.Partition = engine.Partition(partition)
			cmd.WorkerId = cli.workerId

			return cli.e.SuspendProcessInstance(context.Background(), cmd)
		},
	}

	c.Flags().Var(&partition, "partition", "Process instance partition")
	c.Flags().Int32Var(&cmd.Id, "id", 0, "Process instance ID")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newProcessInstanceQueryCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue
		tagsV     map[string]string

		criteria engine.ProcessInstanceCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query processes",
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

			results, err := q.QueryProcessInstances(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := newTable([]string{
				"PARTITION",
				"ID",
				"BPMN PROCESS ID",
				"CREATED AT",
				"CREATED BY",
				"ENDED AT",
				"STATE",
			})

			for _, result := range results {
				table.addRow([]string{
					result.Partition.String(),
					strconv.Itoa(int(result.Id)),
					result.BpmnProcessId,
					formatTime(result.CreatedAt),
					result.CreatedBy,
					formatTimeOrNil(result.EndedAt),
					result.State.String(),
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Process instance partition")
	c.Flags().Int32Var(&criteria.Id, "id", 0, "Process instance ID")

	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process ID")
	c.Flags().StringToStringVar(&tagsV, "tag", nil, "Tag, consisting of name and value")

	flagQueryOptions(&c, &options)

	return &c
}
