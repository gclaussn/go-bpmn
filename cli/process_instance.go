package cli

import (
	"encoding/json"
	"fmt"
	"sort"
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
		variablesV map[string]string

		cmd engine.CreateProcessInstanceCmd
	)

	c := cobra.Command{
		Use:   "create",
		Short: "Create a process instance",
		RunE: func(c *cobra.Command, _ []string) error {
			variables := make(map[string]*engine.Data)
			for variableName, dataJson := range variablesV {
				var data engine.Data
				if err := json.Unmarshal([]byte(dataJson), &data); err != nil {
					return fmt.Errorf("failed to unmarshal variable %s: %v", variableName, err)
				}
				variables[variableName] = &data
			}

			cmd.Variables = variables
			cmd.WorkerId = cli.workerId

			processInstance, err := cli.engine.CreateProcessInstance(cmd)
			if err != nil {
				return err
			}

			c.Println(processInstance.Id)
			return nil
		},
	}

	c.Flags().StringVar(&cmd.BpmnProcessId, "bpmn-process-id", "", "BPMN ID of an existing process")
	c.Flags().StringVar(&cmd.CorrelationKey, "correlation-key", "", "Optional key, used to correlate a process instance with a business entity")
	c.Flags().StringToStringVar(&cmd.Tags, "tag", nil, "Tag, consisting of name and value")
	c.Flags().StringToStringVar(&variablesV, "variable", nil, "Variable to set at process instance scope")
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

			variables, err := cli.engine.GetProcessVariables(cmd)
			if err != nil {
				return err
			}

			keys := make([]string, 0, len(variables))
			for key := range variables {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			var sb strings.Builder
			for i := range keys {
				data := variables[keys[i]]

				if i != 0 {
					sb.WriteRune('\n')
				}

				sb.WriteString(keys[i])
				sb.WriteString(" (encoding: ")
				sb.WriteString(data.Encoding)
				sb.WriteString(", encrypted: ")
				sb.WriteString(strconv.FormatBool(data.IsEncrypted))
				sb.WriteString(")\n")
				sb.WriteString(data.Value)
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

			return cli.engine.ResumeProcessInstance(cmd)
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
		partition  partitionValue
		variablesV map[string]string

		cmd engine.SetProcessVariablesCmd
	)

	c := cobra.Command{
		Use:   "set-variables",
		Short: "Get process variables",
		RunE: func(c *cobra.Command, args []string) error {
			variables := make(map[string]*engine.Data)
			for variableName, dataJson := range variablesV {
				if dataJson == "" || dataJson == "null" {
					variables[variableName] = nil
					continue
				}

				var data engine.Data
				if err := json.Unmarshal([]byte(dataJson), &data); err != nil {
					return fmt.Errorf("failed to unmarshal variable %s: %v", variableName, err)
				}
				variables[variableName] = &data
			}

			cmd.Partition = engine.Partition(partition)
			cmd.Variables = variables
			cmd.WorkerId = cli.workerId

			return cli.engine.SetProcessVariables(cmd)
		},
	}

	c.Flags().Var(&partition, "partition", "Process instance partition")
	c.Flags().Int32Var(&cmd.ProcessInstanceId, "id", 0, "Process instance ID")

	c.Flags().StringToStringVar(&variablesV, "variable", nil, "Variable to set or delete")

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

			return cli.engine.SuspendProcessInstance(cmd)
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

		criteria engine.ProcessInstanceCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query processes",
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)

			results, err := cli.engine.QueryWithOptions(criteria, options)
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

			for i := range results {
				processInstance := results[i].(engine.ProcessInstance)

				table.addRow([]string{
					processInstance.Partition.String(),
					strconv.Itoa(int(processInstance.Id)),
					processInstance.BpmnProcessId,
					formatTime(processInstance.CreatedAt),
					processInstance.CreatedBy,
					formatTimeOrNil(processInstance.EndedAt),
					processInstance.State.String(),
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Process instance partition")
	c.Flags().Int32Var(&criteria.Id, "id", 0, "Process instance ID")

	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process ID")
	c.Flags().StringToStringVar(&criteria.Tags, "tag", nil, "Tag, consisting of name and value")

	flagQueryOptions(&c, &options)

	return &c
}
