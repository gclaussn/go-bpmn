package mem

import (
	"fmt"

	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type signalRepository struct {
	entities []internal.SignalEntity
}

func (r *signalRepository) Insert(entity *internal.SignalEntity) error {
	entity.Id = int64(len(r.entities) + 1)
	r.entities = append(r.entities, *entity)
	return nil
}

func (r *signalRepository) Select(id int64) (*internal.SignalEntity, error) {
	for _, e := range r.entities {
		if e.Id == id {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *signalRepository) Update(entity *internal.SignalEntity) error {
	for i, e := range r.entities {
		if e.Id == entity.Id {
			r.entities[i] = *entity
			return nil
		}
	}
	return fmt.Errorf("failed to update signal %d: %v", entity.Id, pgx.ErrNoRows)
}

type signalSubscriptionRepository struct {
	entities []internal.SignalSubscriptionEntity
}

func (r *signalSubscriptionRepository) DeleteByName(name string) ([]*internal.SignalSubscriptionEntity, error) {
	var results []*internal.SignalSubscriptionEntity
	for i, e := range r.entities {
		if e.Id == -1 {
			continue // skip deleted signal subscription
		}
		if e.Name != name {
			continue
		}

		results = append(results, &r.entities[i])

		e.Id = -1
		r.entities[i] = e
	}
	return results, nil
}

func (r *signalSubscriptionRepository) Insert(entity *internal.SignalSubscriptionEntity) error {
	entity.Id = int64(len(r.entities) + 1)
	r.entities = append(r.entities, *entity)
	return nil
}

type signalVariableRepository struct {
	variables map[int64][]*internal.SignalVariableEntity
	nextId    int64
}

func (r *signalVariableRepository) InsertBatch(entities []*internal.SignalVariableEntity) error {
	if len(entities) == 0 {
		return nil
	}

	for _, entity := range entities {
		r.nextId++
		entity.Id = r.nextId
	}

	signalId := entities[0].SignalId
	r.variables[signalId] = entities

	return nil
}

func (r *signalVariableRepository) SelectBySignalId(signalId int64) ([]*internal.SignalVariableEntity, error) {
	return r.variables[signalId], nil
}
