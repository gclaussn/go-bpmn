package mem

import (
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
		err := e.SetTime(engine.SetTimeCmd{})
		assert.IsTypef(engine.Error{}, err, "expected engine error")

		engineErr := err.(engine.Error)
		assert.Equal(engine.ErrorConflict, engineErr.Type)
	})

	t.Run("set time", func(t *testing.T) {
		// given
		newTime := time.Now().Add(time.Hour).UTC()

		// when
		err := e.SetTime(engine.SetTimeCmd{Time: newTime})

		// then
		assert.Nil(err)

		// when called again
		time.Sleep(time.Second)
		err = e.SetTime(engine.SetTimeCmd{Time: newTime})

		// then
		assert.IsTypef(engine.Error{}, err, "expected engine error")

		engineErr := err.(engine.Error)
		assert.Equal(engine.ErrorConflict, engineErr.Type)
	})
}
