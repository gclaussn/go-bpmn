package internal

import (
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/model"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateProcess(t *testing.T) {
	assert := assert.New(t)

	t.Run("returns cause when BPMN process is not executable", func(t *testing.T) {
		causes := mustValidateProcess(t, "invalid/process-not-executable.bpmn")
		assert.Len(causes, 1)

		assert.Equal("/processNotExecutableTest", causes[0].Pointer)
		assert.NotEmpty(causes[0].Type)
		assert.Contains(causes[0].Detail, "not executable")
	})

	t.Run("returns cause when BPMN element has no ID", func(t *testing.T) {
		causes := mustValidateProcess(t, "start-end.bpmn", func(processElement *model.Element) {
			startEvent := processElement.ChildById("startEvent")
			startEvent.Id = ""
		})
		assert.Len(causes, 1)

		assert.Equal("/startEndTest/", causes[0].Pointer)
		assert.NotEmpty(causes[0].Type)
		assert.Equal("BPMN element of type NONE_START_EVENT has no ID", causes[0].Detail)
	})

	t.Run("exclusive gateway", func(t *testing.T) {
		t.Run("returns cause when default sequence flow not exists", func(t *testing.T) {
			causes := mustValidateProcess(t, "gateway/exclusive-default.bpmn", func(processElement *model.Element) {
				fork := processElement.ChildById("fork")
				fork.Model = model.ExclusiveGateway{
					Default: "notExisting",
				}
			})
			assert.Len(causes, 1)

			assert.Equal("/exclusiveDefaultTest/fork", causes[0].Pointer)
			assert.NotEmpty(causes[0].Type)
			assert.Equal("exclusive gateway fork has no default sequence flow notExisting", causes[0].Detail)
		})

		t.Run("returns cause when default sequence flow is incoming", func(t *testing.T) {
			causes := mustValidateProcess(t, "gateway/exclusive-default.bpmn", func(processElement *model.Element) {
				fork := processElement.ChildById("fork")
				fork.Model = model.ExclusiveGateway{
					Default: "f1",
				}
			})
			assert.Len(causes, 1)

			assert.Equal("/exclusiveDefaultTest/fork", causes[0].Pointer)
			assert.NotEmpty(causes[0].Type)
			assert.Equal("exclusive gateway fork has no default sequence flow f1", causes[0].Detail)
		})
	})

	t.Run("inclusive gateway", func(t *testing.T) {
		t.Run("returns cause when default sequence flow not exists", func(t *testing.T) {
			causes := mustValidateProcess(t, "gateway/inclusive-default.bpmn", func(processElement *model.Element) {
				fork := processElement.ChildById("fork")
				fork.Model = model.InclusiveGateway{
					Default: "notExisting",
				}
			})
			assert.Len(causes, 1)

			assert.Equal("/inclusiveDefaultTest/fork", causes[0].Pointer)
			assert.NotEmpty(causes[0].Type)
			assert.Equal("inclusive gateway fork has no default sequence flow notExisting", causes[0].Detail)
		})

		t.Run("returns cause when default sequence flow is incoming", func(t *testing.T) {
			causes := mustValidateProcess(t, "gateway/inclusive-default.bpmn", func(processElement *model.Element) {
				fork := processElement.ChildById("fork")
				fork.Model = model.InclusiveGateway{
					Default: "f1",
				}
			})
			assert.Len(causes, 1)

			assert.Equal("/inclusiveDefaultTest/fork", causes[0].Pointer)
			assert.NotEmpty(causes[0].Type)
			assert.Equal("inclusive gateway fork has no default sequence flow f1", causes[0].Detail)
		})

		t.Run("returns cause when BPMN process contains joining gateway", func(t *testing.T) {
			causes := mustValidateProcess(t, "invalid/inclusive-gateway-join.bpmn")
			assert.Len(causes, 1)

			assert.Equal("/inclusiveGatewayJoinTest/join", causes[0].Pointer)
			assert.NotEmpty(causes[0].Type)
			assert.Contains(causes[0].Detail, "joining inclusive gateway", causes[0].Detail)
		})
	})

	t.Run("returns cause when BPMN sequence flow has no source or target", func(t *testing.T) {
		causes := mustValidateProcess(t, "invalid/unknown-element.bpmn")
		assert.Len(causes, 2)

		assert.Equal("/unknownElementTest/f1", causes[0].Pointer)
		assert.NotEmpty(causes[0].Type)
		assert.Contains(causes[0].Detail, "no target element")

		assert.Equal("/unknownElementTest/f2", causes[1].Pointer)
		assert.NotEmpty(causes[1].Type)
		assert.Contains(causes[1].Detail, "no source element")
	})
}

