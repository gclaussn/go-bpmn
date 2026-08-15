package process

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

//go:embed process.tpl
var processTemplate string

func NewCmd() *cobra.Command {
	c := cobra.Command{
		Use:   "process",
		Short: "Manage and query processes",
		RunE:  common.Help,
	}

	c.AddCommand(
		newCreateCmd(),
		newGetBpmnXmlCmd(),
		newQueryCmd(),
	)

	return &c
}

func newCreateCmd() *cobra.Command {
	var (
		bpmnFileName    string
		errorMap        map[string]string
		escalationMap   map[string]string
		messageMap      map[string]string
		signalMap       map[string]string
		tagMap          map[string]string
		timeMap         map[string]string
		timeCycleMap    map[string]string
		timeDurationMap map[string]string

		cmd engine.CreateProcessCmd

		formatter common.Formatter
	)

	c := cobra.Command{
		Use:         "create",
		Short:       "Create a process",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			bpmnFile, err := os.Open(bpmnFileName)
			if err != nil {
				return fmt.Errorf("failed to open BPMN file %s: %v", bpmnFileName, err)
			}

			defer bpmnFile.Close()

			bpmnXml, err := io.ReadAll(bpmnFile)
			if err != nil {
				return fmt.Errorf("failed to read BPMN file %s: %v", bpmnFileName, err)
			}

			timers := make([]engine.TimerDefinition, 0, len(timeMap)+len(timeCycleMap)+len(timeDurationMap))

			var timerErrs []error
			for bpmnElementId, v := range timeMap {
				var value common.Time
				if err := value.Set(v); err != nil {
					timerErrs = append(timerErrs, err)
					continue
				}
				timers = append(timers, engine.TimerDefinition{
					BpmnElementId: bpmnElementId,
					Timer:         &engine.Timer{Time: value.Time()},
				})
			}

			for bpmnElementId, v := range timeCycleMap {
				timers = append(timers, engine.TimerDefinition{
					BpmnElementId: bpmnElementId,
					Timer:         &engine.Timer{TimeCycle: v},
				})
			}

			for bpmnElementId, v := range timeDurationMap {
				var value common.ISO8601Duration
				if err := value.Set(v); err != nil {
					timerErrs = append(timerErrs, err)
					continue
				}
				timers = append(timers, engine.TimerDefinition{
					BpmnElementId: bpmnElementId,
					Timer:         &engine.Timer{TimeDuration: engine.ISO8601Duration(value)},
				})
			}

			if len(timerErrs) != 0 {
				return errors.Join(timerErrs...)
			}

			slices.SortFunc(timers, func(a engine.TimerDefinition, b engine.TimerDefinition) int {
				return strings.Compare(a.BpmnElementId, b.BpmnElementId)
			})

			errors := make([]engine.ErrorDefinition, 0, len(errorMap))
			for bpmnElementId, errorCode := range errorMap {
				errors = append(errors, engine.ErrorDefinition{
					BpmnElementId: bpmnElementId,
					ErrorCode:     errorCode,
				})
			}

			slices.SortFunc(errors, func(a engine.ErrorDefinition, b engine.ErrorDefinition) int {
				return strings.Compare(a.BpmnElementId, b.BpmnElementId)
			})

			escalations := make([]engine.EscalationDefinition, 0, len(escalationMap))
			for bpmnElementId, escalationCode := range escalationMap {
				escalations = append(escalations, engine.EscalationDefinition{
					BpmnElementId:  bpmnElementId,
					EscalationCode: escalationCode,
				})
			}

			slices.SortFunc(escalations, func(a engine.EscalationDefinition, b engine.EscalationDefinition) int {
				return strings.Compare(a.BpmnElementId, b.BpmnElementId)
			})

			messages := make([]engine.MessageDefinition, 0, len(messageMap))
			for bpmnElementId, messageName := range messageMap {
				messages = append(messages, engine.MessageDefinition{
					BpmnElementId: bpmnElementId,
					MessageName:   messageName,
				})
			}

			slices.SortFunc(messages, func(a engine.MessageDefinition, b engine.MessageDefinition) int {
				return strings.Compare(a.BpmnElementId, b.BpmnElementId)
			})

			signals := make([]engine.SignalDefinition, 0, len(signalMap))
			for bpmnElementId, signalName := range signalMap {
				signals = append(signals, engine.SignalDefinition{
					BpmnElementId: bpmnElementId,
					SignalName:    signalName,
				})
			}

			slices.SortFunc(signals, func(a engine.SignalDefinition, b engine.SignalDefinition) int {
				return strings.Compare(a.BpmnElementId, b.BpmnElementId)
			})

			tags := make([]engine.Tag, 0, len(tagMap))
			for name, value := range tagMap {
				tags = append(tags, engine.Tag{
					Name:  name,
					Value: value,
				})
			}

			slices.SortFunc(tags, func(a engine.Tag, b engine.Tag) int {
				return strings.Compare(a.Name, b.Name)
			})

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.BpmnXml = string(bpmnXml)
			cmd.Errors = errors
			cmd.Escalations = escalations
			cmd.Messages = messages
			cmd.Signals = signals
			cmd.Tags = common.Tags(tagMap)
			cmd.Timers = timers
			cmd.WorkerId = workerId

			process, err := e.CreateProcess(context.Background(), cmd)
			if err != nil {
				return err
			}

			return formatter.Format(c, process, processTemplate)
		},
	}

	c.Flags().StringVar(&bpmnFileName, "bpmn-file", "", "Path to a BPMN XML file")

	c.Flags().StringVar(&cmd.BpmnProcessId, "bpmn-process-id", "", "ID of the process element within the BPMN XML")
	c.Flags().StringToStringVar(&errorMap, "error", nil, "Mapping between BPMN element ID and error code")
	c.Flags().StringToStringVar(&escalationMap, "escalation", nil, "Mapping between BPMN element ID and escalation code")
	c.Flags().StringToStringVar(&messageMap, "message", nil, "Mapping between BPMN element ID and message name")
	c.Flags().IntVar(&cmd.Parallelism, "parallelism", 0, "Maximum number of parallel process instances being executed")
	c.Flags().StringToStringVar(&signalMap, "signal", nil, "Mapping between BPMN element ID and signal name")
	c.Flags().StringToStringVarP(&tagMap, "tag", "t", nil, "Tags")
	c.Flags().StringToStringVar(&timeMap, "time", nil, "A point in time, when the timer event is triggered")
	c.Flags().StringToStringVar(&timeCycleMap, "time-cycle", nil, "CRON expression that specifies a cyclic timer event")
	c.Flags().StringToStringVar(&timeDurationMap, "time-duration", nil, "Duration until the timer event is triggered")
	c.Flags().StringVar(&cmd.Version, "version", "", "Any process version")

	formatter.Flag(&c)

	c.MarkFlagRequired("bpmn-process-id")
	c.MarkFlagRequired("bpmn-xml-file")
	c.MarkFlagRequired("version")

	c.MarkFlagFilename("bpmn-file", ".bpmn", ".bpmn20.xml", ".xml")

	return &c
}

