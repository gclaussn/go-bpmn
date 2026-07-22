package variable

import (
	"context"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "variable",
		Short: "Query variables",
		RunE:  common.Help,
	}

	c.AddCommand(newQueryCmd())

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		partition common.Partition

		criteria engine.VariableCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:         "query",
		Short:       "Query variables",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)

			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryVariables(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := common.NewTable([]string{
				"PARTITION",
				"NAME",
				"PROCESS INSTANCE ID",
				"ELEMENT INSTANCE ID",
				"ENCODING",
				"CREATED AT",
				"UPDATED AT",
			})

			for _, result := range results {
				table.AddRow([]string{
					result.Partition.String(),
					result.Name,
					strconv.Itoa(int(result.ProcessInstanceId)),
					strconv.Itoa(int(result.ElementInstanceId)),
					result.Encoding,
					common.FormatTime(result.CreatedAt),
					common.FormatTime(result.UpdatedAt),
				})
			}

			c.Print(table.String())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition filter")

	c.Flags().Int32Var(&criteria.ElementInstanceId, "element-instance-id", 0, "Element instance filter")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance filter")
	c.Flags().StringSliceVarP(&criteria.Names, "name", "n", nil, "Names of variables to include")

	common.FlagQueryOptions(&c, &options)

	return &c
}
