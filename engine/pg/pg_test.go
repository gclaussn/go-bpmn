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
		// given
		newTime := time.Now().Add(time.Second * -1)

		// when
		_, _, err := e.SetTime(context.Background(), engine.SetTimeCmd{Time: &newTime})

		// then
		assert.IsTypef(engine.Error{}, err, "expected engine error")

		engineErr := err.(engine.Error)
		assert.Equal(engine.ErrorConflict, engineErr.Type)
	})

	t.Run("returns error when no time, time cycle or time duration is specified", func(t *testing.T) {
		_, _, err := e.SetTime(context.Background(), engine.SetTimeCmd{})
		assert.IsTypef(engine.Error{}, err, "expected engine error")

		engineErr := err.(engine.Error)
		assert.Equal(engine.ErrorValidation, engineErr.Type)
	})

	t.Run("set time", func(t *testing.T) {
		// given
		newTime := time.Now().AddDate(0, 0, 7).UTC()

		// when
		new, old, err := e.SetTime(context.Background(), engine.SetTimeCmd{Time: &newTime})

		// then
		assert.Nil(err)
		assert.Equal(new, old.AddDate(0, 0, 7))

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
		_, _, err = e.SetTime(context.Background(), engine.SetTimeCmd{Time: &newTime})

		// then
		assert.IsTypef(engine.Error{}, err, "expected engine error")

		engineErr := err.(engine.Error)
		assert.Equal(engine.ErrorConflict, engineErr.Type)
	})

	t.Run("set time cycle", func(t *testing.T) {
		// given
		cmd := engine.SetTimeCmd{TimeCycle: "0 * * * *"}

		// when
		new, old, err := e.SetTime(context.Background(), cmd)

		// then
		assert.Nil(err)
		assert.NotEqual(new, old)

		// when called again
		time.Sleep(time.Second)
		new, old, err = e.SetTime(context.Background(), cmd)

		// then
		assert.Nil(err)
		assert.NotEqual(new, old)
	})

	t.Run("set time duration", func(t *testing.T) {
		// given
		cmd := engine.SetTimeCmd{TimeDuration: "PT1H"}

		// when
		new1, old1, err := e.SetTime(context.Background(), cmd)

		// then
		assert.Nil(err)
		assert.Equal(new1, old1.Add(time.Hour))

		// when called again
		time.Sleep(time.Second)
		new2, old2, err := e.SetTime(context.Background(), cmd)

		// then
		assert.Nil(err)
		assert.Equal(new2, old2.Add(time.Hour))
	})
}
