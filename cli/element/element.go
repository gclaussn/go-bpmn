package element

import (
	"context"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
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

			table := common.NewTable([]string{
				"ID",
				"PROCESS ID",
				"BPMN ELEMENT ID",
				"BPMN ELEMENT NAME",
				"BPMN ELEMENT TYPE",
			})

			for _, result := range results {
				table.AddRow([]string{
					strconv.Itoa(int(result.Id)),
					strconv.Itoa(int(result.ProcessId)),
					result.BpmnElementId,
					result.BpmnElementName,
					result.BpmnElementType.String(),
				})
			}

			c.Print(table.String())
			return nil
		},
	}

	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process filter")

	c.Flags().StringVar(&criteria.BpmnElementId, "bpmn-element-id", "", "BPMN element ID filter")

	common.FlagQueryOptions(&c, &options)

	return &c
}
