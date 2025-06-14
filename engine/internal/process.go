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
	model, err := model.New(strings.NewReader(process.BpmnXml))
	if err != nil {
		return err
	}

	processElement, err := model.ProcessById(process.BpmnProcessId)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to cache process",
			Detail: fmt.Sprintf("BPMN model has no process %s", process.BpmnProcessId),
		}
	}

	elements, err := ctx.Elements().SelectByProcessId(process.Id)
	if err != nil {
		return err
	}

	graph, err := newGraph(processElement.AllElements(), elements)
	if err != nil {
		return engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to cache process",
			Detail: fmt.Sprintf("BPMN model is invalid: %v", err),
		}
	}

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

	Query(engine.ProcessCriteria, engine.QueryOptions) ([]any, error)
}

func CreateProcess(ctx Context, cmd engine.CreateProcessCmd) (engine.Process, error) {
	md5Hash := md5.New()
	md5Hash.Write([]byte(cmd.BpmnXml))
	bpmnXmlMd5 := hex.EncodeToString(md5Hash.Sum(nil))

	model, err := model.New(strings.NewReader(cmd.BpmnXml))
	if err != nil {
		return engine.Process{}, engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to parse BPMN XML",
			Detail: err.Error(),
		}
	}

	// find process
	processElement, err := model.ProcessById(cmd.BpmnProcessId)
	if err != nil {
		// collect actual BPMN process IDs
		bpmnProcessIds := make([]string, len(model.Definitions.Processes))
		for i := 0; i < len(bpmnProcessIds); i++ {
			bpmnProcessIds[i] = model.Definitions.Processes[i].Id
		}

		return engine.Process{}, engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to find BPMN process element",
			Detail: fmt.Sprintf("BPMN model has no process %s, but [%s]", cmd.BpmnProcessId, strings.Join(bpmnProcessIds, ", ")),
		}
	}

	// validate process
	processElements := processElement.AllElements()
	problems := validateProcess(processElements)
	if len(problems) != 0 {
		return engine.Process{}, engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to validate BPMN process",
			Detail: strings.Join(problems, "; "),
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
	elements := make([]*ElementEntity, len(processElements))
	for i, e := range processElements {
		element := ElementEntity{
			ProcessId: process.Id,

			BpmnElementId:   e.Id,
			BpmnElementName: e.Name,
			BpmnElementType: e.Type,
		}

		elements[i] = &element
	}

	if err := ctx.Elements().Insert(elements); err != nil {
		return engine.Process{}, err
	}

	// cache process
	graph, err := newGraph(processElements, elements)
	if err != nil {
		return engine.Process{}, engine.Error{
			Type:   engine.ErrorProcessModel,
			Title:  "failed to create execution graph",
			Detail: err.Error(),
		}
	}

	process.graph = &graph
	ctx.ProcessCache().Add(process)

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
