package cli

import (
	"context"
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
			q := cli.e.CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryElements(context.Background(), criteria)
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

			for _, result := range results {
				table.addRow([]string{
					strconv.Itoa(int(result.Id)),
					strconv.Itoa(int(result.ProcessId)),
					result.BpmnElementId,
					result.BpmnElementName,
					result.BpmnElementType.String(),
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
