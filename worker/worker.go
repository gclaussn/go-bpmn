package worker

import (
	"context"
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
		e:              e,
		defaultDecoder: options.Decoders[options.DefaultEncoding],
		defaultEncoder: options.Encoders[options.DefaultEncoding],
		id:             options.WorkerId,
		options:        options,
		processHandles: make(map[int32]ProcessHandle),
	}

	return &worker, nil
}

func NewBpmnError(code string) error {
	return bpmnError{code: code}
}

func NewBpmnEscalation(code string) error {
	return bpmnEscalation{code: code}
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

type Decoder interface {
	Decode(string, any) error
}

type Encoder interface {
	Encode(any) (string, error)
}

// Handler must be implemented to automate a [engine.Process].
type Handler interface {
	CreateProcessCmd() (engine.CreateProcessCmd, error)
	Handle(JobMux) error
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

type ProcessHandle struct {
	Process  engine.Process
	Elements map[string]engine.Element

	handler Handler
	mux     JobMux
	w       *Worker
}

func (h ProcessHandle) CreateProcessInstanceCmd() engine.CreateProcessInstanceCmd {
	return engine.CreateProcessInstanceCmd{
		BpmnProcessId: h.Process.BpmnProcessId,
		Version:       h.Process.Version,
		WorkerId:      h.w.id,
	}
}

func (h ProcessHandle) CreateProcessInstance(ctx context.Context, variables Variables) (engine.ProcessInstance, error) {
	processVariables, err := h.w.encodeVariables(variables)
	if err != nil {
		return engine.ProcessInstance{}, nil
	}

	return h.w.e.CreateProcessInstance(ctx, engine.CreateProcessInstanceCmd{
		BpmnProcessId: h.Process.BpmnProcessId,
		Version:       h.Process.Version,
		Variables:     processVariables,
		WorkerId:      h.w.id,
	})
}

type Worker struct {
	e              engine.Engine
	defaultDecoder Decoder
	defaultEncoder Encoder
	id             string
	jobExecutor    *jobExecutor
	options        Options
	processHandles map[int32]ProcessHandle
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

func (w *Worker) ExecuteJob(ctx context.Context, job engine.Job) (engine.Job, error) {
	processHandle, ok := w.processHandles[job.ProcessId]
	if !ok {
		return engine.Job{}, fmt.Errorf("no handler registered for process ID %d", job.ProcessId)
	}

	jc := JobContext{
		Job:     job,
		Process: processHandle.Process,
		Element: processHandle.Elements[job.BpmnElementId],

		w:   w,
		ctx: ctx,

		processVariables: Variables{},
		elementVariables: Variables{},
	}

	jobHandler := processHandle.mux[job.BpmnElementId]
	if jobHandler == nil {
		return engine.Job{}, fmt.Errorf("no job handler registered for process %s and BPMN element %s", jc.Process, job.BpmnElementId)
	}

	completion, jobHandlerErr := jobHandler(jc)

	elementVariables, err := jc.w.encodeVariables(jc.elementVariables)
	if err != nil {
		return engine.Job{}, err
	}

	processVariables, err := jc.w.encodeVariables(jc.processVariables)
	if err != nil {
		return engine.Job{}, err
	}

	cmd := engine.CompleteJobCmd{
		Partition: job.Partition,
		Id:        job.Id,

		Completion:       completion,
		ElementVariables: elementVariables,
		ProcessVariables: processVariables,
		WorkerId:         w.id,
	}

	if jobHandlerErr != nil {
		switch jobHandlerErr := jobHandlerErr.(type) {
		case jobError:
			if err := jobHandlerErr.Unwrap(); err != nil {
				cmd.Error = err.Error()
			} else {
				cmd.Error = fmt.Sprintf("%T failed to execute job", processHandle.handler)
			}

			cmd.RetryLimit = jobHandlerErr.retryLimit
			cmd.RetryTimer = jobHandlerErr.retryTimer
		case bpmnError:
			cmd.Completion = &engine.JobCompletion{
				ErrorCode: jobHandlerErr.code,
			}
		case bpmnEscalation:
			cmd.Completion = &engine.JobCompletion{
				EscalationCode: jobHandlerErr.code,
			}
		default:
			cmd.Error = jobHandlerErr.Error()
		}
	}

	return w.e.CompleteJob(ctx, cmd)
}

func (w *Worker) Register(handler Handler) (ProcessHandle, error) {
	createProcessCmd, err := handler.CreateProcessCmd()
	if err != nil {
		return ProcessHandle{}, fmt.Errorf("failed to create process command: %v", err)
	}

	createProcessCmd.WorkerId = w.id

	process, err := w.e.CreateProcess(context.Background(), createProcessCmd)
	if err != nil {
		return ProcessHandle{}, fmt.Errorf("failed to create process: %v", err)
	}

	if _, ok := w.processHandles[process.Id]; ok {
		return ProcessHandle{}, fmt.Errorf("handler for process %s:%s is already registered", createProcessCmd.BpmnProcessId, createProcessCmd.Version)
	}

	elements, err := w.e.CreateQuery().QueryElements(context.Background(), engine.ElementCriteria{ProcessId: process.Id})
	if err != nil {
		return ProcessHandle{}, fmt.Errorf("failed to query elements: %v", err)
	}

	elementMap := make(map[string]engine.Element, len(elements))
	for _, element := range elements {
		elementMap[element.BpmnElementId] = element
	}

	mux := make(JobMux, len(elements))
	if err := handler.Handle(mux); err != nil {
		return ProcessHandle{}, fmt.Errorf("failed to register job handlers: %v", err)
	}

	for bpmnElementId := range mux {
		if _, ok := elementMap[bpmnElementId]; !ok {
			return ProcessHandle{}, fmt.Errorf("invalid job handler %s: process %s has no such BPMN element", bpmnElementId, process)
		}
	}

	processHandle := ProcessHandle{
		Process:  process,
		Elements: elementMap,

		handler: handler,
		mux:     mux,
		w:       w,
	}

	w.processHandles[process.Id] = processHandle

	return processHandle, nil
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

func (w *Worker) encodeVariables(variables Variables) ([]engine.VariableData, error) {
	if len(variables) == 0 {
		return nil, nil
	}

	encodedVariables := make([]engine.VariableData, 0, len(variables))
	for variableName, variable := range variables {
		if variable.IsDeleted() {
			encodedVariables = append(encodedVariables, engine.VariableData{
				Name: variableName,
			})
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
			return nil, fmt.Errorf("failed to encode variable %s: %v", variableName, err)
		}

		encodedVariables = append(encodedVariables, engine.VariableData{
			Name: variableName,
			Data: &engine.Data{
				Encoding:    encoding,
				IsEncrypted: variable.IsEncrypted,
				Value:       value,
			},
		})
	}

	return encodedVariables, nil
}

type bpmnError struct {
	code string
}

func (e bpmnError) Error() string {
	return e.code
}

type bpmnEscalation struct {
	code string
}

func (e bpmnEscalation) Error() string {
	return e.code
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
