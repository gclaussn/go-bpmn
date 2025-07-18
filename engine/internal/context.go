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
	Events() EventRepository
	EventDefinitions() EventDefinitionRepository
	EventVariables() EventVariableRepository
	Incidents() IncidentRepository
	Jobs() JobRepository
	Processes() ProcessRepository
	ProcessCache() *ProcessCache
	ProcessInstances() ProcessInstanceRepository
	ProcessInstanceQueues() ProcessInstanceQueueRepository
	SignalSubscriptions() SignalSubscriptionRepository
	Tasks() TaskRepository
	Variables() VariableRepository
}
