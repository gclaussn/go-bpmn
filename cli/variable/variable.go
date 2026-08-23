package variable

import (
	"context"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

const (
	allColumns     = "partition,id,elementId,elementInstanceId,processId,processInstanceId,createdAt,createdBy,encoding,encrypted,name,updatedAt,updatedBy"
	defaultColumns = "partition,name,processInstanceId,elementInstanceId,encoding,updatedAt"
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

	formatter := common.NewTableFormatter(allColumns, defaultColumns)

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

			if ok, err := formatter.Format(c, results, common.Rows(results)); ok || err != nil {
				return err
			}

			table := common.NewTable(allColumns)

			for _, result := range results {
				table.AddRow([]string{
					result.Partition.String(),
					common.FormatId(result.Id),

					common.FormatId(result.ElementId),
					common.FormatId(result.ElementInstanceId),
					common.FormatId(result.ProcessId),
					common.FormatId(result.ProcessInstanceId),

					common.FormatTime(result.CreatedAt),
					result.CreatedBy,
					result.Encoding,
					strconv.FormatBool(result.IsEncrypted),
					result.Name,
					common.FormatTime(result.UpdatedAt),
					result.UpdatedBy,
				})
			}

			table.Format(c, formatter.SelectedColumns())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition filter")

	c.Flags().Int32Var(&criteria.ElementInstanceId, "element-instance-id", 0, "Element instance filter")
	c.Flags().Int32Var(&criteria.ProcessInstanceId, "process-instance-id", 0, "Process instance filter")
	c.Flags().StringSliceVarP(&criteria.Names, "name", "n", nil, "Names of variables to include")

	common.FlagQueryOptions(&c, &options)

	formatter.Flag(&c)

	return &c
}
