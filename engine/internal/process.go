package internal

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func NewProcessCache() *ProcessCache {
	return &ProcessCache{
		processes:     make(map[string]*ProcessEntity),
		processesById: make(map[int32]*ProcessEntity),
	}
}

type ProcessCache struct {
	mutex         sync.RWMutex
	processes     map[string]*ProcessEntity
	processesById map[int32]*ProcessEntity
}

func (c *ProcessCache) Add(process *ProcessEntity) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.processesById[process.Id] = process

	key := fmt.Sprintf("%s:%s", process.BpmnProcessId, process.Version)
	c.processes[key] = process
}

func (c *ProcessCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	clear(c.processes)
	clear(c.processesById)
}

func (c *ProcessCache) Get(bpmnProcessId string, version string) (*ProcessEntity, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	key := fmt.Sprintf("%s:%s", bpmnProcessId, version)
	process, ok := c.processes[key]
	return process, ok
}

func (c *ProcessCache) GetById(id int32) (*ProcessEntity, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	process, ok := c.processesById[id]
	return process, ok
}

func (c *ProcessCache) GetOrCache(ctx Context, bpmnProcessId string, version string) (*ProcessEntity, error) {
	if process, ok := c.Get(bpmnProcessId, version); ok {
		return process, nil
	}

	process, err := ctx.Processes().SelectByBpmnProcessIdAndVersion(bpmnProcessId, version)
	if err != nil {
		return nil, err
	}

	if err := c.cache(ctx, process); err != nil {
		if _, ok := err.(engine.Error); ok {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to cache process %s:%s: %v", bpmnProcessId, version, err)
		}
	}

	return process, nil
}

func (c *ProcessCache) GetOrCacheById(ctx Context, id int32) (*ProcessEntity, error) {
	if process, ok := c.GetById(id); ok {
		return process, nil
	}

	process, err := ctx.Processes().Select(id)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to select process %d: %v", id, err)
	}
	if err != nil {
		return nil, err
	}

	if err := c.cache(ctx, process); err != nil {
		if _, ok := err.(engine.Error); ok {
			return nil, err
		} else {
			return nil, fmt.Errorf("failed to cache process %d: %v", id, err)
		}
	}

	return process, nil
}

func (c *ProcessCache) cache(ctx Context, process *ProcessEntity) error {
	bpmnModel, err := model.New(strings.NewReader(process.BpmnXml))
	if err != nil {
		return err
	}

	processElement := bpmnModel.ProcessById(process.BpmnProcessId)
	if processElement == nil {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to cache process",
			Detail: fmt.Sprintf("BPMN model has no process %s", process.BpmnProcessId),
		}
	}

	elements, err := ctx.Elements().SelectByProcessId(process.Id)
	if err != nil {
		return err
	}

	bpmnElements := bpmnModel.ElementsByProcessId(process.BpmnProcessId)

	graph, err := newGraph(bpmnModel, bpmnElements, elements)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to create execution graph",
			Detail: err.Error(),
		}
	}

	eventDefinitions, err := ctx.EventDefinitions().SelectByProcessId(process.Id)
	if err != nil {
		return err
	}

	graph.setEventDefinitions(eventDefinitions)

	process.graph = &graph
	c.Add(process)

	return nil
}

type ProcessEntity struct {
	Id int32

	BpmnProcessId string
	BpmnXml       string
	BpmnXmlMd5    string
	CreatedAt     time.Time
	CreatedBy     string
	Parallelism   int
	Tags          pgtype.Text
	Version       string

	graph *graph
}

func (e ProcessEntity) Process() engine.Process {
	var tags map[string]string
	if e.Tags.Valid {
		_ = json.Unmarshal([]byte(e.Tags.String), &tags)
	}

	return engine.Process{
		Id: e.Id,

		BpmnProcessId: e.BpmnProcessId,
		CreatedAt:     e.CreatedAt,
		CreatedBy:     e.CreatedBy,
		Parallelism:   e.Parallelism,
		Tags:          tags,
		Version:       e.Version,
	}
}

type ProcessRepository interface {
	// Insert inserts a process.
	//
	// If a concurrent insert caused an conflict (BPMN process ID and version must be unique), [pgx.ErrNoRows] is returned.
	Insert(*ProcessEntity) error

	Select(id int32) (*ProcessEntity, error)

	// SelectByBpmnProcessIdAndVersion selects a process by BPMN process ID and version.
	//
	// If no process is found, nil is returned.
	SelectByBpmnProcessIdAndVersion(bpmnProcessId string, version string) (*ProcessEntity, error)

	Query(engine.ProcessCriteria, engine.QueryOptions) ([]engine.Process, error)
}

