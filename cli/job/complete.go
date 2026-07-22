package job

import (
	"context"

	"github.com/gclaussn/go-bpmn/cli/common"
	"github.com/gclaussn/go-bpmn/engine"
	"github.com/spf13/cobra"
)

func newCallProcessCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables       common.ElementVariables
		calledProcess          engine.CalledProcess
		calledProcessTagMap    map[string]string
		calledProcessVariables common.ProcessVariables
		completion             engine.JobCompletion
		processVariables       common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "call-process",
		Short:       "Complete a job of type CALL_PROCESS",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			calledProcessVariables, err := calledProcessVariables.Variables()
			if err != nil {
				return err
			}

			calledProcess.Tags = common.Tags(calledProcessTagMap)
			calledProcess.Variables = calledProcessVariables

			completion.CalledProcess = &calledProcess

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringVar(&calledProcess.BpmnProcessId, "bpmn-process-id", "", "BPMN ID of the process to call")
	c.Flags().StringVar(&calledProcess.CorrelationKey, "correlation-key", "", "Optional key, used to correlate the child process instance with a business entity")
	c.Flags().StringToStringVar(&calledProcessTagMap, "tag", nil, "Tags to apply to the child process instance")
	c.Flags().StringToStringVar(&calledProcessVariables.EncodingMap, "variable-encoding", nil, "Variables to set at child process instance scope\nEncoding of the value - e.g. `json`")
	c.Flags().StringToStringVar(&calledProcessVariables.EncryptedMap, "variable-encrypted", nil, "Variables to set at child process instance scope\nDetermines if a value is encrypted before it is stored")
	c.Flags().StringToStringVar(&calledProcessVariables.ValueMap, "variable", nil, "Variables to set at child process instance scope\nData value, encoded as a string")
	c.Flags().StringVar(&calledProcess.Version, "version", "", "Version of the process to call")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newEvaluateExclusiveGatewayCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		completion       engine.JobCompletion
		processVariables common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "evaluate-exclusive-gateway",
		Short:       "Complete a job of type EVALUATE_EXCLUSIVE_GATEWAY",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringVar(&completion.ExclusiveGatewayDecision, "decision", "", "Evaluated BPMN element ID to continue with after the exclusive gateway")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("decision")

	return &c
}

func newEvaluateInclusiveGatewayCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		completion       engine.JobCompletion
		processVariables common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "evaluate-inclusive-gateway",
		Short:       "Complete a job of type EVALUATE_INCLUSIVE_GATEWAY",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringSliceVar(&completion.InclusiveGatewayDecision, "decision", nil, "Evaluated BPMN element IDs to continue with after the inclusive gateway")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("decision")

	return &c
}

func newExecuteCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		completion       engine.JobCompletion
		processVariables common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "execute",
		Short:       "Complete a job of type EXECUTE",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.Flags().StringVar(&completion.ErrorCode, "error-code", "", "Code of a BPMN error to trigger")
	c.Flags().StringVar(&completion.EscalationCode, "escalation-code", "", "Code of a BPMN escalation to trigger")

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newPassVariablesCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		processVariables common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "pass-variables",
		Short:       "Complete a job of type PASS_VARIABLES",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newSetErrorCodeCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		completion       engine.JobCompletion
		processVariables common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "set-error-code",
		Short:       "Complete a job of type SET_ERROR_CODE",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringVar(&completion.ErrorCode, "code", "", "Code of a BPMN error to specify")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("code")

	return &c
}

func newSetEscalationCodeCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		completion       engine.JobCompletion
		processVariables common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "set-escalation-code",
		Short:       "Complete a job of type SET_ESCALATION_CODE",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringVar(&completion.EscalationCode, "code", "", "Code of a BPMN escalation to specify")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("code")

	return &c
}

func newSetMessageCorrelationKeyCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		completion       engine.JobCompletion
		processVariables common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "set-message-correlation-key",
		Short:       "Complete a job of type SET_MESSAGE_CORRELATION_KEY",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringVar(&completion.MessageCorrelationKey, "correlation-key", "", "Key, used to correlate a message subscription with a message")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("correlation-key")

	return &c
}

func newSetSignalNameCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		completion       engine.JobCompletion
		processVariables common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "set-signal-name",
		Short:       "Complete a job of type SET_SIGNAL_NAME",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringVar(&completion.SignalName, "name", "", "Name of the signal to send")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("name")

	return &c
}

func newSetTimerCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		completion       engine.JobCompletion
		processVariables common.ProcessVariables
		timer            common.Timer

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "set-timer",
		Short:       "Complete a job of type SET_TIMER",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			completion.Timer = timer.Timer()

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().Var(&timer.Time, "time", "A point in time, when the timer event is triggered")
	c.Flags().StringVar(&timer.TimeCycle, "time-cycle", "", "CRON expression that specifies a cyclic trigger")
	c.Flags().Var(&timer.TimeDuration, "time-duration", "Duration until the timer event is triggered")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")

	return &c
}

func newSubscribeMessageCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		completion       engine.JobCompletion
		processVariables common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "subscribe-message",
		Short:       "Complete a job of type SUBSCRIBE_MESSAGE",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringVar(&completion.MessageCorrelationKey, "correlation-key", "", "Key, used to correlate a message subscription with a message")
	c.Flags().StringVar(&completion.MessageName, "name", "", "Name of the message to subscribe to")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("correlation-key")
	c.MarkFlagRequired("name")

	return &c
}

func newSubscribeSignalCmd() *cobra.Command {
	var (
		partition common.Partition

		elementVariables common.ElementVariables
		completion       engine.JobCompletion
		processVariables common.ProcessVariables

		cmd engine.CompleteJobCmd
	)

	c := cobra.Command{
		Use:         "subscribe-signal",
		Short:       "Complete a job of type SUBSCRIBE_SIGNAL",
		Annotations: common.AnnotationEngineMap,
		RunE: func(c *cobra.Command, _ []string) error {
			elementVariables, err := elementVariables.Variables()
			if err != nil {
				return err
			}

			processVariables, err := processVariables.Variables()
			if err != nil {
				return err
			}

			e, workerId := common.GetEngineAndWorkerId(c)

			cmd.Partition = engine.Partition(partition)
			cmd.Completion = &completion
			cmd.ElementVariables = elementVariables
			cmd.ProcessVariables = processVariables
			cmd.WorkerId = workerId

			job, err := e.CompleteJob(context.Background(), cmd)
			if err != nil {
				return err
			}

			c.Print(job)
			return nil
		},
	}

	c.Flags().VarP(&partition, "partition", "p", "Job partition")
	c.Flags().Int32VarP(&cmd.Id, "id", "i", 0, "Job ID")

	c.Flags().StringVar(&completion.SignalName, "name", "", "Name of the signal to subscribe to")

	flagCompleteVariables(&c, &elementVariables, &processVariables)

	c.MarkFlagRequired("partition")
	c.MarkFlagRequired("id")
	c.MarkFlagRequired("name")

	return &c
}

func flagCompleteVariables(cmd *cobra.Command, elementVariables *common.ElementVariables, processVariables *common.ProcessVariables) {
	cmd.Flags().StringToStringVar(&elementVariables.BpmnElementIdMap, "ev-bpmn-element-id", nil, "Variable to set or delete at element instance scope\nBPMN element ID to determine the variable's scope")
	cmd.Flags().StringToStringVar(&elementVariables.EncodingMap, "ev-encoding", nil, "Variable to set or delete at element instance scope\nEncoding of the value - e.g. `json`")
	cmd.Flags().StringToStringVar(&elementVariables.EncryptedMap, "ev-encrypted", nil, "Variable to set or delete at element instance scope\nDetermines if a value is encrypted before it is stored")
	cmd.Flags().StringToStringVar(&elementVariables.ValueMap, "ev", nil, "Variable to set or delete at element instance scope\nData value, encoded as a string")

	cmd.Flags().StringToStringVar(&processVariables.EncodingMap, "pv-encoding", nil, "Variable to set or delete at process instance scope\nEncoding of the value - e.g. `json`")
	cmd.Flags().StringToStringVar(&processVariables.EncryptedMap, "pv-encrypted", nil, "Variable to set or delete at process instance scope\nDetermines if a value is encrypted before it is stored")
	cmd.Flags().StringToStringVar(&processVariables.ValueMap, "pv", nil, "Variable to set or delete at process instance scope\nData value, encoded as a string")
}