func TestContinueExecution(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	scope := &ElementInstanceEntity{}

	t.Run("start event", func(t *testing.T) {
		// given
		graph := mustCreateGraph(t, "start-end.bpmn", "startEndTest")

		t.Run("scope QUEUED", func(t *testing.T) {
			// given
			scope.State = engine.InstanceQueued

			execution := &ElementInstanceEntity{
				BpmnElementId:   "startEvent",
				BpmnElementType: model.ElementNoneStartEvent,

				parent: scope,
			}

			// when
			executions1, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions1, 0)

			assert.Equal(engine.InstanceQueued, execution.State)

			// when
			scope.State = engine.InstanceStarted

			executions2, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions2, 1)

			assert.Equal(engine.InstanceCompleted, execution.State)

			assert.Equal("endEvent", executions2[0].BpmnElementId)
			assert.Equal(model.ElementNoneEndEvent, executions2[0].BpmnElementType)
			assert.Equal(scope, executions2[0].parent)
			assert.Nil(executions2[0].prev)
		})
	})

	t.Run("service task with boundary event", func(t *testing.T) {
		// given
		graph := mustCreateGraph(t, "event/error-boundary-event.bpmn", "errorBoundaryEventTest")

		t.Run("scope STARTED", func(t *testing.T) {
			// given
			scope.State = engine.InstanceStarted

			execution := &ElementInstanceEntity{
				BpmnElementId:   "serviceTask",
				BpmnElementType: model.ElementServiceTask,

				parent: scope,
			}

			// when
			executions1, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions1, 1)

			assert.Equal(engine.InstanceCreated, execution.State)
			assert.Equal(-1, execution.ExecutionCount)

			assert.Equal("errorBoundaryEvent", executions1[0].BpmnElementId)
			assert.Equal(model.ElementErrorBoundaryEvent, executions1[0].BpmnElementType)
			assert.Equal(scope, executions1[0].parent)
			assert.Equal(execution, executions1[0].prev)

			// when
			executions2, err := graph.continueExecution(nil, executions1[0])

			// then
			require.NoError(err)
			require.Len(executions2, 0)

			assert.Equal(engine.InstanceCreated, executions1[0].State)
		})

		t.Run("scope SUSPENDED", func(t *testing.T) {
			// given
			scope.State = engine.InstanceSuspended

			execution := &ElementInstanceEntity{
				BpmnElementId:   "serviceTask",
				BpmnElementType: model.ElementServiceTask,

				parent: scope,
			}

			// when
			executions1, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions1, 1)

			assert.Equal(engine.InstanceSuspended, execution.State)
			assert.Equal(-1, execution.ExecutionCount)

			assert.Equal("errorBoundaryEvent", executions1[0].BpmnElementId)
			assert.Equal(model.ElementErrorBoundaryEvent, executions1[0].BpmnElementType)
			assert.Equal(scope, executions1[0].parent)
			assert.Equal(execution, executions1[0].prev)

			// when
			executions2, err := graph.continueExecution(nil, executions1[0])

			// then
			require.NoError(err)
			require.Len(executions2, 0)

			assert.Equal(engine.InstanceSuspended, executions1[0].State)

			// when
			scope.State = engine.InstanceStarted

			executions3, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions3, 0)

			assert.Equal(engine.InstanceCreated, execution.State)
			assert.Equal(-1, execution.ExecutionCount)

			// when
			executions4, err := graph.continueExecution(nil, executions1[0])

			// then
			require.NoError(err)
			require.Len(executions4, 0)

			assert.Equal(engine.InstanceCreated, executions1[0].State)
		})

		t.Run("with event definition", func(t *testing.T) {
			// given
			graph.setEventDefinitions([]*EventDefinitionEntity{
				{ElementId: graph.nodes["errorBoundaryEvent"].id, ErrorCode: pgtype.Text{String: "TEST_CODE"}},
			})

			scope.State = engine.InstanceStarted

			execution := &ElementInstanceEntity{
				BpmnElementId:   "serviceTask",
				BpmnElementType: model.ElementServiceTask,

				parent: scope,
			}

			// when
			executions, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions, 1)

			assert.Equal(engine.InstanceStarted, execution.State)
			assert.Equal(0, execution.ExecutionCount)

			assert.Equal("errorBoundaryEvent", executions[0].BpmnElementId)
			assert.Equal(model.ElementErrorBoundaryEvent, executions[0].BpmnElementType)
			assert.Equal("TEST_CODE", executions[0].Context.String)
			assert.True(executions[0].Context.Valid)
		})
	})

	t.Run("created service task with boundary event", func(t *testing.T) {
		// given
		graph := mustCreateGraph(t, "event/error-boundary-event.bpmn", "errorBoundaryEventTest")

		t.Run("scope STARTED", func(t *testing.T) {
			// given
			scope.State = engine.InstanceStarted

			execution := &ElementInstanceEntity{
				BpmnElementId:   "serviceTask",
				BpmnElementType: model.ElementServiceTask,
				State:           engine.InstanceCreated,

				parent: scope,
			}

			// when
			executions, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions, 0)

			assert.Equal(engine.InstanceStarted, execution.State)
			assert.Equal(0, execution.ExecutionCount)
		})

		t.Run("scope STARTED, but not all boundary events defined", func(t *testing.T) {
			// given
			scope.State = engine.InstanceStarted

			execution := &ElementInstanceEntity{
				BpmnElementId:   "serviceTask",
				BpmnElementType: model.ElementServiceTask,
				ExecutionCount:  -1,
				State:           engine.InstanceCreated,

				parent: scope,
			}

			// when
			executions, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions, 0)

			assert.Equal(engine.InstanceCreated, execution.State)
			assert.Equal(-1, execution.ExecutionCount)
		})

		t.Run("scope SUSPENDED, but not all boundary events defined", func(t *testing.T) {
			// given
			scope.State = engine.InstanceSuspended

			execution := &ElementInstanceEntity{
				BpmnElementId:   "serviceTask",
				BpmnElementType: model.ElementServiceTask,
				ExecutionCount:  -1,
				State:           engine.InstanceCreated,

				parent: scope,
			}

			// when
			executions1, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions1, 0)

			assert.Equal(engine.InstanceSuspended, execution.State)
			assert.Equal(-1, execution.ExecutionCount)

			// when
			scope.State = engine.InstanceStarted

			executions2, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions2, 0)

			assert.Equal(engine.InstanceCreated, execution.State)
			assert.Equal(-1, execution.ExecutionCount)
		})

		t.Run("SUSPENDED", func(t *testing.T) {
			// given
			scope.State = engine.InstanceStarted

			execution := &ElementInstanceEntity{
				BpmnElementId:   "serviceTask",
				BpmnElementType: model.ElementServiceTask,
				State:           engine.InstanceSuspended,

				parent: scope,
			}

			// when
			executions, err := graph.continueExecution(nil, execution)

			// then
			require.NoError(err)
			require.Len(executions, 0)

			assert.Equal(engine.InstanceStarted, execution.State)
			assert.Equal(0, execution.ExecutionCount)
		})
	})
}

