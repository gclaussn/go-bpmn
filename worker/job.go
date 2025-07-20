package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
)

func NewJobError(err error, retryCount int) error {
	return jobError{
		err:        err,
		retryCount: retryCount,
		retryTimer: engine.ISO8601Duration(""),
	}
}

func NewJobErrorWithTimer(err error, retryCount int, retryTimer engine.ISO8601Duration) error {
	return jobError{
		err:        err,
		retryCount: retryCount,
		retryTimer: retryTimer,
	}
}

func newJobExecutor(worker *Worker) *jobExecutor {
	tickerCtx, tickerCancel := context.WithCancel(context.Background())

	return &jobExecutor{
		worker: worker,

		tickerCtx:    tickerCtx,
		tickerCancel: tickerCancel,
		ticker:       time.NewTicker(worker.options.JobExecutorInterval),
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

// ExecuteGeneric delegates jobs of any type. The delegation function allows to return a job-specific completion.
func (d Delegator) ExecuteGeneric(bpmnElementId string, delegation func(jc JobContext) (*engine.JobCompletion, error)) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		return delegation(jc)
	}
}

// SetSignalName delegates jobs of type [engine.JobSetSignalName].
//
// Applicable for BPMN element types:
//   - signal catch event
func (d Delegator) SetSignalName(bpmnElementId string, delegation func(jc JobContext) (string, error)) {
	d[bpmnElementId] = func(jc JobContext) (*engine.JobCompletion, error) {
		if jc.Job.Type != engine.JobSetSignalName {
			return nil, fmt.Errorf("expected job type %s, but got %s", engine.JobSetSignalName, jc.Job.Type)
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

// SetTimer delegates jobs of type [engine.JobSetTimer].
//
// Applicable for BPMN element types:
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

type JobContext struct {
	Engine engine.Engine

	Job     engine.Job
	Process engine.Process
	Element engine.Element

	worker *Worker

	processVariables Variables
	elementVariables Variables
}

func (jc JobContext) ElementVariables(names ...string) (Variables, error) {
	variables := Variables{}

	elementVariables, err := jc.Engine.GetElementVariables(engine.GetElementVariablesCmd{
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

func (jc JobContext) ProcessVariables(names ...string) (Variables, error) {
	variables := Variables{}

	processVariables, err := jc.Engine.GetProcessVariables(engine.GetProcessVariablesCmd{
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
	retryCount int
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
	worker *Worker

	tickerCtx    context.Context
	tickerCancel context.CancelFunc
	ticker       *time.Ticker
}

func (e *jobExecutor) execute() {
	go func(worker *Worker) {
		processIds := make([]int32, len(worker.processes))

		i := 0
		for processId := range worker.processes {
			processIds[i] = processId
			i++
		}

		lockJobsCmd := engine.LockJobsCmd{
			Limit:      worker.options.JobExecutorLimit,
			ProcessIds: processIds,
			WorkerId:   worker.id,
		}

		onFailure := worker.options.OnJobExecutionFailure

		for {
			select {
			case <-e.ticker.C:
				lockedJobs, err := worker.engine.LockJobs(lockJobsCmd)
				if err != nil {
					break
				}

				for i := range lockedJobs {
					_, err := worker.ExecuteJob(lockedJobs[i])
					if err != nil && onFailure != nil {
						onFailure(lockedJobs[i], err)
					}
				}
			case <-e.tickerCtx.Done():
				return
			}
		}
	}(e.worker)
}

func (e *jobExecutor) stop() {
	e.ticker.Stop()
	e.tickerCancel()
}