func newGetBpmnXmlCmd() *cobra.Command {
	var cmd engine.GetBpmnXmlCmd

	c := cobra.Command{
		Use:         "get-bpmn-xml",
		Short:       "Get BPMN XML",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			bpmnXml, err := common.GetEngine(c).GetBpmnXml(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(bpmnXml)
			return nil
		},
	}

	c.Flags().Int32VarP(&cmd.ProcessId, "id", "i", 0, "Process ID")

	c.MarkFlagRequired("id")

	return &c
}

func newQueryCmd() *cobra.Command {
	var (
		tagMap map[string]string

		criteria engine.ProcessCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:         "query",
		Short:       "Query processes",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			criteria.Tags = common.Tags(tagMap)

			q := common.GetEngine(c).CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryProcesses(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := common.NewTable([]string{
				"ID",
				"BPMN PROCESS ID",
				"VERSION",
				"CREATED AT",
				"CREATED BY",
			})

			for _, result := range results {
				table.AddRow([]string{
					strconv.Itoa(int(result.Id)),
					result.BpmnProcessId,
					result.Version,
					common.FormatTime(result.CreatedAt),
					result.CreatedBy,
				})
			}

			c.Print(table.String())
			return nil
		},
	}

	c.Flags().Int32VarP(&criteria.Id, "id", "i", 0, "Process filter")

	c.Flags().StringToStringVarP(&tagMap, "tag", "t", nil, "Tags, a process must have, to be included")

	common.FlagQueryOptions(&c, &options)

	return &c
}
