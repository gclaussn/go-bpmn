package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

type specialTest struct {
	e engine.Engine
}

func (x specialTest) startEnd(t *testing.T) {
	assert := assert.New(t)

	process := mustCreateProcess(t, x.e, "start-end.bpmn", "startEndTest")

	piAssert, _ := mustCreateProcessInstance(t, x.e, process)
	piAssert.IsCompleted()

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 3)
}
