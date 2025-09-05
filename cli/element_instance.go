package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newElementInstanceCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "element-instance",
		Short:       "Manage and query element instances",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newElementInstanceGetVariablesCmd(cli))
	c.AddCommand(newElementInstanceSetVariablesCmd(cli))
	c.AddCommand(newElementInstanceQueryCmd(cli))

	return &c
}

func newElementInstanceGetVariablesCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		cmd engine.GetElementVariablesCmd
	)

	c := cobra.Command{
		Use:   "get-variables",
		Short: "Get element variables",
		RunE: func(c *cobra.Command, args []string) error {
			cmd.Partition = engine.Partition(partition)

			variables, err := cli.e.GetElementVariables(context.Background(), cmd)
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

	c.Flags().Var(&partition, "partition", "Element instance partition")
	c.Flags().Int32Var(&cmd.ElementInstanceId, "id", 0, "Element instance ID")

	c.Flags().StringSliceVarP(&cmd.Names, "name", "n", nil, "Names of element variables to get")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newElementInstanceSetVariablesCmd(cli *Cli) *cobra.Command {
	var (
		partition  partitionValue
		variablesV map[string]string

		cmd engine.SetElementVariablesCmd
	)

	c := cobra.Command{
		Use:   "set-variables",
		Short: "Set element variables",
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

			return cli.e.SetElementVariables(context.Background(), cmd)
		},
	}

	c.Flags().Var(&partition, "partition", "Element instance partition")
	c.Flags().Int32Var(&cmd.ElementInstanceId, "id", 0, "Element instance ID")

	c.Flags().StringToStringVar(&variablesV, "variable", nil, "Variable to set or delete")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newElementInstanceQueryCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue
		statesV   []string

		criteria engine.ElementInstanceCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query element instances",
		RunE: func(c *cobra.Command, _ []string) error {
			states := make([]engine.InstanceState, len(statesV))
			for i := range states {
				var value instanceStateValue
				if err := value.Set(statesV[i]); err != nil {
					return err
				}
				states[i] = engine.InstanceState(value)
			}

			criteria.Partition = engine.Partition(partition)
			criteria.States = states

			q := cli.e.CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryElementInstances(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := newTable([]string{
				"PARTITION",
				"ID",
				"PROCESS ID",
				"PROCESS INSTANCE ID",
				"BPMN ELEMENT NAME",
				"BPMN ELEMENT TYPE",
				"ENDED AT",
				"STATE",
			})

			for _, result := range results {
				table.addRow([]string{
					result.Partition.String(),
					strconv.Itoa(int(result.Id)),
					strconv.Itoa(int(result.ProcessId)),
					strconv.Itoa(int(result.ProcessInstanceId)),
					result.BpmnElementId,
					result.BpmnElementType.String(),
					formatTimeOrNil(result.EndedAt),
					result.State.String(),
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Element instance partition")
	c.Flags().Int32Var(&criteria.Id, "id", 0, "Element instance ID")

	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process ID")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance ID")

	c.Flags().StringVar(&criteria.BpmnElementId, "bpmn-element-id", "", "BPMN element ID")
	c.Flags().StringSliceVar(&statesV, "state", nil, "States to include")

	flagQueryOptions(&c, &options)

	return &c
}
