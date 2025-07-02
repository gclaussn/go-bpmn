package pg

import (
	"context"

	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type signalRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

type signalEventRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}

func (r signalEventRepository) Insert(entities []*internal.SignalEventEntity) error {
}

func (r signalEventRepository) Select(elementId int32) (*internal.SignalEventEntity, error) {

}

func (r signalEventRepository) SelectByBpmnProcessId(bpmnProcessId string) ([]*internal.SignalEventEntity, error) {

}

func (r signalEventRepository) Update(entities []*SignalEventEntity) error {

}

type signalSubscriptionRepository struct {
	tx    pgx.Tx
	txCtx context.Context
}
