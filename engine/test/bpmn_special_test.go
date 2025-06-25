package test

import (
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

func newSpecialTest(t *testing.T, e engine.Engine) specialTest {
	return specialTest{
		e: e,

		startEndTest: mustCreateProcess(t, e, "start-end.bpmn", "startEndTest"),
	}
}

type specialTest struct {
	e engine.Engine

	startEndTest engine.Process
}

func (x specialTest) startEnd(t *testing.T) {
	assert := assert.New(t)

	piAssert := mustCreateProcessInstance(t, x.e, x.startEndTest)
	piAssert.IsCompleted()

	completed := piAssert.ElementInstances(engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceCompleted}})
	assert.Len(completed, 3)
}
