package mem

import (
	"github.com/gclaussn/go-bpmn/engine/internal"
)

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