func TestCreateExecution(t *testing.T) {
	assert := assert.New(t)

	t.Run("create execution", func(t *testing.T) {
		// given
		graph := mustCreateGraph(t, "task/service.bpmn", "serviceTest")

		startEvent := graph.nodes["startEvent"]

		processInstance := ProcessInstanceEntity{
			Partition: time.Now(),
			Id:        10,

			ProcessId: 1,
		}

		scope := ElementInstanceEntity{
			Partition: processInstance.Partition,
			Id:        100,

			ProcessId:         processInstance.ProcessId,
			ProcessInstanceId: processInstance.Id,

			BpmnElementId: "serviceTest",
			State:         engine.InstanceStarted,
		}

		// when
		execution, err := graph.createExecution(&scope)
		if err != nil {
			t.Fatalf("failed to create execution: %v", err)
		}

		// then
		assert.Equal(ElementInstanceEntity{
			Partition: processInstance.Partition,

			ElementId:         startEvent.id,
			ProcessId:         processInstance.ProcessId,
			ProcessInstanceId: processInstance.Id,

			BpmnElementId:   startEvent.bpmnElement.Id,
			BpmnElementType: startEvent.bpmnElement.Type,

			parent: &scope,
		}, execution)
	})

	t.Run("returns error when none start event not exists", func(t *testing.T) {
		// given
		graph := mustCreateGraph(t, "empty.bpmn", "emptyTest")

		scope := ElementInstanceEntity{BpmnElementId: "emptyTest", State: engine.InstanceStarted}

		// when
		execution, err := graph.createExecution(&scope)

		// then
		assert.Equal(engine.MapInstanceState(""), execution.State)
		assert.Contains(err.Error(), "has no none start event element")
	})
}

