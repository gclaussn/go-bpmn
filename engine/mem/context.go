package mem

import (
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
)

func newMemContext(options Options) *memContext {
	ctx := memContext{
		options:      options,
		processCache: internal.NewProcessCache(),
	}

	ctx.elementInstances.partitions = make(map[string][]internal.ElementInstanceEntity)
	ctx.events.partitions = make(map[string][]internal.EventEntity)
	ctx.eventDefinitions.entities = make(map[int32]internal.EventDefinitionEntity)
	ctx.incidents.partitions = make(map[string][]internal.IncidentEntity)
	ctx.jobs.partitions = make(map[string][]internal.JobEntity)
	ctx.messageVariables.variables = make(map[int64][]*internal.MessageVariableEntity)
	ctx.processInstanceQueues.queues = make(map[string]internal.ProcessInstanceQueueEntity)
	ctx.processInstanceQueues.queueElementPartitions = make(map[string][]internal.ProcessInstanceQueueElementEntity)
	ctx.processInstances.partitions = make(map[string][]internal.ProcessInstanceEntity)
	ctx.signalVariables.variables = make(map[int64][]*internal.SignalVariableEntity)
	ctx.tasks.partitions = make(map[string][]internal.TaskEntity)
	ctx.tasks.engineId = options.Common.EngineId
	ctx.variables.partitions = make(map[string][]internal.VariableEntity)

	ctx.elements.eventDefinitions = ctx.eventDefinitions
	ctx.processInstances.elementInstances = ctx.elementInstances
	ctx.processInstances.jobs = ctx.jobs

	return &ctx
}

type memContext struct {
	options Options

	time time.Time

	elements              elementRepository
	elementInstances      elementInstanceRepository
	events                eventRepository
	eventDefinitions      eventDefinitionRepository
	incidents             incidentRepository
	jobs                  jobRepository
	messages              messageRepository
	messageSubscriptions  messageSubscriptionRepository
	messageVariables      messageVariableRepository
	processes             processRepository
	processCache          *internal.ProcessCache
	processInstances      processInstanceRepository
	processInstanceQueues processInstanceQueueRepository
	signals               signalRepository
	signalSubscriptions   signalSubscriptionRepository
	signalVariables       signalVariableRepository
	tasks                 taskRepository
	variables             variableRepository
}

func (c *memContext) Options() engine.Options {
	return c.options.Common
}

func (c *memContext) Date() time.Time {
	return c.time.Truncate(24 * time.Hour)
}

func (c *memContext) Time() time.Time {
	return c.time
}

func (c *memContext) Elements() internal.ElementRepository {
	return &c.elements
}

func (c *memContext) ElementInstances() internal.ElementInstanceRepository {
	return &c.elementInstances
}

func (c *memContext) Events() internal.EventRepository {
	return &c.events
}

func (c *memContext) EventDefinitions() internal.EventDefinitionRepository {
	return &c.eventDefinitions
}

func (c *memContext) Incidents() internal.IncidentRepository {
	return &c.incidents
}

func (c *memContext) Jobs() internal.JobRepository {
	return &c.jobs
}

func (c *memContext) Messages() internal.MessageRepository {
	return &c.messages
}

func (c *memContext) MessageSubscriptions() internal.MessageSubscriptionRepository {
	return &c.messageSubscriptions
}

func (c *memContext) MessageVariables() internal.MessageVariableRepository {
	return &c.messageVariables
}

func (c *memContext) Processes() internal.ProcessRepository {
	return &c.processes
}

func (c *memContext) ProcessCache() *internal.ProcessCache {
	return c.processCache
}

func (c *memContext) ProcessInstances() internal.ProcessInstanceRepository {
	return &c.processInstances
}

func (c *memContext) ProcessInstanceQueues() internal.ProcessInstanceQueueRepository {
	return &c.processInstanceQueues
}

func (c *memContext) Signals() internal.SignalRepository {
	return &c.signals
}

func (c *memContext) SignalSubscriptions() internal.SignalSubscriptionRepository {
	return &c.signalSubscriptions
}

func (c *memContext) SignalVariables() internal.SignalVariableRepository {
	return &c.signalVariables
}

func (c *memContext) Tasks() internal.TaskRepository {
	return &c.tasks
}

func (c *memContext) Variables() internal.VariableRepository {
	return &c.variables
}

func (c *memContext) clear() {
	c.processCache.Clear()

	c.elements.entities = nil
	clear(c.events.partitions)
	c.eventDefinitions.entities = nil
	clear(c.elementInstances.partitions)
	clear(c.incidents.partitions)
	clear(c.jobs.partitions)
	c.messages.entities = nil
	c.messageSubscriptions.entities = nil
	clear(c.messageVariables.variables)
	c.processes.entities = nil
	clear(c.processInstances.partitions)
	clear(c.processInstanceQueues.queues)
	clear(c.processInstanceQueues.queueElementPartitions)
	c.signals.entities = nil
	c.signalSubscriptions.entities = nil
	clear(c.signalVariables.variables)
	clear(c.tasks.partitions)
	clear(c.variables.partitions)
}
