package test

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func TestResolveIncident(t *testing.T) {
	assert := assert.New(t)

	engines, engineTypes := mustCreateEngines(t)
	for _, e := range engines {
		defer e.Shutdown()
	}

	for i, e := range engines {
		t.Run(engineTypes[i]+"returns error when incident not exists", func(t *testing.T) {
			// when
			err := e.ResolveIncident(context.Background(), engine.ResolveIncidentCmd{})

			// then
			assert.IsTypef(engine.Error{}, err, "expected engine error")

			engineErr := err.(engine.Error)
			assert.Equal(engine.ErrorNotFound, engineErr.Type)
			assert.NotEmpty(engineErr.Title)
			assert.NotEmpty(engineErr.Detail)
		})
	}
}
