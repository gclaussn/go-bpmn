package element

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

const (
	allColumns     = "id,processId,bpmnElementId,bpmnElementName,bpmnElementType,multiInstance,parentBpmnElementId,hasEventDefinition,eventDefinition,eventDefinitionSuspended"
	defaultColumns = "id,processId,bpmnElementId,bpmnElementType,parentBpmnElementId,hasEventDefinition"
)

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "element",
		Short: "Query elements",
		RunE:  common.Help,
	}

	c.AddCommand(newQueryCmd())

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		criteria engine.ElementCriteria
		options  engine.QueryOptions
	)

	formatter := common.NewTableFormatter(allColumns, defaultColumns)

	c := cobra.Command{
		Use:         "query",
		Short:       "Query elements",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryElements(context.Background(), criteria)
			if err != nil {
				return err
			}

			if ok, err := formatter.Format(c, results, common.Rows(results)); ok || err != nil {
				return err
			}

			table := common.NewTable(allColumns)

			for _, result := range results {
				table.AddRow([]string{
					common.FormatId(result.Id),

					common.FormatId(result.ProcessId),

					result.BpmnElementId,
					result.BpmnElementName,
					result.BpmnElementType.String(),
					strconv.FormatBool(result.IsMultiInstance),
					result.ParentBpmnElementId,

					strconv.FormatBool(result.EventDefinition != nil),
					formatEventDefinition(result.EventDefinition),
					formatEventDefinitionSuspended(result.EventDefinition),
				})
			}

			table.Format(c, formatter.SelectedColumns())
			return nil
		},
	}

	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process filter")

	c.Flags().StringVar(&criteria.BpmnElementId, "bpmn-element-id", "", "BPMN element ID filter")

	common.FlagQueryOptions(&c, &options)

	formatter.Flag(&c)

	return &c
}

func formatEventDefinition(d *engine.EventDefinition) string {
	if d == nil {
		return ""
	}
	switch {
	case d.ErrorCode != "":
		return fmt.Sprintf("errorCode=%s", d.ErrorCode)
	case d.EscalationCode != "":
		return fmt.Sprintf("escalationCode=%s", d.EscalationCode)
	case d.MessageName != "":
		return fmt.Sprintf("messageName=%s", d.MessageName)
	case d.SignalName != "":
		return fmt.Sprintf("signalName=%s", d.SignalName)
	case d.Timer != nil:
		return fmt.Sprintf("timer=%s", d.Timer)
	default:
		return ""
	}
}

func formatEventDefinitionSuspended(d *engine.EventDefinition) string {
	if d == nil {
		return ""
	}
	return strconv.FormatBool(d.IsSuspended)
}
