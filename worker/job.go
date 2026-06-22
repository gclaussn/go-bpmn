package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// CallProcess handles jobs of type [engine.JobCallProcess].
//
// Applicable for BPMN elements of type call activity.
func (m JobMux) CallProcess(bpmnElementId string, jobHandler func(jc JobContext) (CalledProcess, error)) {
	m[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobCallProcess); err != nil {
			return nil, err
		}

		calledProcess, err := jobHandler(jc)
		if err != nil {
			return nil, err
		}

		variables, err := jc.w.encodeProcessVariables(calledProcess.Variables.variables)
		if err != nil {
			return nil, err
		}

		return &engine.JobCompletion{
			CalledProcess: &engine.CalledProcess{
				BpmnProcessId:  calledProcess.BpmnProcessId,
				CorrelationKey: calledProcess.CorrelationKey,
				Tags:           calledProcess.Tags,
				Variables:      variables,
				Version:        calledProcess.Version,
			},
		}, nil
	}
}

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

// PassVariables handles jobs of type [engine.JobPassVariables].
//
// Applicable for BPMN elements of type call activity.
func (m JobMux) PassVariables(bpmnElementId string, jobHandler func(jc JobContext, subProcessInstance engine.ProcessInstance) error) {
	m[bpmnElementId+":passVariables"] = func(jc JobContext) (*engine.JobCompletion, error) {
		if err := ensureJobType(jc.Job.Type, engine.JobPassVariables); err != nil {
			return nil, err
		}

		query := jc.w.e.CreateQuery()

		elementInstances, err := query.QueryElementInstances(jc.ctx, engine.ElementInstanceCriteria{
			Partition: jc.Job.Partition,
			ParentId:  jc.Job.ElementInstanceId,
		})
		if err != nil {
			return nil, err
		}

		if len(elementInstances) == 0 {
			return nil, errors.New("failed to find sub element instance")
		}

		processInstances, err := query.QueryProcessInstances(jc.ctx, engine.ProcessInstanceCriteria{
			Partition: elementInstances[0].Partition,
			Id:        elementInstances[0].ProcessInstanceId,
		})
		if err != nil {
			return nil, err
		}

		if len(processInstances) == 0 {
			return nil, errors.New("failed to find sub process instance")
		}

		if err := jobHandler(jc, processInstances[0]); err != nil {
			return nil, err
		}

		return nil, nil
	}
}

// SetErrorCode handles jobs of type [engine.JobSetErrorCode].
//
// Applicable for BPMN element types:
//   - error boundary event
//   - error end event
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
//   - escalation end event
//   - escalation boundary event
//   - escalation throw event
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

	newProcessVariables *ProcessVariables
	newElementVariables *ElementVariables
}

func (jc JobContext) Context() context.Context {
	return jc.ctx
}

// ElementVariables gets variables of a job's element instance as well as direct and indirect parent element instances.
//
// names defines which variables to get. If empty, all variables are returned.
func (jc JobContext) ElementVariables(names ...string) (ElementVariables, error) {
	elementVariables, err := jc.w.e.GetElementVariables(jc.ctx, engine.GetElementVariablesCmd{
		Partition:         jc.Job.Partition,
		ElementInstanceId: jc.Job.ElementInstanceId,

		Names: names,
	})
	if err != nil {
		return ElementVariables{}, err
	}

	variables := make([]ElementVariable, len(elementVariables))
	for i, variable := range elementVariables {
		variables[i] = ElementVariable{
			BpmnElementId: variable.BpmnElementId,
			Encoding:      variable.Data.Encoding,
			IsEncrypted:   variable.Data.IsEncrypted,
			IsEncoded:     true,
			Name:          variable.Name,
			Value:         variable.Data.Value,
		}
	}

	return ElementVariables{
		bpmnElementId: jc.Job.BpmnElementId,
		variables:     variables,
	}, nil
}

func (jc JobContext) Engine() engine.Engine {
	return jc.w.e
}

// NewElementVariables provides a pointer to element variables, set on job completion.
//
// NewElementVariables is used to set or delete variables at element instance scope.
func (jc JobContext) NewElementVariables() *ElementVariables {
	return jc.newElementVariables
}

// NewProcessVariables provides a pointer to process variables, set on job completion.
//
// NewProcessVariables is used to set or delete variables at process instance scope.
func (jc JobContext) NewProcessVariables() *ProcessVariables {
	return jc.newProcessVariables
}

// ProcessVariables gets variables of a job's process instance.
//
// names defines which variables to get. If empty, all variables are returned.
func (jc JobContext) ProcessVariables(names ...string) (ProcessVariables, error) {
	processVariables, err := jc.w.e.GetProcessVariables(jc.ctx, engine.GetProcessVariablesCmd{
		Partition:         jc.Job.Partition,
		ProcessInstanceId: jc.Job.ProcessInstanceId,

		Names: names,
	})
	if err != nil {
		return ProcessVariables{}, err
	}

	variables := make([]ProcessVariable, len(processVariables))
	for i, variable := range processVariables {
		variables[i] = ProcessVariable{
			Encoding:    variable.Data.Encoding,
			IsEncrypted: variable.Data.IsEncrypted,
			IsEncoded:   true,
			Name:        variable.Name,
			Value:       variable.Data.Value,
		}
	}

	return ProcessVariables{variables: variables}, nil
}

// SubProcessVariables gets process variables of a sub process instance.
//
// names defines which variables to get. If empty, all variables are returned.
func (jc JobContext) SubProcessVariables(subProcessInstance engine.ProcessInstance, names ...string) (ProcessVariables, error) {
	processVariables, err := jc.w.e.GetProcessVariables(jc.ctx, engine.GetProcessVariablesCmd{
		Partition:         subProcessInstance.Partition,
		ProcessInstanceId: subProcessInstance.Id,

		Names: names,
	})
	if err != nil {
		return ProcessVariables{}, err
	}

	variables := make([]ProcessVariable, len(processVariables))
	for i, variable := range processVariables {
		variables[i] = ProcessVariable{
			Encoding:    variable.Data.Encoding,
			IsEncrypted: variable.Data.IsEncrypted,
			IsEncoded:   true,
			Name:        variable.Name,
			Value:       variable.Data.Value,
		}
	}

	return ProcessVariables{variables: variables}, nil
}

// CalledProcess is used to create a child process instance.
type CalledProcess struct {
	// BPMN ID of the process to call.
	BpmnProcessId string
	// Optional key, used to correlate the child process instance with a business entity.
	CorrelationKey string
	// Tags to apply to the child process instance.
	Tags []engine.Tag
	// Variables to set at child process instance scope.
	Variables ProcessVariables
	// Version of the process to call.
	Version string
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
