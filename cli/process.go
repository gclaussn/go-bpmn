package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

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
		bpmnFileName  string
		timeV         map[string]string
		timeCycleV    map[string]string
		timeDurationV map[string]string

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

			timers := make(map[string]*engine.Timer)
			for bpmnElementId, v := range timeV {
				var value timeValue
				if err := value.Set(v); err != nil {
					return err
				}
				timers[bpmnElementId] = &engine.Timer{Time: time.Time(value)}
			}
			for bpmnElementId, v := range timeCycleV {
				timers[bpmnElementId] = &engine.Timer{TimeCycle: v}
			}
			for bpmnElementId, v := range timeDurationV {
				var value iso8601DurationValue
				if err := value.Set(v); err != nil {
					return err
				}
				timers[bpmnElementId] = &engine.Timer{TimeDuration: engine.ISO8601Duration(value)}
			}

			cmd.BpmnXml = string(bpmnXml)
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
	c.Flags().StringToStringVar(&cmd.ErrorCodes, "error-code", nil, "Mapping between BPMN element ID and error code")
	c.Flags().StringToStringVar(&cmd.MessageNames, "message-name", nil, "Message name that triggers the message start event")
	c.Flags().IntVar(&cmd.Parallelism, "parallelism", 0, "Maximum number of parallel process instances being executed")
	c.Flags().StringToStringVar(&cmd.SignalNames, "signal-name", nil, "Signal name that triggers the signal start event")
	c.Flags().StringToStringVar(&cmd.Tags, "tag", nil, "Tag, consisting of name and value")
	c.Flags().StringToStringVar(&timeV, "time", nil, "A point in time, when the timer start event is triggered")
	c.Flags().StringToStringVar(&timeCycleV, "time-cycle", nil, "CRON expression that specifies a cyclic timer start")
	c.Flags().StringToStringVar(&timeDurationV, "time-duration", nil, "Duration until the timer start event is triggered")
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
		criteria engine.ProcessCriteria
		options  engine.QueryOptions
	)

	c := cobra.Command{
		Use:   "query",
		Short: "Query processes",
		RunE: func(c *cobra.Command, _ []string) error {
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

	c.Flags().StringToStringVar(&criteria.Tags, "tag", nil, "Tag, consisting of name and value")

	flagQueryOptions(&c, &options)

	return &c
}
