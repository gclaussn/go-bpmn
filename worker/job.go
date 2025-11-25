package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
)

func NewJobError(err error, retryLimit int) error {
	return jobError{
		err:        err,
		retryLimit: retryLimit,
		retryTimer: engine.ISO8601Duration(""),
	}
}

func NewJobErrorWithTimer(err error, retryLimit int, retryTimer engine.ISO8601Duration) error {
	return jobError{
		err:        err,
		retryLimit: retryLimit,
		retryTimer: retryTimer,
	}
}

func newJobExecutor(w *Worker) *jobExecutor {
	tickerCtx, tickerCancel := context.WithCancel(context.Background())

	return &jobExecutor{
		w: w,

		tickerCtx:    tickerCtx,
		tickerCancel: tickerCancel,
		ticker:       time.NewTicker(w.options.JobExecutorInterval),
	}
}

type Delegator map[string]func(JobContext) (*engine.JobCompletion, error)

func (d Delegator) EvaluateExclusiveGateway(bpmnElementId string, delegation func(jc JobContext) (string, error)) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if jc.Job.Type != engine.JobEvaluateExclusiveGateway {
			return nil, fmt.Errorf("expected job type %s, but got %s", engine.JobEvaluateExclusiveGateway, jc.Job.Type)
		}

		bpmnElementId, err := delegation(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			ExclusiveGatewayDecision: bpmnElementId,
		}, nil
	}
}

func (d Delegator) EvaluateInclusiveGateway(bpmnElementId string, delegation func(jc JobContext) ([]string, error)) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if jc.Job.Type != engine.JobEvaluateInclusiveGateway {
			return nil, fmt.Errorf("expected job type %s, but got %s", engine.JobEvaluateInclusiveGateway, jc.Job.Type)
		}

		bpmnElementIds, err := delegation(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			InclusiveGatewayDecision: bpmnElementIds,
		}, nil
	}
}

func (d Delegator) Execute(bpmnElementId string, delegation func(jc JobContext) error) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if jc.Job.Type != engine.JobExecute {
			return nil, fmt.Errorf("expected job type %s, but got %s", engine.JobExecute, jc.Job.Type)
		}

		if err := delegation(jc); err != nil {
			return nil, err
		}

		return nil, nil
	}
}

// ExecuteAny delegates jobs of any type. The delegation function allows to return a job-specific completion.
func (d Delegator) ExecuteAny(bpmnElementId string, delegation func(jc JobContext) (*engine.JobCompletion, error)) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		return delegation(jc)
	}
}

// SetErrorCode delegates jobs of type [engine.JobSetErrorCode].
//
// Applicable for BPMN element types:
//   - error boundary event
func (d Delegator) SetErrorCode(bpmnElementId string, delegation func(jc JobContext) (string, error)) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if jc.Job.Type != engine.JobSetErrorCode {
			return nil, fmt.Errorf("expected job type %s, but got %s", engine.JobSetErrorCode, jc.Job.Type)
		}

		errorCode, err := delegation(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			ErrorCode: errorCode,
		}, nil
	}
}

// SetEscalationCode delegates jobs of type [engine.JobSetEscalationCode].
//
// Applicable for BPMN element types:
//   - escalation boundary event
func (d Delegator) SetEscalationCode(bpmnElementId string, delegation func(jc JobContext) (string, error)) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if jc.Job.Type != engine.JobSetEscalationCode {
			return nil, fmt.Errorf("expected job type %s, but got %s", engine.JobSetEscalationCode, jc.Job.Type)
		}

		esclationCode, err := delegation(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			EscalationCode: esclationCode,
		}, nil
	}
}

// SetTimer delegates jobs of type [engine.JobSetTimer].
//
// Applicable for BPMN element types:
//   - timer boundary event
//   - timer catch event
func (d Delegator) SetTimer(bpmnElementId string, delegation func(jc JobContext) (engine.Timer, error)) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if jc.Job.Type != engine.JobSetTimer {
			return nil, fmt.Errorf("expected job type %s, but got %s", engine.JobSetTimer, jc.Job.Type)
		}

		timer, err := delegation(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			Timer: &timer,
		}, nil
	}
}

