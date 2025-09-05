package cli

import (
	"context"
	"strconv"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newVariableCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "variable",
		Short:       "Query variables",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newVariableQueryCmd(cli))

	return &c
}

func newVariableQueryCmd(cli *Cli) *cobra.Command {
	var (
		partition partitionValue

		criteria engine.VariableCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query variables",
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)

			q := cli.e.CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryVariables(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := newTable([]string{
				"PARTITION",
				"NAME",
				"PROCESS INSTANCE ID",
				"ELEMENT INSTANCE ID",
				"ENCODING",
				"CREATED AT",
				"UPDATED AT",
			})

			for _, result := range results {
				table.addRow([]string{
					result.Partition.String(),
					result.Name,
					strconv.Itoa(int(result.ProcessInstanceId)),
					strconv.Itoa(int(result.ElementInstanceId)),
					result.Encoding,
					formatTime(result.CreatedAt),
					formatTime(result.UpdatedAt),
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().Var(&partition, "partition", "Variable partition")

	c.Flags().Int32Var(&criteria.ElementInstanceId, "element-instance-id", 0, "Element instance ID")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance ID")
	c.Flags().StringSliceVar(&criteria.Names, "name", nil, "Names of variables to include")

	flagQueryOptions(&c, &options)

	return &c
}
