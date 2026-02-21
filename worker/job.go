package worker

import (
	"context"
	"fmt"
	"time"

	"maps"

	"github.com/gclaussn/go-bpmn/engine"
)

func NewJobError(err error) error {
	return jobError{err: err}
}

func NewJobErrorWithRetry(err error, retryLimit int) error {
	return jobError{
		err:        err,
		retryLimit: retryLimit,
		retryTimer: engine.ISO8601Duration(""),
	}
}

func NewJobErrorWithRetryTimer(err error, retryLimit int, retryTimer engine.ISO8601Duration) error {
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

type JobMux map[string]func(JobContext) (*engine.JobCompletion, error)

// EvaluateExclusiveGateway handles jobs of type [engine.JobEvaluateExclusiveGateway].
//
// Applicable for BPMN elements of type exclusive gateway.
func (m JobMux) EvaluateExclusiveGateway(bpmnElementId string, jobHandler func(jc JobContext) (string, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobEvaluateExclusiveGateway); err != nil {
			return nil, err
		}

		bpmnElementId, err := jobHandler(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			ExclusiveGatewayDecision: bpmnElementId,
		}, nil
	}
}

// EvaluateInclusiveGateway handles jobs of type [engine.JobEvaluateInclusiveGateway].
//
// Applicable for BPMN elements of type inclusive gateway.
func (m JobMux) EvaluateInclusiveGateway(bpmnElementId string, jobHandler func(jc JobContext) ([]string, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobEvaluateInclusiveGateway); err != nil {
			return nil, err
		}

		bpmnElementIds, err := jobHandler(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			InclusiveGatewayDecision: bpmnElementIds,
		}, nil
	}
}

// Execute handles jobs of type [engine.JobExecute].
//
// Applicable for BPMN element types:
//   - business rule task
//   - script task
//   - send task
//   - service task
//   - message end event
//   - message throw event
func (m JobMux) Execute(bpmnElementId string, jobHandler func(jc JobContext) error) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobExecute); err != nil {
			return nil, err
		}

		if err := jobHandler(jc); err != nil {
			return nil, err
		}

		return nil, nil
	}
}

// ExecuteAny handles jobs of any type. The job handler function allows to return a job-specific completion.
func (m JobMux) ExecuteAny(bpmnElementId string, jobHandler func(jc JobContext) (*engine.JobCompletion, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		return jobHandler(jc)
	}
}

// SetErrorCode handles jobs of type [engine.JobSetErrorCode].
//
// Applicable for BPMN element types:
//   - error boundary event
func (m JobMux) SetErrorCode(bpmnElementId string, jobHandler func(jc JobContext) (string, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobSetErrorCode); err != nil {
			return nil, err
		}

		errorCode, err := jobHandler(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			ErrorCode: errorCode,
		}, nil
	}
}

// SetEscalationCode handles jobs of type [engine.JobSetEscalationCode].
//
// Applicable for BPMN element types:
//   - escalation boundary event
func (m JobMux) SetEscalationCode(bpmnElementId string, jobHandler func(jc JobContext) (string, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobSetEscalationCode); err != nil {
			return nil, err
		}

		esclationCode, err := jobHandler(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			EscalationCode: esclationCode,
		}, nil
	}
}

// SetMessageCorrelationKey handles jobs of type [engine.JobSetMessageCorrelationKey].
//
// Applicable for BPMN element types:
//   - message boundary event
//   - message catch event
func (m JobMux) SetMessageCorrelationKey(bpmnElementId string, jobHandler func(jc JobContext) (string, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobSetMessageCorrelationKey); err != nil {
			return nil, err
		}

		messageCorrelationKey, err := jobHandler(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			MessageCorrelationKey: messageCorrelationKey,
		}, nil
	}
}

// SetSignalName handles jobs of type [engine.JobSetSignalName].
//
// Applicable for BPMN element types:
//   - signal end event
//   - signal throw event
func (m JobMux) SetSignalName(bpmnElementId string, jobHandler func(jc JobContext) (string, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobSetSignalName); err != nil {
			return nil, err
		}

		signalName, err := jobHandler(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			SignalName: signalName,
		}, nil
	}
}

// SetTimer handles jobs of type [engine.JobSetTimer].
//
// Applicable for BPMN element types:
//   - timer boundary event
//   - timer catch event
func (m JobMux) SetTimer(bpmnElementId string, jobHandler func(jc JobContext) (engine.Timer, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobSetTimer); err != nil {
			return nil, err
		}

		timer, err := jobHandler(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			Timer: &timer,
		}, nil
	}
}

// SubscribeMessage handles jobs of type [engine.JobSubscribeMessage].
//
// A job handler function must return message name and correlation key.
//
// Applicable for BPMN element types:
//   - message boundary event
//   - message catch event
func (m JobMux) SubscribeMessage(bpmnElementId string, jobHandler func(jc JobContext) (string, string, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobSubscribeMessage); err != nil {
			return nil, err
		}

		messageName, messageCorrelationKey, err := jobHandler(jc)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			MessageName:           messageName,
			MessageCorrelationKey: messageCorrelationKey,
		}, nil
	}
}

// SubscribeSignal handles jobs of type [engine.JobSubscribeSignal].
//
// Applicable for BPMN element types:
//   - signal boundary event
//   - signal catch event
func (m JobMux) SubscribeSignal(bpmnElementId string, jobHandler func(jc JobContext) (string, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobSubscribeSignal); err != nil {
			return nil, err
		}

		signalName, err := jobHandler(jc)
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
	elementVariables, err := jc.w.e.GetElementVariables(jc.ctx, engine.GetElementVariablesCmd{
		Partition:         jc.Job.Partition,
		ElementInstanceId: jc.Job.ElementInstanceId,

		Names: names,
	})
	if err != nil {
		return nil, err
	}

	variables := make(Variables, len(elementVariables))
	for _, variable := range elementVariables {
		variables.Set(Variable{
			Encoding:    variable.Data.Encoding,
			IsEncrypted: variable.Data.IsEncrypted,
			Name:        variable.Name,
			Value:       variable.Data.Value,
		})
	}

	return variables, nil
}

func (jc JobContext) Engine() engine.Engine {
	return jc.w.e
}

func (jc JobContext) ProcessVariables(names ...string) (Variables, error) {
	processVariables, err := jc.w.e.GetProcessVariables(jc.ctx, engine.GetProcessVariablesCmd{
		ProcessInstanceId: jc.Job.ProcessInstanceId,
		Partition:         jc.Job.Partition,

		Names: names,
	})
	if err != nil {
		return nil, err
	}

	variables := make(Variables, len(processVariables))
	for _, variable := range processVariables {
		variables.Set(Variable{
			Encoding:    variable.Data.Encoding,
			IsEncrypted: variable.Data.IsEncrypted,
			Name:        variable.Name,
			Value:       variable.Data.Value,
		})
	}

	return variables, nil
}

func (jc JobContext) SetElementVariables(elementVariables Variables) {
	maps.Copy(jc.elementVariables, elementVariables)
}

func (jc JobContext) SetProcessVariables(processVariables Variables) {
	maps.Copy(jc.processVariables, processVariables)
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
		processIds := make([]int32, len(w.processHandles))

		i := 0
		for processId := range w.processHandles {
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

func ensureJobType(actual engine.JobType, expected engine.JobType) error {
	if actual != expected {
		return fmt.Errorf("expected job type %s, but got %s", expected, actual)
	}
	return nil
}
