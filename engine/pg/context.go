package pg

import (
	"context"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type pgContext struct {
	options Options

	time time.Time

	tx    pgx.Tx
	txCtx context.Context

	processCache *internal.ProcessCache
}

func (c *pgContext) Options() engine.Options {
	return c.options.Common
}

func (c *pgContext) Date() time.Time {
	return c.time.Truncate(24 * time.Hour)
}

func (c *pgContext) Time() time.Time {
	return c.time
}

func (c *pgContext) Elements() internal.ElementRepository {
	return &elementRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) ElementInstances() internal.ElementInstanceRepository {
	return &elementInstanceRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) Events() internal.EventRepository {
	return &eventRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) EventDefinitions() internal.EventDefinitionRepository {
	return &eventDefinitionRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) Incidents() internal.IncidentRepository {
	return &incidentRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) Jobs() internal.JobRepository {
	return &jobRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) Processes() internal.ProcessRepository {
	return &processRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) ProcessCache() *internal.ProcessCache {
	return c.processCache
}

func (c *pgContext) ProcessInstances() internal.ProcessInstanceRepository {
	return &processInstanceRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) ProcessInstanceQueues() internal.ProcessInstanceQueueRepository {
	return &processInstanceQueueRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) Signals() internal.SignalRepository {
	return &signalRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) SignalSubscriptions() internal.SignalSubscriptionRepository {
	return &signalSubscriptionRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) SignalVariables() internal.SignalVariableRepository {
	return &signalVariableRepository{tx: c.tx, txCtx: c.txCtx}
}

func (c *pgContext) Tasks() internal.TaskRepository {
	return &taskRepository{tx: c.tx, txCtx: c.txCtx, engineId: c.Options().EngineId}
}

func (c *pgContext) Variables() internal.VariableRepository {
	return &variableRepository{tx: c.tx, txCtx: c.txCtx}
}
