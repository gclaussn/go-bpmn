package mem

import (
	"fmt"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5"
)

type messageRepository struct {
	entities []internal.MessageEntity
}

func (r *messageRepository) Insert(entity *internal.MessageEntity) error {
	if entity.UniqueKey.Valid {
		// try to find a conflicting message
		for i, e := range r.entities {
			if e.Name != entity.Name || e.CorrelationKey != entity.CorrelationKey || e.UniqueKey.String != entity.UniqueKey.String {
				continue
			}

			if !e.IsCorrelated || !e.ExpiresAt.Valid {
				e.IsConflict = true
				r.entities[i] = e
				*entity = e
				return nil
			}
		}
	}

	entity.Id = int64(len(r.entities) + 1)
	r.entities = append(r.entities, *entity)
	return nil
}

func (r *messageRepository) Select(id int64) (*internal.MessageEntity, error) {
	for _, e := range r.entities {
		if e.Id == id {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *messageRepository) SelectBuffered(name string, correlationKey string, now time.Time) (*internal.MessageEntity, error) {
	for _, e := range r.entities {
		if e.IsCorrelated {
			continue
		}

		if e.ExpiresAt.Valid && !now.Before(e.ExpiresAt.Time) {
			continue // skip expired message
		}

		if e.Name == name && e.CorrelationKey == correlationKey {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (r *messageRepository) Update(entity *internal.MessageEntity) error {
	for i, e := range r.entities {
		if e.Id == entity.Id {
			r.entities[i] = *entity
			return nil
		}
	}
	return fmt.Errorf("failed to update message %d: %v", entity.Id, pgx.ErrNoRows)
}

func (r *messageRepository) Query(c engine.MessageCriteria, o engine.QueryOptions, now time.Time) ([]engine.Message, error) {
	var (
		offset int
		limit  int
	)

	results := make([]engine.Message, 0)
	for _, e := range r.entities {
		if c.Id != 0 && c.Id != e.Id {
			continue
		}

		if c.ExcludeExpired && (e.ExpiresAt.Valid && !now.Before(e.ExpiresAt.Time)) {
			continue
		}
		if c.Name != "" && c.Name != e.Name {
			continue
		}

		if offset < o.Offset {
			offset++
			continue
		}

		results = append(results, e.Message())
		limit++

		if o.Limit > 0 && limit == o.Limit {
			break
		}
	}

	return results, nil
}

type messageSubscriptionRepository struct {
	entities []internal.MessageSubscriptionEntity
}

func (r *messageSubscriptionRepository) Delete(entity *internal.MessageSubscriptionEntity) error {
	e := r.entities[entity.Id-1]
	e.Id = -1
	r.entities[entity.Id-1] = e
	return nil
}

func (r *messageSubscriptionRepository) Insert(entity *internal.MessageSubscriptionEntity) error {
	entity.Id = int64(len(r.entities) + 1)
	r.entities = append(r.entities, *entity)
	return nil
}

func (r *messageSubscriptionRepository) SelectByNameAndCorrelationKey(name string, correlationKey string) (*internal.MessageSubscriptionEntity, error) {
	for _, e := range r.entities {
		if e.Id == -1 {
			continue // skip deleted message subscription
		}
		if e.Name == name && e.CorrelationKey == correlationKey {
			return &e, nil
		}
	}
	return nil, pgx.ErrNoRows
}

type messageVariableRepository struct {
	variables map[int64][]*internal.MessageVariableEntity
	nextId    int64
}

func (r *messageVariableRepository) InsertBatch(entities []*internal.MessageVariableEntity) error {
	if len(entities) == 0 {
		return nil
	}

	for _, entity := range entities {
		r.nextId++
		entity.Id = r.nextId
	}

	messageId := entities[0].MessageId
	r.variables[messageId] = entities

	return nil
}

func (r *messageVariableRepository) SelectByMessageId(messageId int64) ([]*internal.MessageVariableEntity, error) {
	return r.variables[messageId], nil
}
