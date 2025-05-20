package cli

import (
	"strconv"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newElementCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "element",
		Short:       "Query elements",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newElementQueryCmd(cli))

	return &c
}

func newElementQueryCmd(cli *Cli) *cobra.Command {
	var (
		criteria engine.ElementCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query elements",
		RunE: func(c *cobra.Command, _ []string) error {
			results, err := cli.engine.QueryWithOptions(criteria, options)
			if err != nil {
				return err
			}

			table := newTable([]string{
				"ID",
				"PROCESS ID",
				"BPMN ELEMENT ID",
				"BPMN ELEMENT NAME",
				"BPMN ELEMENT TYPE",
			})

			for i := 0; i < len(results); i++ {
				element := results[i].(engine.Element)

				table.addRow([]string{
					strconv.Itoa(int(element.Id)),
					strconv.Itoa(int(element.ProcessId)),
					element.BpmnElementId,
					element.BpmnElementName,
					element.BpmnElementType.String(),
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process ID")

	flagQueryOptions(&c, &options)

	return &c
}
