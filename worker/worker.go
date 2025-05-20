package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
)

const (
	DefaultEncoding = "json"           // Default variable enconding.
	DefaultWorkerId = "default-worker" // Default ID of a worker, used when no specific ID is provided via [Options].
)

func New(e engine.Engine, customizers ...func(*Options)) (*Worker, error) {
	if e == nil {
		return nil, errors.New("engine is nil")
	}

	options := NewOptions()
	for _, customizer := range customizers {
		customizer(&options)
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	worker := Worker{
		defaultDecoder: options.Decoders[options.DefaultEncoding],
		defaultEncoder: options.Encoders[options.DefaultEncoding],
		engine:         e,
		id:             options.WorkerId,
		options:        options,
		processes:      make(map[int32]Process),
	}

	return &worker, nil
}

func NewOptions() Options {
	return Options{
		Decoders: map[string]Decoder{
			DefaultEncoding: jsonDecoder{},
		},
		DefaultEncoding: DefaultEncoding,
		Encoders: map[string]Encoder{
			DefaultEncoding: jsonEncoder{},
		},
		JobExecutorInterval: 60 * time.Second,
		JobExecutorLimit:    10,
		WorkerId:            DefaultWorkerId,
	}
}

type Delegate interface {
	CreateProcessCmd() (engine.CreateProcessCmd, error)
	Delegate(delegator Delegator) error
}

type Decoder interface {
	Decode(string, any) error
}

type Encoder interface {
	Encode(any) (string, error)
}

type Options struct {
	Decoders            map[string]Decoder // Mapping between encodings and decoders.
	DefaultEncoding     string             // Default encoding to use, when a variable does not specifiy an encoding.
	Encoders            map[string]Encoder // Mapping between encodings and encoders.
	JobExecutorInterval time.Duration      // Interval between execution of due jobs.
	JobExecutorLimit    int                // Maximum number of jobs to lock and execute at once by the worker.
	WorkerId            string             // Worker ID.

	OnJobExecutionFailure func(engine.Job, error) // Called when the worker failed to execute a locked job.
}

func (o Options) Validate() error {
	if _, ok := o.Decoders[o.DefaultEncoding]; !ok {
		return errors.New("default decoder is nil")
	}
	if _, ok := o.Encoders[o.DefaultEncoding]; !ok {
		return errors.New("default encoder is nil")
	}
	if strings.TrimSpace(o.WorkerId) == "" {
		return errors.New("worker ID must not be empty or blank")
	}

	return nil
}

type Process struct {
	Process  engine.Process
	Elements map[string]engine.Element

	worker    *Worker
	delegate  Delegate
	delegator Delegator
}

func (p Process) CreateProcessInstanceCmd() engine.CreateProcessInstanceCmd {
	return engine.CreateProcessInstanceCmd{
		BpmnProcessId: p.Process.BpmnProcessId,
		Version:       p.Process.Version,
		WorkerId:      p.worker.id,
	}
}

func (p Process) CreateProcessInstance(variables Variables) (engine.ProcessInstance, error) {
	processVariables, err := p.worker.encodeVariables(variables)
	if err != nil {
		return engine.ProcessInstance{}, nil
	}

	return p.worker.engine.CreateProcessInstance(engine.CreateProcessInstanceCmd{
		BpmnProcessId: p.Process.BpmnProcessId,
		Version:       p.Process.Version,
		Variables:     processVariables,
		WorkerId:      p.worker.id,
	})
}

type Worker struct {
	defaultDecoder Decoder
	defaultEncoder Encoder
	engine         engine.Engine
	id             string
	jobExecutor    *jobExecutor
	options        Options
	processes      map[int32]Process
}

func (w *Worker) Decoder(encoding string) Decoder {
	if encoding == "" {
		return w.defaultDecoder
	} else {
		return w.options.Decoders[encoding]
	}
}

func (w *Worker) Encoder(encoding string) Encoder {
	if encoding == "" {
		return w.defaultEncoder
	} else {
		return w.options.Encoders[encoding]
	}
}

func (w *Worker) ExecuteJob(job engine.Job) (engine.Job, error) {
	process, ok := w.processes[job.ProcessId]
	if !ok {
		return engine.Job{}, fmt.Errorf("no process registered for ID %d", job.ProcessId)
	}

	jc := JobContext{
		Engine:  w.engine,
		Job:     job,
		Process: process.Process,
		Element: process.Elements[job.BpmnElementId],

		worker: w,

		processVariables: Variables{},
		elementVariables: Variables{},
	}

	delegation := process.delegator[job.BpmnElementId]
	if delegation == nil {
		return engine.Job{}, fmt.Errorf("no job delegation registered for process %s and BPMN element %s", jc.Process, job.BpmnElementId)
	}

	completion, delegationErr := delegation(jc)

	elementVariables, err := jc.worker.encodeVariables(jc.elementVariables)
	if err != nil {
		return engine.Job{}, err
	}

	processVariables, err := jc.worker.encodeVariables(jc.processVariables)
	if err != nil {
		return engine.Job{}, err
	}

	cmd := engine.CompleteJobCmd{
		Partition: job.Partition,
		Id:        job.Id,

		Completion:       completion,
		ElementVariables: elementVariables,
		ProcessVariables: processVariables,
		RetryCount:       job.RetryCount - 1,
		RetryTimer:       job.RetryTimer,
		WorkerId:         w.id,
	}

	if delegationErr != nil {
		if jobErr, ok := delegationErr.(jobError); ok {
			if err := jobErr.Unwrap(); err != nil {
				cmd.Error = err.Error()
			} else {
				cmd.Error = fmt.Sprintf("%T failed to execute job", process.delegate)
			}

			cmd.RetryCount = jobErr.retryCount
			cmd.RetryTimer = jobErr.retryTimer
		} else {
			cmd.Error = delegationErr.Error()
		}
	}

	return w.engine.CompleteJob(cmd)
}

func (w *Worker) Register(delegate Delegate) (Process, error) {
	createProcessCmd, err := delegate.CreateProcessCmd()
	if err != nil {
		return Process{}, fmt.Errorf("failed to create process command: %v", err)
	}

	createProcessCmd.WorkerId = w.id

	process, err := w.engine.CreateProcess(createProcessCmd)
	if err != nil {
		return Process{}, fmt.Errorf("failed to create process: %v", err)
	}

	results, err := w.engine.Query(engine.ElementCriteria{
		ProcessId: process.Id,
	})
	if err != nil {
		return Process{}, fmt.Errorf("failed to query elements: %v", err)
	}

	elements := make(map[string]engine.Element, len(results))
	for i := 0; i < len(results); i++ {
		element := results[i].(engine.Element)
		elements[element.BpmnElementId] = element
	}

	delegator := Delegator{}
	if err := delegate.Delegate(delegator); err != nil {
		return Process{}, fmt.Errorf("failed to delegate jobs: %v", err)
	}

	for bpmnElementId := range delegator {
		if _, ok := elements[bpmnElementId]; !ok {
			return Process{}, fmt.Errorf("invalid job delegation %s: process %s has no such BPMN element", bpmnElementId, process)
		}
	}

	w.processes[process.Id] = Process{
		Process:  process,
		Elements: elements,

		worker:    w,
		delegate:  delegate,
		delegator: delegator,
	}

	return w.processes[process.Id], nil
}

func (w *Worker) Start() {
	w.jobExecutor = newJobExecutor(w)
	w.jobExecutor.execute()
}

func (w *Worker) Stop() {
	if w.jobExecutor != nil {
		w.jobExecutor.stop()
		w.jobExecutor = nil
	}
}

func (w *Worker) encodeVariables(variables Variables) (map[string]*engine.Data, error) {
	if len(variables) == 0 {
		return nil, nil
	}

	encodedVariables := make(map[string]*engine.Data, len(variables))
	for _, variable := range variables {
		if variable.IsDeleted() {
			encodedVariables[variable.Name] = nil
			continue
		}

		encoding := variable.Encoding
		if encoding == "" {
			encoding = w.options.DefaultEncoding
		}

		encoder := w.Encoder(encoding)
		if encoder == nil {
			return nil, fmt.Errorf("no encoder registered for %s", encoding)
		}

		value, err := encoder.Encode(variable.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to encode variable %s: %v", variable.Name, err)
		}

		data := engine.Data{
			Encoding:    encoding,
			IsEncrypted: variable.IsEncrypted,
			Value:       value,
		}

		encodedVariables[variable.Name] = &data
	}

	return encodedVariables, nil
}

type jsonDecoder struct{}

func (d jsonDecoder) Decode(data string, value any) error {
	return json.Unmarshal([]byte(data), value)
}

type jsonEncoder struct{}

func (e jsonEncoder) Encode(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	} else {
		return string(b), nil
	}
}
