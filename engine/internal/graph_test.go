package internal

import (
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestValidateProcess(t *testing.T) {
	assert := assert.New(t)

	t.Run("returns problem when BPMN process is not executable", func(t *testing.T) {
		causes := mustValidateProcess(t, "invalid/process-not-executable.bpmn", "processNotExecutableTest")
		assert.Len(causes, 1)

		assert.Equal("/processNotExecutableTest", causes[0].Pointer)
		assert.NotEmpty(causes[0].Type)
		assert.Contains(causes[0].Detail, "not executable")
	})

	t.Run("returns problem when BPMN element has no ID", func(t *testing.T) {
		causes := mustValidateProcess(t, "invalid/element-without-id.bpmn", "elementWithoutIdTest")
		assert.Len(causes, 1)

		assert.Equal("/elementWithoutIdTest/", causes[0].Pointer)
		assert.NotEmpty(causes[0].Type)
		assert.Equal("BPMN element of type NONE_START_EVENT has no ID", causes[0].Detail)
	})

	t.Run("returns problem when BPMN process contains joining inclusive gateway", func(t *testing.T) {
		causes := mustValidateProcess(t, "invalid/inclusive-gateway-join.bpmn", "inclusiveGatewayJoinTest")
		assert.Len(causes, 1)

		assert.Equal("/inclusiveGatewayJoinTest/join", causes[0].Pointer)
		assert.NotEmpty(causes[0].Type)
		assert.Contains(causes[0].Detail, "joining inclusive gateway", causes[0].Detail)
	})

	t.Run("returns problem when BPMN sequence flow has no source or target", func(t *testing.T) {
		causes := mustValidateProcess(t, "invalid/element-unknown.bpmn", "elementUnknownTest")
		assert.Len(causes, 2)

		assert.Equal("/elementUnknownTest/f1", causes[0].Pointer)
		assert.NotEmpty(causes[0].Type)
		assert.Contains(causes[0].Detail, "no target element")

		assert.Equal("/elementUnknownTest/f2", causes[1].Pointer)
		assert.NotEmpty(causes[1].Type)
		assert.Contains(causes[1].Detail, "no source element")
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

		CreatedAt:      time.Now(),
		CreatedBy:      "test",
		State:          engine.InstanceStarted,
		StateChangedBy: "test",
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
		StateChangedBy:  processInstance.StateChangedBy,
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