func TestCreateProcessScope(t *testing.T) {
	assert := assert.New(t)

	// given
	graph := mustCreateGraph(t, "task/service.bpmn", "serviceTest")

	node := graph.nodes["serviceTest"]

	processInstance := ProcessInstanceEntity{
		Partition: time.Now(),
		Id:        10,

		ProcessId: 1,

		CreatedAt: time.Now(),
		CreatedBy: "test",
		State:     engine.InstanceStarted,
	}

	// when
	scope := graph.createProcessScope(&processInstance)

	// then
	assert.Equal(ElementInstanceEntity{
		Partition: processInstance.Partition,

		ElementId:         node.id,
		ProcessId:         processInstance.ProcessId,
		ProcessInstanceId: processInstance.Id,

		BpmnElementId:   node.bpmnElement.Id,
		BpmnElementType: node.bpmnElement.Type,
		CreatedAt:       processInstance.CreatedAt,
		CreatedBy:       processInstance.CreatedBy,
		State:           engine.InstanceStarted,
	}, scope)
}

func TestJoinParallelGateway(t *testing.T) {
	assert := assert.New(t)

	graph := mustCreateGraph(t, "gateway/parallel-service-tasks.bpmn", "parallelServiceTasksTest")

	t.Run("no join when there is only one execution", func(t *testing.T) {
		// given
		executions := []*ElementInstanceEntity{{}}

		// when
		joinedExecutions, err := graph.joinParallelGateway(executions)
		if err != nil {
			t.Fatalf("failed to join parallel gateways: %v", err)
		}

		// then
		assert.Empty(joinedExecutions)
	})

	t.Run("returns error when BPMN process has no such element", func(t *testing.T) {
		// given
		executions := []*ElementInstanceEntity{
			{BpmnElementId: "not-existing-a"},
			{BpmnElementId: "not-existing-b"},
		}

		// when
		joinedExecutions, err := graph.joinParallelGateway(executions)

		// then
		assert.Empty(joinedExecutions)
		assert.Contains(err.Error(), "not-existing-a")
	})

	t.Run("returns error when BPMN element is not a joining parallel gateway", func(t *testing.T) {
		// given
		executions := []*ElementInstanceEntity{
			{BpmnElementId: "fork", ElementId: 3},
			{BpmnElementId: "join", ElementId: 6},
		}

		// when
		joinedExecutions, err := graph.joinParallelGateway(executions)

		// then
		assert.Empty(joinedExecutions)
		assert.Contains(err.Error(), "not a joining parallel gateway")
	})

	t.Run("returns error when not all executions have the same element ID", func(t *testing.T) {
		// given
		executions := []*ElementInstanceEntity{
			{BpmnElementId: "join", ElementId: 6},
			{BpmnElementId: "fork", ElementId: 3},
		}

		// when
		joinedExecutions, err := graph.joinParallelGateway(executions)

		// then
		assert.Empty(joinedExecutions)
		assert.Contains(err.Error(), "6")
	})

	t.Run("no join when not all incoming sequence flows satisfied", func(t *testing.T) {
		// given
		executions := []*ElementInstanceEntity{
			{Id: 1, BpmnElementId: "join", ElementId: 6, PrevElementId: pgtype.Int4{Int32: 4}},
			{Id: 2, BpmnElementId: "join", ElementId: 6, PrevElementId: pgtype.Int4{Int32: 4}},
		}

		// when
		joinedExecutions, err := graph.joinParallelGateway(executions)
		if err != nil {
			t.Fatalf("failed to join parallel gateways: %v", err)
		}

		// then
		assert.Empty(joinedExecutions)
	})

	t.Run("join", func(t *testing.T) {
		// given
		executions := []*ElementInstanceEntity{
			{Id: 1, BpmnElementId: "join", ElementId: 6, PrevElementId: pgtype.Int4{Int32: 4}},
			{Id: 2, BpmnElementId: "join", ElementId: 6, PrevElementId: pgtype.Int4{Int32: 4}},
			{Id: 3, BpmnElementId: "join", ElementId: 6, PrevElementId: pgtype.Int4{Int32: 5}},
		}

		// when
		joinedExecutions, err := graph.joinParallelGateway(executions)
		if err != nil {
			t.Fatalf("failed to join parallel gateways: %v", err)
		}

		// then
		assert.Len(joinedExecutions, 2)
		assert.Equal(int32(1), joinedExecutions[0].Id)
		assert.Equal(engine.InstanceStarted, joinedExecutions[0].State)
		assert.Equal(int32(3), joinedExecutions[1].Id)
		assert.Equal(engine.InstanceCompleted, joinedExecutions[1].State)
	})
}
