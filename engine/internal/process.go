package internal

import (
	"container/list"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func NewProcessCache(capacity int, expiration time.Duration) *ProcessCache {
	return &ProcessCache{
		capacity:   capacity,
		expiration: expiration,

		l:             list.New(),
		processes:     make(map[string]*list.Element),
		processesById: make(map[int32]*list.Element),
	}
}

type ProcessCache struct {
	capacity   int
	expiration time.Duration

	mutex         sync.RWMutex
	l             *list.List
	processes     map[string]*list.Element
	processesById map[int32]*list.Element
}

func (c *ProcessCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	clear(c.processes)
	clear(c.processesById)

	for {
		element := c.l.Front()
		if element == nil {
			break
		}
		c.l.Remove(element)
	}
}

func (c *ProcessCache) Get(ctx Context, bpmnProcessId string, version string) (*ProcessEntity, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	key := fmt.Sprintf("%s:%s", bpmnProcessId, version)
	element, ok := c.processes[key]
	if !ok {
		return nil, false
	}

	entry := element.Value.(*ProcessCacheEntry)
	if entry.IsExpired(ctx.Time()) {
		c.l.Remove(element)
		delete(c.processes, key)
		delete(c.processesById, entry.process.Id)
		return nil, false
	}

	c.l.MoveToFront(element)
	return entry.process, true
}

func (c *ProcessCache) GetById(ctx Context, id int32) (*ProcessEntity, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	element, ok := c.processesById[id]
	if !ok {
		return nil, false
	}

	entry := element.Value.(*ProcessCacheEntry)
	if entry.IsExpired(ctx.Time()) {
		c.l.Remove(element)
		delete(c.processes, entry.Key())
		delete(c.processesById, entry.process.Id)
		return nil, false
	}

	c.l.MoveToFront(element)
	return entry.process, true
}

func (c *ProcessCache) GetOrCache(ctx Context, bpmnProcessId string, version string) (*ProcessEntity, error) {
	if process, ok := c.Get(ctx, bpmnProcessId, version); ok {
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
	if process, ok := c.GetById(ctx, id); ok {
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

func (c *ProcessCache) Put(ctx Context, process *ProcessEntity) {
	entry := &ProcessCacheEntry{
		process:   process,
		expiresAt: ctx.Time().Add(c.expiration),
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if element, ok := c.processesById[entry.process.Id]; ok {
		element.Value = entry
		c.l.MoveToFront(element)
		return
	}

	element := c.l.PushFront(entry)

	c.processes[entry.Key()] = element
	c.processesById[entry.process.Id] = element

	if c.l.Len() > c.capacity {
		oldest := c.l.Back()
		oldestEntry := oldest.Value.(*ProcessCacheEntry)

		c.l.Remove(oldest)
		delete(c.processes, oldestEntry.Key())
		delete(c.processesById, oldestEntry.process.Id)
	}
}

func (c *ProcessCache) Size() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.l.Len()
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

	c.Put(ctx, process)
	return nil
}

type ProcessCacheEntry struct {
	process   *ProcessEntity
	expiresAt time.Time
}

func (e ProcessCacheEntry) Key() string {
	return fmt.Sprintf("%s:%s", e.process.BpmnProcessId, e.process.Version)
}

func (e ProcessCacheEntry) IsExpired(t time.Time) bool {
	return !t.Before(e.expiresAt)
}

type ProcessEntity struct {
	Id int32

	BpmnCollaborationId pgtype.Text
	BpmnParticipantId   pgtype.Text
	BpmnParticipantName pgtype.Text
	BpmnProcessId       string
	BpmnXml             string
	BpmnXmlMd5          string
	CreatedAt           time.Time
	CreatedBy           string
	Parallelism         int
	Tags                pgtype.Text
	Version             string

	graph *graph
}

func (e ProcessEntity) Process() engine.Process {
	return engine.Process{
		Id: e.Id,

		BpmnCollaborationId: e.BpmnCollaborationId.String,
		BpmnParticipantId:   e.BpmnParticipantId.String,
		BpmnParticipantName: e.BpmnParticipantName.String,
		BpmnProcessId:       e.BpmnProcessId,
		CreatedAt:           e.CreatedAt,
		CreatedBy:           e.CreatedBy,
		Parallelism:         e.Parallelism,
		Tags:                unmarshalTags(e.Tags),
		Version:             e.Version,
	}
}

type ProcessRepository interface {
	// Insert inserts a process entity. BPMN process ID and version must be unique.
	//
	// If a process with the same BPMN process ID and version exists,
	// the entity's pointer value is replaced with the value of the already existing entity.
	//
	// If a concurrent insert caused a conflict,
	// the entity's pointer value is replaced with the value of the concurrently inserted entity.
	//
	// In both cases, true is returned to indicate a conflict.
	Insert(*ProcessEntity) (bool, error)

	Select(id int32) (*ProcessEntity, error)

	// SelectByBpmnProcessIdAndVersion selects a process by BPMN process ID and version.
	//
	// If no process is found, [pgx.ErrNoRows] is returned.
	SelectByBpmnProcessIdAndVersion(bpmnProcessId string, version string) (*ProcessEntity, error)

	// SelectLatest selects the latest process version - the version with the greatest created_at.
	//
	// If no process is found, [pgx.ErrNoRows] is returned.
	SelectLatest(bpmnProcessId string) (*ProcessEntity, error)

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

	// map event definitions
	var (
		errorEvents      []*model.Element
		escalationEvents []*model.Element
		messageEvents    []*model.Element
		signalEvents     []*model.Element
		timerEvents      []*model.Element

		eventDefinitions []EventDefinitionEntity

		timerCount int // number of timer event definitions
	)

	for _, bpmnElement := range bpmnElements {
		switch bpmnElement.Type {
		case
			model.ElementErrorBoundaryEvent,
			model.ElementErrorEndEvent:
			errorEvents = append(errorEvents, bpmnElement)
		case
			model.ElementEscalationBoundaryEvent,
			model.ElementEscalationEndEvent,
			model.ElementEscalationThrowEvent:
			escalationEvents = append(escalationEvents, bpmnElement)
		case
			model.ElementMessageBoundaryEvent,
			model.ElementMessageCatchEvent,
			model.ElementMessageEndEvent,
			model.ElementMessageStartEvent,
			model.ElementMessageThrowEvent:
			messageEvents = append(messageEvents, bpmnElement)
		case
			model.ElementSignalBoundaryEvent,
			model.ElementSignalCatchEvent,
			model.ElementSignalEndEvent,
			model.ElementSignalStartEvent,
			model.ElementSignalThrowEvent:
			signalEvents = append(signalEvents, bpmnElement)
		case
			model.ElementTimerBoundaryEvent,
			model.ElementTimerCatchEvent,
			model.ElementTimerStartEvent:
			timerEvents = append(timerEvents, bpmnElement)
		}
	}

	if len(cmd.Errors) != 0 || len(errorEvents) != 0 {
		eventDefinitions, causes = mapErrorEventDefinitions(eventDefinitions, causes, cmd.Errors, errorEvents, processElement)
	}
	if len(cmd.Escalations) != 0 || len(escalationEvents) != 0 {
		eventDefinitions, causes = mapEscalationEventDefinitions(eventDefinitions, causes, cmd.Escalations, escalationEvents, processElement)
	}
	if len(cmd.Messages) != 0 || len(messageEvents) != 0 {
		eventDefinitions, causes = mapMessageEventDefinitions(eventDefinitions, causes, cmd.Messages, messageEvents, processElement)
	}
	if len(cmd.Signals) != 0 || len(signalEvents) != 0 {
		eventDefinitions, causes = mapSignalEventDefinitions(eventDefinitions, causes, cmd.Signals, signalEvents, processElement)
	}
	if len(cmd.Timers) != 0 || len(timerEvents) != 0 {
		timerCount = len(eventDefinitions)
		eventDefinitions, causes = mapTimerEventDefinitions(eventDefinitions, causes, cmd.Timers, timerEvents, processElement)
		timerCount = len(eventDefinitions) - timerCount
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
	tags, err := marshalTags(cmd.Tags)
	if err != nil {
		return engine.Process{}, err
	}

	collaboration := bpmnModel.Definitions.Collaboration

	var (
		collaborationId string
		participantId   string
		participantName string
	)
	if collaboration != nil {
		collaborationId = collaboration.Id

		for _, participant := range collaboration.Participants {
			if participant.Process == nil {
				continue
			}
			if participant.Process.Id != cmd.BpmnProcessId {
				continue
			}

			participantId, participantName = participant.Id, participant.Name
			break
		}
	}

	process := &ProcessEntity{
		BpmnCollaborationId: pgtype.Text{String: collaborationId, Valid: collaborationId != ""},
		BpmnParticipantId:   pgtype.Text{String: participantId, Valid: participantId != ""},
		BpmnParticipantName: pgtype.Text{String: participantName, Valid: participantName != ""},
		BpmnProcessId:       cmd.BpmnProcessId,
		BpmnXml:             cmd.BpmnXml,
		BpmnXmlMd5:          bpmnXmlMd5,
		CreatedAt:           ctx.Time(),
		CreatedBy:           cmd.WorkerId,
		Parallelism:         cmd.Parallelism,
		Tags:                tags,
		Version:             cmd.Version,
	}

	isConflict, err := ctx.Processes().Insert(process)
	if err != nil {
		return engine.Process{}, err
	}

	// compare checksums
	if isConflict && process.BpmnXmlMd5 != bpmnXmlMd5 {
		return engine.Process{}, engine.Error{
			Type:   engine.ErrorConflict,
			Title:  "failed to create process",
			Detail: fmt.Sprintf("process %s:%s exists with a different BPMN XML", cmd.BpmnProcessId, cmd.Version),
		}
	}

	if isConflict {
		return process.Process(), nil
	}

	// insert elements
	elements := make([]*ElementEntity, len(bpmnElements))
	for i, bpmnElement := range bpmnElements {
		var bpmnElementName pgtype.Text
		if bpmnElement.Name != "" {
			bpmnElementName = pgtype.Text{String: bpmnElement.Name, Valid: true}
		}

		var parentBpmnElementId pgtype.Text
		if bpmnElement.Parent != nil {
			parentBpmnElementId = pgtype.Text{String: bpmnElement.Parent.Id, Valid: true}
		}

		element := ElementEntity{
			ProcessId: process.Id,

			BpmnElementId:       bpmnElement.Id,
			BpmnElementName:     bpmnElementName,
			BpmnElementType:     bpmnElement.Type,
			IsMultiInstance:     getMultiInstance(bpmnElement) != nil,
			ParentBpmnElementId: parentBpmnElementId,
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

	// prepare event definitions
	triggerTimerEventTasks := make([]*TaskEntity, 0, timerCount)
	for i, eventDefinition := range eventDefinitions {
		node, _ := graph.node(eventDefinition.BpmnElementId)

		eventDefinition.ElementId = node.id
		eventDefinition.ProcessId = process.Id

		eventDefinition.BpmnElementType = node.bpmnElement.Type
		eventDefinition.BpmnProcessId = process.BpmnProcessId
		eventDefinition.Version = process.Version

		eventDefinitions[i] = eventDefinition

		bpmnElement := node.bpmnElement

		// start message must be unique across all not suspended event definitions
		if bpmnElement.Type == model.ElementMessageStartEvent {
			messageName := eventDefinition.MessageName.String

			eventDefinition, ok, err := ctx.EventDefinitions().SelectByMessageName(messageName)
			if err != nil {
				return engine.Process{}, err
			}

			if ok && eventDefinition.BpmnProcessId != cmd.BpmnProcessId {
				causes = append(causes, engine.ErrorCause{
					Pointer: elementPointer(bpmnElement),
					Type:    "message_event",
					Detail: fmt.Sprintf(
						"start message name %s must be unique - already defined in process %s:%s",
						messageName,
						eventDefinition.BpmnProcessId,
						eventDefinition.Version,
					),
				})
				continue
			}
		}

		// prepare trigger timer event tasks
		if bpmnElement.Type == model.ElementTimerStartEvent {
			timer := eventDefinition.Timer()

			dueAt, err := evaluateTimer(timer, ctx.Time())
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

				BpmnElementId: pgtype.Text{String: bpmnElement.Id, Valid: true},
				CreatedAt:     ctx.Time(),
				CreatedBy:     cmd.WorkerId,
				DueAt:         dueAt,
				State:         engine.WorkCreated,
				Type:          engine.TaskTriggerEvent,

				Instance: TriggerEventTask{Timer: timer},
			})
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
			State:     engine.WorkCreated,
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
	ctx.ProcessCache().Put(ctx, process)

	return process.Process(), nil
}

func GetBpmnXml(ctx Context, cmd engine.GetBpmnXmlCmd) (string, error) {
	if process, ok := ctx.ProcessCache().GetById(ctx, cmd.ProcessId); ok {
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
