package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newProcessCmd(cli *Cli) *cobra.Command {
	c := cobra.Command{
		Use:         "process",
		Short:       "Manage and query processes",
		RunE:        cli.help,
		Annotations: map[string]string{noEngineRequired: ""},
	}

	c.AddCommand(newProcessCreateCmd(cli))
	c.AddCommand(newProcessGetBpmnXmlCmd(cli))
	c.AddCommand(newProcessQueryCmd(cli))

	return &c
}

func newProcessCreateCmd(cli *Cli) *cobra.Command {
	var (
		bpmnFileName string
		errorsV      map[string]string
		escalationsV map[string]string
		messagesV    map[string]string
		signalsV     map[string]string
		tagsV        map[string]string
		timeV        map[string]string
		timeCycle    map[string]string
		timeDuration map[string]string

		cmd engine.CreateProcessCmd
	)

	c := cobra.Command{
		Use:   "create",
		Short: "Create a process",
		RunE: func(c *cobra.Command, _ []string) error {
			bpmnFile, err := os.Open(bpmnFileName)
			if err != nil {
				return fmt.Errorf("failed to open BPMN file %s: %v", bpmnFileName, err)
			}

			defer bpmnFile.Close()

			bpmnXml, err := io.ReadAll(bpmnFile)
			if err != nil {
				return fmt.Errorf("failed to read BPMN XML: %v", err)
			}

			errors := make([]engine.ErrorDefinition, 0, len(errorsV))
			for bpmnElementId, errorCode := range errorsV {
				errors = append(errors, engine.ErrorDefinition{
					BpmnElementId: bpmnElementId,
					ErrorCode:     errorCode,
				})
			}

			escalations := make([]engine.EscalationDefinition, 0, len(escalationsV))
			for bpmnElementId, escalationCode := range escalationsV {
				escalations = append(escalations, engine.EscalationDefinition{
					BpmnElementId:  bpmnElementId,
					EscalationCode: escalationCode,
				})
			}

			messages := make([]engine.MessageDefinition, 0, len(messagesV))
			for bpmnElementId, messageName := range messagesV {
				messages = append(messages, engine.MessageDefinition{
					BpmnElementId: bpmnElementId,
					MessageName:   messageName,
				})
			}

			signals := make([]engine.SignalDefinition, 0, len(signalsV))
			for bpmnElementId, signalName := range signalsV {
				signals = append(signals, engine.SignalDefinition{
					BpmnElementId: bpmnElementId,
					SignalName:    signalName,
				})
			}

			tags := make([]engine.Tag, 0, len(tagsV))
			for name, value := range tagsV {
				tags = append(tags, engine.Tag{
					Name:  name,
					Value: value,
				})
			}

			timers := make([]engine.TimerDefinition, 0, len(timeV)+len(timeCycle)+len(timeDuration))
			for bpmnElementId, v := range timeV {
				var value timeValue
				if err := value.Set(v); err != nil {
					return err
				}
				timers = append(timers, engine.TimerDefinition{
					BpmnElementId: bpmnElementId,
					Timer:         &engine.Timer{Time: value.Time()},
				})
			}
			for bpmnElementId, v := range timeCycle {
				timers = append(timers, engine.TimerDefinition{
					BpmnElementId: bpmnElementId,
					Timer:         &engine.Timer{TimeCycle: v},
				})
			}
			for bpmnElementId, v := range timeDuration {
				var value iso8601DurationValue
				if err := value.Set(v); err != nil {
					return err
				}
				timers = append(timers, engine.TimerDefinition{
					BpmnElementId: bpmnElementId,
					Timer:         &engine.Timer{TimeDuration: engine.ISO8601Duration(value)},
				})
			}

			cmd.BpmnXml = string(bpmnXml)
			cmd.Errors = errors
			cmd.Escalations = escalations
			cmd.Messages = messages
			cmd.Signals = signals
			cmd.Tags = tags
			cmd.Timers = timers
			cmd.WorkerId = cli.workerId

			process, err := cli.e.CreateProcess(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Println(process.Id)
			return nil
		},
	}

	c.Flags().StringVar(&bpmnFileName, "bpmn-file", "", "Path to a BPMN XML file")

	c.Flags().StringVar(&cmd.BpmnProcessId, "bpmn-process-id", "", "ID of the process element within the BPMN XML")
	c.Flags().StringToStringVar(&errorsV, "error", nil, "Mapping between BPMN element ID and error code")
	c.Flags().StringToStringVar(&escalationsV, "escalation", nil, "Mapping between BPMN element ID and escalation code")
	c.Flags().StringToStringVar(&messagesV, "message", nil, "Mapping between BPMN element ID and message name")
	c.Flags().IntVar(&cmd.Parallelism, "parallelism", 0, "Maximum number of parallel process instances being executed")
	c.Flags().StringToStringVar(&signalsV, "signal", nil, "Mapping between BPMN element ID and signal name")
	c.Flags().StringToStringVar(&tagsV, "tag", nil, "Tag, consisting of name and value")
	c.Flags().StringToStringVar(&timeV, "time", nil, "A point in time, when the timer event is triggered")
	c.Flags().StringToStringVar(&timeCycle, "time-cycle", nil, "CRON expression that specifies a cyclic timer")
	c.Flags().StringToStringVar(&timeDuration, "time-duration", nil, "Duration until the timer event is triggered")
	c.Flags().StringVar(&cmd.Version, "version", "", "Any process version")

	c.MarkFlagRequired("bpmn-process-id")
	c.MarkFlagRequired("bpmn-xml-file")
	c.MarkFlagRequired("version")

	c.MarkFlagFilename("bpmn-file", ".bpmn", ".bpmn20.xml", ".xml")

	return &c
}

func newProcessGetBpmnXmlCmd(cli *Cli) *cobra.Command {
	var cmd engine.GetBpmnXmlCmd

	c := cobra.Command{
		Use:   "get-bpmn-xml",
		Short: "Get BPMN XML",
		RunE: func(c *cobra.Command, _ []string) error {
			bpmnXml, err := cli.e.GetBpmnXml(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(bpmnXml)
			return nil
		},
	}

	c.Flags().Int32Var(&cmd.ProcessId, "id", 0, "Process ID")

	c.MarkFlagRequired("id")

	return &c
}

func newProcessQueryCmd(cli *Cli) *cobra.Command {
	var (
		tagsV map[string]string

		criteria engine.ProcessCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query processes",
		RunE: func(c *cobra.Command, _ []string) error {
			tags := make([]engine.Tag, 0, len(tagsV))
			for name, value := range tagsV {
				tags = append(tags, engine.Tag{
					Name:  name,
					Value: value,
				})
			}

			criteria.Tags = tags

			q := cli.e.CreateQuery()
			q.SetOptions(options)

			results, err := q.QueryProcesses(context.Background(), criteria)
			if err != nil {
				return err
			}

			table := newTable([]string{
				"ID",
				"BPMN PROCESS ID",
				"VERSION",
				"CREATED AT",
				"CREATED BY",
			})

			for _, result := range results {
				table.addRow([]string{
					strconv.Itoa(int(result.Id)),
					result.BpmnProcessId,
					result.Version,
					formatTime(result.CreatedAt),
					result.CreatedBy,
				})
			}

			c.Print(table.format())
			return nil
		},
	}

	c.Flags().StringToStringVar(&tagsV, "tag", nil, "Tag, consisting of name and value")

	flagQueryOptions(&c, &options)

	return &c
}
