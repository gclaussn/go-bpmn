package internal

import (
	"time"

	"github.com/gclaussn/go-bpmn/engine"
)

type Context interface {
	Options() engine.Options

	Date() time.Time
	Time() time.Time

	Elements() ElementRepository
	ElementInstances() ElementInstanceRepository
	Incidents() IncidentRepository
	Jobs() JobRepository
	Processes() ProcessRepository
	ProcessCache() *ProcessCache
	ProcessInstances() ProcessInstanceRepository
	ProcessInstanceQueues() ProcessInstanceQueueRepository
	Signals() SignalRepository
	SignalEvents() SignalEventRepository
	SignalSubscriptions() SignalSubscriptionRepository
	Tasks() TaskRepository
	TimerEvents() TimerEventRepository
	Variables() VariableRepository
}
