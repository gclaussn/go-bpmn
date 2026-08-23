package process_instance

import (
	"context"
	_ "embed"
	"strconv"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

//go:embed process_instance.tpl
var processInstanceTemplate string

//go:embed process_instance_variables.tpl
var processInstanceVariablesTemplate string

const (
	allColumns     = "partition,id,parentId,rootId,processId,bpmnProcessId,correlationKey,createdAt,createdBy,endedAt,startedAt,state,tagCount,version"
	defaultColumns = "partition,id,processId,bpmnProcessId,createdAt,endedAt,state,tagCount"
)

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "process-instance",
		Short: "Manage and query process instances",
		RunE:  common.Help,
	}

	c.AddCommand(
		newCreateCmd(),
		newGetVariablesCmd(),
		newResumeCmd(),
		newSetVariablesCmd(),
		newSuspendCmd(),
		newQueryCmd(),
	)

	return &c
}

func newCreateCmd() *cobra.Command {
	var (
		tagMap    map[string]string
		variables common.ProcessVariables

		cmd engine.CreateProcessInstanceCmd

		formatter common.Formatter
	)

	c := cobra.Command{
		Use:         "create",
		Short:       "Create a process instance",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			variables, err := variables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Tags = common.Tags(tagMap)
			cmd.Variables = variables
			cmd.WorkerId = workerId

			processInstance, err := e.CreateProcessInstance(context.Background(), cmd)
			if err != nil {
				return err
			}

			return formatter.Format(c, processInstance, processInstanceTemplate)
		},
	}

	c.Flags().StringVar(&cmd.BpmnProcessId, "bpmn-process-id", "", "BPMN ID of an existing process")
	c.Flags().StringVar(&cmd.CorrelationKey, "correlation-key", "", "Optional key, used to correlate a process instance with a business entity")
	c.Flags().StringToStringVarP(&tagMap, "tag", "t", nil, "Tags")
	c.Flags().StringToStringVar(&variables.EncodingMap, "variable-encoding", nil, "Variable to set at process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&variables.EncryptedMap, "variable-encrypted", nil, "Variable to set at process instance scope\nDetermines if a value is encrypted before it is stored")
	c.Flags().StringToStringVar(&variables.ValueMap, "variable", nil, "Variable to set at process instance scope\nData value, encoded as a string")
	c.Flags().StringVar(&cmd.Version, "version", "", "Version of an existing process")

	formatter.Flag(&c)

	c.MarkFlagRequired("bpmn-process-id")
	c.MarkFlagRequired("version")

	return &c
}

func newGetVariablesCmd() *cobra.Command {
	var (
		partition common.Partition

		cmd engine.GetProcessVariablesCmd

		formatter common.Formatter
	)

	c := cobra.Command{
		Use:         "get-variables",
		Short:       "Get process variables",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, args []string) error {
			cmd.Partition = engine.Partition(partition)

			variables, err := common.GetEngine(c).GetProcessVariables(context.Background(), cmd)
			if err != nil {
				return err
			}

			return formatter.Format(c, variables, processInstanceVariablesTemplate)
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Process instance partition")
	c.Flags().Int32VarP(&cmd.ProcessInstanceId, "id", "i", 0, "Process instance ID")

	c.Flags().StringSliceVarP(&cmd.Names, "name", "n", nil, "Names of process variables to get")

	formatter.Flag(&c)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newResumeCmd() *cobra.Command {
	var (
		partition common.Partition

		cmd engine.ResumeProcessInstanceCmd
	)

	c := cobra.Command{
		Use:         "resume",
		Short:       "Resume a process instance",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, args []string) error {
			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.WorkerId = workerId

			return e.ResumeProcessInstance(context.Background(), cmd)
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Process instance partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Process instance ID")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newSetVariablesCmd() *cobra.Command {
	var (
		partition common.Partition

		variables common.ProcessVariables

		cmd engine.SetProcessVariablesCmd
	)

	c := cobra.Command{
		Use:         "set-variables",
		Short:       "Set process variables",
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

			return e.SetProcessVariables(context.Background(), cmd)
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Process instance partition")
	c.Flags().Int32VarP(&cmd.ProcessInstanceId, "id", "i", 0, "Process instance ID")

	c.Flags().StringToStringVar(&variables.EncodingMap, "variable-encoding", nil, "Variable to set or delete at process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&variables.EncryptedMap, "variable-encrypted", nil, "Variable to set or delete at process instance scope\nDetermines if a value is encrypted before it is stored")
	c.Flags().StringToStringVar(&variables.ValueMap, "variable", nil, "Variable to set or delete at process instance scope\nData value, encoded as a string")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newSuspendCmd() *cobra.Command {
	var (
		partition common.Partition

		cmd engine.SuspendProcessInstanceCmd
	)

	c := cobra.Command{
		Use:         "suspend",
		Short:       "Suspend a process instance",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, args []string) error {
			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.WorkerId = workerId

			return e.SuspendProcessInstance(context.Background(), cmd)
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Process instance partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Process instance ID")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		partition common.Partition

		tagMap map[string]string

		criteria engine.ProcessInstanceCriteria
		options  engine.QueryOptions
	)

	formatter := common.NewTableFormatter(allColumns, defaultColumns)

	c := cobra.Command{
		Use:         "query",
		Short:       "Query process instances",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Partition = engine.Partition(partition)
			criteria.Tags = common.Tags(tagMap)

			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryProcessInstances(context.Background(), criteria)
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

					common.FormatId(result.ParentId),
					common.FormatId(result.RootId),

					common.FormatId(result.ProcessId),

					result.BpmnProcessId,
					result.CorrelationKey,
					common.FormatTime(result.CreatedAt),
					result.CreatedBy,
					common.FormatTimeOrNil(result.EndedAt),
					common.FormatTimeOrNil(result.StartedAt),
					result.State.String(),
					strconv.Itoa(len(result.Tags)),
					result.Version,
				})
			}

			table.Format(c, formatter.SelectedColumns())
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Partition filter")
	c.Flags().Int32VarP(&criteria.Id, "id", "i", 0, "Process instance filter")

	c.Flags().Int32Var(&criteria.ParentId, "parent-id", 0, "Filter, used to query process instances that have a specific parent process instance")
	c.Flags().Int32Var(&criteria.RootId, "root-id", 0, "Filter, used to query process instances descending from a root process instance (which is included)")

	c.Flags().Int32Var(&criteria.ProcessId, "process-id", 0, "Process filter")

	c.Flags().StringToStringVarP(&tagMap, "tag", "t", nil, "Tags, a process instance must have, to be included")

	common.FlagQueryOptions(&c, &options)

	formatter.Flag(&c)

	return &c
}