func CreateProcess(ctx Context, cmd engine.CreateProcessCmd) (engine.Process, error) {
	md5Hash := md5.New()
	md5Hash.Write([]byte(cmd.BpmnXml))
	bpmnXmlMd5 := hex.EncodeToString(md5Hash.Sum(nil))

	bpmnModel, err := model.New(strings.NewReader(cmd.BpmnXml))
	if err != nil {
		return engine.Process{}, engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create process",
			Detail: fmt.Sprintf("BPMN XML is invalid: %v", err),
		}
	}

	// find process
	processElement := bpmnModel.ProcessById(cmd.BpmnProcessId)
	if processElement == nil {
		// collect actual BPMN process IDs
		bpmnProcessIds := make([]string, len(bpmnModel.Definitions.Processes))
		for i := range bpmnProcessIds {
			bpmnProcessIds[i] = bpmnModel.Definitions.Processes[i].Id
		}

		return engine.Process{}, engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create process",
			Detail: fmt.Sprintf("BPMN model has no process %s, but [%s]", cmd.BpmnProcessId, strings.Join(bpmnProcessIds, ", ")),
		}
	}

	bpmnElements := bpmnModel.ElementsByProcessId(cmd.BpmnProcessId)

	// validate process
	causes, err := validateProcess(bpmnElements)
	if err != nil {
		return engine.Process{}, err
	}

	if len(causes) != 0 {
		return engine.Process{}, engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create process",
			Detail: "BPMN process is invalid",
			Causes: causes,
		}
	}

	// validate events
	for _, bpmnElement := range bpmnElements {
		errorCode := cmd.ErrorCodes[bpmnElement.Id]
		if errorCode != "" {
			if bpmnElement.Type != model.ElementErrorBoundaryEvent {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "error_event",
					Detail:  fmt.Sprintf("BPMN element %s is not an error boundary event", bpmnElement.Id),
				})
			}
		}

		messageName := cmd.MessageNames[bpmnElement.Id]
		if messageName != "" {
			if bpmnElement.Type != model.ElementMessageStartEvent {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "message_event",
					Detail:  fmt.Sprintf("BPMN element %s is not a message start event", bpmnElement.Id),
				})
			}
		} else {
			if bpmnElement.Type == model.ElementMessageStartEvent {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "message_event",
					Detail:  fmt.Sprintf("no message name defined for BPMN element %s", bpmnElement.Id),
				})
			}
		}

		signalName := cmd.SignalNames[bpmnElement.Id]
		if signalName != "" {
			if bpmnElement.Type != model.ElementSignalStartEvent {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "signal_event",
					Detail:  fmt.Sprintf("BPMN element %s is not a signal start event", bpmnElement.Id),
				})
			}
		} else {
			if bpmnElement.Type == model.ElementSignalStartEvent {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "signal_event",
					Detail:  fmt.Sprintf("no signal name defined for BPMN element %s", bpmnElement.Id),
				})
			}
		}

		timer := cmd.Timers[bpmnElement.Id]
		if timer != nil {
			if bpmnElement.Type != model.ElementTimerStartEvent {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "timer_event",
					Detail:  fmt.Sprintf("BPMN element %s is not a timer start event", bpmnElement.Id),
				})
			}
		} else {
			if bpmnElement.Type == model.ElementTimerStartEvent {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "timer_event",
					Detail:  fmt.Sprintf("no timer defined for BPMN element %s", bpmnElement.Id),
				})
			}
		}
	}

	if len(causes) != 0 {
		return engine.Process{}, engine.Error{
			Type:   engine.ErrorValidation,
			Title:  "failed to create process",
			Detail: "invalid or missing event definitions",
			Causes: causes,
		}
	}

	// insert process
	var tags string
	if len(cmd.Tags) != 0 {
		b, err := json.Marshal(cmd.Tags)
		if err != nil {
			return engine.Process{}, fmt.Errorf("failed to marshal tags: %v", err)
		}
		tags = string(b)
	}

	process := &ProcessEntity{
		BpmnProcessId: cmd.BpmnProcessId,
		BpmnXml:       cmd.BpmnXml,
		BpmnXmlMd5:    bpmnXmlMd5,
		CreatedAt:     ctx.Time(),
		CreatedBy:     cmd.WorkerId,
		Parallelism:   cmd.Parallelism,
		Tags:          pgtype.Text{String: tags, Valid: tags != ""},
		Version:       cmd.Version,
	}

	var isConflict bool
	if err := ctx.Processes().Insert(process); err != nil {
		if err != pgx.ErrNoRows {
			return engine.Process{}, err
		}

		isConflict = true

		// select the concurrently inserted entity due to a conflict
		process, err = ctx.Processes().SelectByBpmnProcessIdAndVersion(cmd.BpmnProcessId, cmd.Version)
		if err != nil {
			return engine.Process{}, err
		}
	}

	// compare checksums
	if process.BpmnXmlMd5 != bpmnXmlMd5 {
		return engine.Process{}, engine.Error{
			Type:   engine.ErrorConflict,
			Title:  "failed to create process",
			Detail: fmt.Sprintf("process %s:%s already exists with a different BPMN XML", cmd.BpmnProcessId, cmd.Version),
		}
	}

	if isConflict {
		return process.Process(), nil
	}

	// insert elements
	elements := make([]*ElementEntity, len(bpmnElements))
	for i, bpmnElement := range bpmnElements {
		element := ElementEntity{
			ProcessId: process.Id,

			BpmnElementId:   bpmnElement.Id,
			BpmnElementName: bpmnElement.Name,
			BpmnElementType: bpmnElement.Type,
		}

		elements[i] = &element
	}

	if err := ctx.Elements().InsertBatch(elements); err != nil {
		return engine.Process{}, err
	}

	// create execution graph
	graph, err := newGraph(bpmnModel, bpmnElements, elements)
	if err != nil {
		return engine.Process{}, engine.Error{
			Type:   engine.ErrorBug,
			Title:  "failed to create execution graph",
			Detail: err.Error(),
		}
	}

	eventDefinitions := make([]*EventDefinitionEntity, 0, 0+
		len(cmd.ErrorCodes)+
		len(cmd.MessageNames)+
		len(cmd.SignalNames)+
		len(cmd.Timers),
	)

	// prepare error event definitions
	for bpmnElementId, errorCode := range cmd.ErrorCodes {
		node, ok := graph.nodes[bpmnElementId]
		if !ok {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(graph.processElement),
				Type:    "error_event",
				Detail:  fmt.Sprintf("BPMN process %s has no element %s", processElement.Id, bpmnElementId),
			})
			continue
		}

		eventDefinitions = append(eventDefinitions, &EventDefinitionEntity{
			ElementId: node.id,

			ProcessId: process.Id,

			BpmnElementId:   bpmnElementId,
			BpmnElementType: node.bpmnElement.Type,
			BpmnProcessId:   process.BpmnProcessId,
			ErrorCode:       pgtype.Text{String: errorCode, Valid: true},
			Version:         process.Version,
		})
	}

	// prepare message event definitions
	for bpmnElementId, messageName := range cmd.MessageNames {
		node, ok := graph.nodes[bpmnElementId]
		if !ok {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(graph.processElement),
				Type:    "message_event",
				Detail:  fmt.Sprintf("BPMN process %s has no element %s", processElement.Id, bpmnElementId),
			})
			continue
		}

		if messageName == "" {
			continue
		}

		eventDefinitions = append(eventDefinitions, &EventDefinitionEntity{
			ElementId: node.id,

			ProcessId: process.Id,

			BpmnElementId:   bpmnElementId,
			BpmnElementType: node.bpmnElement.Type,
			BpmnProcessId:   process.BpmnProcessId,
			MessageName:     pgtype.Text{String: messageName, Valid: true},
			Version:         process.Version,
		})
	}

	// prepare signal event definitions
	for bpmnElementId, signalName := range cmd.SignalNames {
		node, ok := graph.nodes[bpmnElementId]
		if !ok {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(graph.processElement),
				Type:    "signal_event",
				Detail:  fmt.Sprintf("BPMN process %s has no element %s", processElement.Id, bpmnElementId),
			})
			continue
		}

		if signalName == "" {
			continue
		}

		eventDefinitions = append(eventDefinitions, &EventDefinitionEntity{
			ElementId: node.id,

			ProcessId: process.Id,

			BpmnElementId:   bpmnElementId,
			BpmnElementType: node.bpmnElement.Type,
			BpmnProcessId:   process.BpmnProcessId,
			SignalName:      pgtype.Text{String: signalName, Valid: true},
			Version:         process.Version,
		})
	}

	// prepare timer event definitions and trigger timer event tasks
	triggerTimerEventTasks := make([]*TaskEntity, 0, len(cmd.Timers))

	for bpmnElementId, timer := range cmd.Timers {
		node, ok := graph.nodes[bpmnElementId]
		if !ok {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(graph.processElement),
				Type:    "timer_event",
				Detail:  fmt.Sprintf("BPMN process %s has no element %s", processElement.Id, bpmnElementId),
			})
			continue
		}

		if timer == nil {
			continue
		}

		eventDefinitions = append(eventDefinitions, &EventDefinitionEntity{
			ElementId: node.id,

			ProcessId: process.Id,

			BpmnElementId:   bpmnElementId,
			BpmnElementType: node.bpmnElement.Type,
			BpmnProcessId:   process.BpmnProcessId,
			Time:            pgtype.Timestamp{Time: timer.Time, Valid: !timer.Time.IsZero()},
			TimeCycle:       pgtype.Text{String: timer.TimeCycle, Valid: timer.TimeCycle != ""},
			TimeDuration:    pgtype.Text{String: timer.TimeDuration.String(), Valid: !timer.TimeDuration.IsZero()},
			Version:         process.Version,
		})

		dueAt, err := evaluateTimer(*timer, ctx.Time())
		if err != nil {
			causes = append(causes, engine.ErrorCause{
				Pointer: elementPointer(node.bpmnElement),
				Type:    "timer_event",
				Detail:  fmt.Sprintf("failed to evaluate timer: %v", err),
			})
			continue
		}

		triggerTimerEventTasks = append(triggerTimerEventTasks, &TaskEntity{
			Partition: ctx.Date(),

			ElementId: pgtype.Int4{Int32: node.id, Valid: true},
			ProcessId: pgtype.Int4{Int32: process.Id, Valid: true},

			CreatedAt: ctx.Time(),
			CreatedBy: cmd.WorkerId,
			DueAt:     dueAt,
			Type:      engine.TaskTriggerEvent,

			Instance: TriggerEventTask{Timer: timer},
		})
	}

	if len(causes) != 0 {
		return engine.Process{}, engine.Error{
			Type:   engine.ErrorValidation,
			Title:  "failed to create process",
			Detail: "invalid or missing event definitions",
			Causes: causes,
		}
	}

	// update parallelism
	if err := ctx.ProcessInstanceQueues().Upsert(&ProcessInstanceQueueEntity{
		BpmnProcessId: process.BpmnProcessId,
		Parallelism:   process.Parallelism,
	}); err != nil {
		return engine.Process{}, err
	}

	processInstanceQueue, err := ctx.ProcessInstanceQueues().Select(process.BpmnProcessId)
	if err != nil {
		return engine.Process{}, err
	}

	if processInstanceQueue.MustDequeue() {
		dequeueProcessInstance := TaskEntity{
			Partition: ctx.Date(),

			ProcessId: pgtype.Int4{Int32: process.Id, Valid: true},

			CreatedAt: ctx.Time(),
			CreatedBy: cmd.WorkerId,
			DueAt:     ctx.Time(),
			Type:      engine.TaskDequeueProcessInstance,

			Instance: DequeueProcessInstanceTask{BpmnProcessId: cmd.BpmnProcessId},
		}

		if err := ctx.Tasks().Insert(&dequeueProcessInstance); err != nil {
			return engine.Process{}, err
		}
	}

	// suspend event definitions
	if err := suspendEventDefinitions(ctx, process.BpmnProcessId); err != nil {
		return engine.Process{}, err
	}

	// insert event definitions
	if err := ctx.EventDefinitions().InsertBatch(eventDefinitions); err != nil {
		return engine.Process{}, err
	}

	// insert trigger timer trigger tasks
	if err := ctx.Tasks().InsertBatch(triggerTimerEventTasks); err != nil {
		return engine.Process{}, err
	}

	graph.setEventDefinitions(eventDefinitions)

	// cache process
	process.graph = &graph
	ctx.ProcessCache().Add(process)

	return process.Process(), nil
}

func GetBpmnXml(ctx Context, cmd engine.GetBpmnXmlCmd) (string, error) {
	if process, ok := ctx.ProcessCache().GetById(cmd.ProcessId); ok {
		return process.BpmnXml, nil
	}

	process, err := ctx.Processes().Select(cmd.ProcessId)
	if err == pgx.ErrNoRows {
		return "", engine.Error{
			Type:   engine.ErrorNotFound,
			Title:  "failed to get BPMN XML",
			Detail: fmt.Sprintf("process %d could not be found", cmd.ProcessId),
		}
	}
	if err != nil {
		return "", err
	}

	return process.BpmnXml, nil
}
