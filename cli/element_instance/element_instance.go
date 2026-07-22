package element_instance

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "element-instance",
		Short: "Manage and query element instances",
		RunE:  common.Help,
	}

	c.AddCommand(
		newGetVariablesCmd(),
		newSetVariablesCmd(),
		newQueryCmd(),
	)

	return &c
}

func newGetVariablesCmd() *cobra.Command {
	var (
		partition common.Partition

		cmd engine.GetElementVariablesCmd
	)

	c := cobra.Command{
		Use:         "get-variables",
		Short:       "Get element variables",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, args []string) error {
			cmd.Partition = engine.Partition(partition)

			variables, err := common.GetEngine(c).GetElementVariables(context.Background(), cmd)
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

	c.Flags().VarP(&partition, "partition", "p", "Element instance partition")
	c.Flags().Int32VarP(&cmd.ElementInstanceId, "id", "i", 0, "Element instance ID")

	c.Flags().BoolVar(&cmd.ExcludeParentVariables, "exclude-parent-variables", false, "Determines if variables of direct or indirect parent element instances are not returned")
	c.Flags().StringSliceVarP(&cmd.Names, "name", "n", nil, "Names of element variables to get")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newSetVariablesCmd() *cobra.Command {
	var (
		partition common.Partition

		variables common.ElementVariables

		cmd engine.SetElementVariablesCmd
	)

	c := cobra.Command{
		Use:         "set-variables",
		Short:       "Set element variables",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, args []string) error {
			variables, err := variables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Variables = variables
			cmd.WorkerId = workerId

			return e.SetElementVariables(context.Background(), cmd)
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Element instance partition")
	c.Flags().Int32VarP(&cmd.ElementInstanceId, "id", "i", 0, "Element instance ID")

	c.Flags().StringToStringVar(&variables.BpmnElementIdMap, "variable-bpmn-element-id", nil, "Variable to set or delete\nBPMN element ID to determine the variable's scope")
	c.Flags().StringToStringVar(&variables.EncodingMap, "variable-encoding", nil, "Variable to set or delete\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&variables.EncryptedMap, "variable-encrypted", nil, "Variable to set or delete\nDetermines if a value is encrypted before it is stored")
	c.Flags().StringToStringVar(&variables.ValueMap, "variable", nil, "Variable to set or delete\nData value, encoded as a string")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		partition common.Partition

		stateSlice []string

		criteria engine.ElementInstanceCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:         "query",
		Short:       "Query element instances",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			states := make([]engine.InstanceState, len(stateSlice))
			for i := range stateSlice {
				state := engine.MapInstanceState(stateSlice[i])
				if state == 0 {
					return fmt.Errorf("invalid instance state %s", stateSlice[i])
				}

				states[i] = state
			}

			criteria.Partition = engine.Partition(partition)
			criteria.States = states

			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryElementInstances(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := common.NewTable([]string{
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
				table.AddRow([]string{
					result.Partition.String(),
					strconv.Itoa(int(result.Id)),
					strconv.Itoa(int(result.ProcessId)),
					strconv.Itoa(int(result.ProcessInstanceId)),
					result.BpmnElementId,
					result.BpmnElementType.String(),
					common.FormatTimeOrNil(result.EndedAt),
					result.State.String(),
				})
			}

			c.Print(table.String())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition filter")
	c.Flags().Int32VarP(&criteria.Id, "id", "i", 0, "Element instance filter")

	c.Flags().Int32Var(&criteria.ParentId, "parent-id", 0, "Filter, used to query children of a parent element instance")

	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process filter")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance filter")

	c.Flags().StringVar(&criteria.BpmnElementId, "bpmn-element-id", "", "BPMN element ID filter")
	c.Flags().StringSliceVar(&stateSlice, "state", nil, "States to include")

	common.FlagQueryOptions(&c, &options)

	return &c
}