// SubscribeMessage delegates jobs of type [engine.JobSubscribeMessage].
//
// A delegation function must return message name and correlation key.
//
// Applicable for BPMN element types:
//   - message catch event
func (d Delegator) SubscribeMessage(bpmnElementId string, delegation func(jc JobContext) (string, string, error)) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if jc.Job.Type != engine.JobSubscribeMessage {
			return nil, fmt.Errorf("expected job type %s, but got %s", engine.JobSubscribeMessage, jc.Job.Type)
		}

		messageName, messageCorrelationKey, err := delegation(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			MessageName:           messageName,
			MessageCorrelationKey: messageCorrelationKey,
		}, nil
	}
}

// SubscribeSignal delegates jobs of type [engine.JobSubscribeSignal].
//
// Applicable for BPMN element types:
//   - signal catch event
func (d Delegator) SubscribeSignal(bpmnElementId string, delegation func(jc JobContext) (string, error)) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if jc.Job.Type != engine.JobSubscribeSignal {
			return nil, fmt.Errorf("expected job type %s, but got %s", engine.JobSubscribeSignal, jc.Job.Type)
		}

		signalName, err := delegation(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			SignalName: signalName,
		}, nil
	}
}

type JobContext struct {
	Job     engine.Job
	Process engine.Process
	Element engine.Element

	w   *Worker
	ctx context.Context

	processVariables Variables
	elementVariables Variables
}

func (jc JobContext) Context() context.Context {
	return jc.ctx
}

func (jc JobContext) ElementVariables(names ...string) (Variables, error) {
	variables := Variables{}

	elementVariables, err := jc.w.e.GetElementVariables(jc.ctx, engine.GetElementVariablesCmd{
		Partition:         jc.Job.Partition,
		ElementInstanceId: jc.Job.ElementInstanceId,

		Names: names,
	})
	if err != nil {
		return variables, err
	}

	for variableName, data := range elementVariables {
		variables.SetVariable(Variable{
			Encoding:    data.Encoding,
			IsEncrypted: data.IsEncrypted,
			Name:        variableName,
			Value:       data.Value,
		})
	}

	return variables, nil
}

func (jc JobContext) Engine() engine.Engine {
	return jc.w.e
}

func (jc JobContext) ProcessVariables(names ...string) (Variables, error) {
	variables := Variables{}

	processVariables, err := jc.w.e.GetProcessVariables(jc.ctx, engine.GetProcessVariablesCmd{
		ProcessInstanceId: jc.Job.ProcessInstanceId,
		Partition:         jc.Job.Partition,

		Names: names,
	})
	if err != nil {
		return variables, err
	}

	for variableName, data := range processVariables {
		variables.SetVariable(Variable{
			Encoding:    data.Encoding,
			IsEncrypted: data.IsEncrypted,
			Name:        variableName,
			Value:       data.Value,
		})
	}

	return variables, nil
}

func (jc JobContext) SetElementVariables(elementVariables Variables) {
	for variableName, variable := range elementVariables {
		jc.elementVariables[variableName] = variable
	}
}

func (jc JobContext) SetProcessVariables(processVariables Variables) {
	for variableName, variable := range processVariables {
		jc.processVariables[variableName] = variable
	}
}

type jobError struct {
	err        error
	retryLimit int
	retryTimer engine.ISO8601Duration
}

func (e jobError) Error() string {
	if e.err != nil {
		return e.err.Error()
	} else {
		return ""
	}
}

func (e jobError) Unwrap() error {
	return e.err
}

type jobExecutor struct {
	w *Worker

	tickerCtx    context.Context
	tickerCancel context.CancelFunc
	ticker       *time.Ticker
}

func (e *jobExecutor) execute() {
	go func(w *Worker) {
		processIds := make([]int32, len(w.processes))

		i := 0
		for processId := range w.processes {
			processIds[i] = processId
			i++
		}

		lockJobsCmd := engine.LockJobsCmd{
			Limit:      w.options.JobExecutorLimit,
			ProcessIds: processIds,
			WorkerId:   w.id,
		}

		onFailure := w.options.OnJobExecutionFailure

		for {
			select {
			case <-e.ticker.C:
				lockedJobs, err := w.e.LockJobs(context.Background(), lockJobsCmd)
				if err != nil {
					break
				}

				for i := range lockedJobs {
					_, err := w.ExecuteJob(context.Background(), lockedJobs[i])
					if err != nil && onFailure != nil {
						onFailure(lockedJobs[i], err)
					}
				}
			case <-e.tickerCtx.Done():
				return
			}
		}
	}(e.w)
}

func (e *jobExecutor) stop() {
	e.ticker.Stop()
	e.tickerCancel()
}
