package pg

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestSetTime(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	t.Run("returns error when time is before engine time", func(t *testing.T) {
		// when
		err := e.SetTime(context.Background(), engine.SetTimeCmd{})

		// then
		assert.IsTypef(engine.Error{}, err, "expected engine error")

		engineErr := err.(engine.Error)
		assert.Equal(engine.ErrorConflict, engineErr.Type)
	})

	t.Run("set time", func(t *testing.T) {
		// given
		newTime := time.Now().AddDate(0, 0, 7).UTC()

		// when
		err := e.SetTime(context.Background(), engine.SetTimeCmd{Time: newTime})

		// then
		assert.Nil(err)

		q := e.CreateQuery()

		tasks, err := q.QueryTasks(context.Background(), engine.TaskCriteria{
			Partition: engine.Partition(newTime.AddDate(0, 0, 1)),

			Type: engine.TaskCreatePartition,
		})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		assert.Len(tasks, 1)

		// then
		tasks, err = q.QueryTasks(context.Background(), engine.TaskCriteria{
			Partition: engine.Partition(newTime.AddDate(0, 0, 2)),

			Type: engine.TaskDetachPartition,
		})
		if err != nil {
			t.Fatalf("failed to query tasks: %v", err)
		}

		assert.Len(tasks, 1)

		// when called again
		time.Sleep(time.Second)
		err = e.SetTime(context.Background(), engine.SetTimeCmd{Time: newTime})

		// then
		assert.IsTypef(engine.Error{}, err, "expected engine error")

		engineErr := err.(engine.Error)
		assert.Equal(engine.ErrorConflict, engineErr.Type)
	})
}
