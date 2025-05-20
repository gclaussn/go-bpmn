package cli

import (
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

			cmd.BpmnXml = string(bpmnXml)
			cmd.WorkerId = cli.workerId

			process, err := cli.engine.CreateProcess(cmd)
			if err != nil {
				return err
			}

			c.Print(process.Id)
			return nil
		},
	}

	c.Flags().StringVar(&bpmnFileName, "bpmn-file", "", "Path to a BPMN XML file")

	c.Flags().StringVar(&cmd.BpmnProcessId, "bpmn-process-id", "", "ID of the process element within the BPMN XML")
	c.Flags().IntVar(&cmd.Parallelism, "parallelism", 0, "Maximum number of parallel process instances being executed")
	c.Flags().StringToStringVar(&cmd.Tags, "tag", nil, "Tag, consisting of name and value")
	c.Flags().StringVar(&cmd.Version, "version", "", "Arbitrary process version")

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
			bpmnXml, err := cli.engine.GetBpmnXml(cmd)
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
			results, err := cli.engine.QueryWithOptions(criteria, options)
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

			for i := 0; i < len(results); i++ {
				process := results[i].(engine.Process)

				table.addRow([]string{
					strconv.Itoa(int(process.Id)),
					process.BpmnProcessId,
					process.Version,
					formatTime(process.CreatedAt),
					process.CreatedBy,
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
