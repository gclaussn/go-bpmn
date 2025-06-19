package pg

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// !keep in sync with engine/mem/query_test.go
func TestQuery(t *testing.T) {
	e := mustCreateEngine(t)
	defer e.Shutdown()

	pgEngine := e.(*pgEngine)
	w, cancel := pgEngine.withTimeout()

	ctx, err := w.require()
	if err != nil {
		cancel()
		t.Fatalf("failed to require context: %v", err)
	}

	// delete management tasks, so that task related tests are not affected
	if _, err := ctx.tx.Exec(ctx.txCtx, "DELETE FROM task"); err != nil {
		w.release(ctx, err)
		cancel()
		t.Fatalf("failed to delete tasks: %v", err)
	}

	w.release(ctx, nil)
	cancel()

	var (
		date      = time.Now().UTC().Truncate(24 * time.Hour)
		datePlus1 = date.AddDate(0, 0, 1)

		partitions = []time.Time{date, date, date, datePlus1, datePlus1}

		elementInstanceIds = []int32{1, 2, 2, 3, 4}
		jobIds             = []int32{1, 2, 3, 0, 0}
		processIds         = []int32{1, 1, 1, 2, 2}
		processInstanceIds = []int32{10, 10, 10, 20, 20}
		taskIds            = []int32{0, 0, 0, 4, 5}

		// element instance
		bpmnElementIds = []string{"a", "b", "c", "a", "d"}

		states = []engine.InstanceState{
			engine.InstanceEnded,
			engine.InstanceEnded,
			engine.InstanceStarted,
			engine.InstanceQueued,
			engine.InstanceQueued,
		}

		// process + process instance
		tags = []map[string]string{
			{
				"a": "b",
			},
			nil,
			{
				"c": "d",
			},
			nil,
			{
				"a": "b",
				"x": "y",
			},
		}

		// task
		taskElementInstanceIds = []int32{0, 2, 0, 3, 4}
		taskProcessIds         = []int32{0, 1, 0, 2, 2}
		taskProcessInstanceIds = []int32{0, 10, 0, 20, 20}
		taskTypes              = []engine.TaskType{
			engine.TaskCreatePartition,
			engine.TaskDetachPartition,
			engine.TaskDropPartition,
			engine.TaskDetachPartition,
			engine.TaskCreatePartition,
		}

		// variable
		variableElementInstanceIds = []int32{0, 2, 0, 3, 3}
		variableProcessInstanceIds = []int32{10, 10, 10, 20, 20}
		variableNames              = []string{"a", "b", "c", "a", "b"}
	)

	var entities []any
	for i := range partitions {
		var tagsJson string
		if len(tags[i]) != 0 {
			b, _ := json.Marshal(tags[i])
			tagsJson = string(b)
		}

		entities = append(entities, &internal.ElementEntity{
			ProcessId: processIds[i],
		})

		entities = append(entities, &internal.ElementInstanceEntity{
			Partition: partitions[i],

			ProcessId:         processIds[i],
			ProcessInstanceId: processInstanceIds[i],

			BpmnElementId: bpmnElementIds[i],
			State:         states[i],
		})

		entities = append(entities, &internal.IncidentEntity{
			Partition: partitions[i],

			JobId:             pgtype.Int4{Int32: jobIds[i], Valid: jobIds[i] != 0},
			ProcessInstanceId: pgtype.Int4{Int32: processInstanceIds[i], Valid: processInstanceIds[i] != 0},
			TaskId:            pgtype.Int4{Int32: taskIds[i], Valid: taskIds[i] != 0},
		})

		entities = append(entities, &internal.JobEntity{
			Partition: partitions[i],

			ElementInstanceId: elementInstanceIds[i],
			ProcessId:         processIds[i],
			ProcessInstanceId: processInstanceIds[i],
		})

		entities = append(entities, &internal.ProcessEntity{
			Tags:    pgtype.Text{String: tagsJson, Valid: tagsJson != ""},
			Version: strconv.Itoa(i),
		})

		entities = append(entities, &internal.ProcessInstanceEntity{
			Partition: partitions[i],

			ProcessId: processIds[i],

			Tags: pgtype.Text{String: tagsJson, Valid: tagsJson != ""},
		})

		entities = append(entities, &internal.TaskEntity{
			Partition: partitions[i],

			ElementInstanceId: pgtype.Int4{Int32: taskElementInstanceIds[i], Valid: taskElementInstanceIds[i] != 0},
			ProcessId:         pgtype.Int4{Int32: taskProcessIds[i], Valid: taskProcessIds[i] != 0},
			ProcessInstanceId: pgtype.Int4{Int32: taskProcessInstanceIds[i], Valid: taskProcessInstanceIds[i] != 0},

			Type: taskTypes[i],
		})

		entities = append(entities, &internal.VariableEntity{
			Partition: partitions[i],

			ElementInstanceId: pgtype.Int4{Int32: variableElementInstanceIds[i], Valid: variableElementInstanceIds[i] != 0},
			ProcessInstanceId: variableProcessInstanceIds[i],

			Name: variableNames[i],
		})
	}

	mustInsertEntities(t, e, entities)

	t.Run("options", func(t *testing.T) {
		criterias := []any{
			engine.ElementCriteria{},
			engine.ElementInstanceCriteria{},
			engine.IncidentCriteria{},
			engine.JobCriteria{},
			engine.ProcessCriteria{},
			engine.ProcessInstanceCriteria{},
			engine.TaskCriteria{},
			engine.VariableCriteria{},
		}

		for _, criteria := range criterias {
			runQueryOptionTests(t, e, criteria, []queryOptionTest{
				{"all", engine.QueryOptions{}, 5},
				{"limit", engine.QueryOptions{Limit: 3}, 3},
				{"limit and offset", engine.QueryOptions{Limit: 3, Offset: 4}, 1},
				{"offset", engine.QueryOptions{Offset: 2}, 3},
			})
		}
	})

	t.Run("element", func(t *testing.T) {
		runQueryTests(t, e, []queryTest{
			{
				"by process ID",
				engine.ElementCriteria{ProcessId: 2},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal(int32(2), results[0].(engine.Element).ProcessId)
					assert.Equal(int32(2), results[0].(engine.Element).ProcessId)
				},
			},
		})
	})

	t.Run("element instance", func(t *testing.T) {
		runQueryTests(t, e, []queryTest{
			{
				"by partition",
				engine.ElementInstanceCriteria{Partition: engine.Partition(date)},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(engine.Partition(date), results[0].(engine.ElementInstance).Partition)
					assert.Equal(int32(1), results[0].(engine.ElementInstance).Id)
					assert.Equal(engine.Partition(date), results[1].(engine.ElementInstance).Partition)
					assert.Equal(int32(2), results[1].(engine.ElementInstance).Id)
					assert.Equal(engine.Partition(date), results[2].(engine.ElementInstance).Partition)
					assert.Equal(int32(3), results[2].(engine.ElementInstance).Id)
				},
			},
			{
				"by partition and ID",
				engine.ElementInstanceCriteria{Partition: engine.Partition(date), Id: 1},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(engine.Partition(date), results[0].(engine.ElementInstance).Partition)
					assert.Equal(int32(1), results[0].(engine.ElementInstance).Id)
				},
			},
			{
				"by process ID",
				engine.ElementInstanceCriteria{ProcessId: 2},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal(int32(2), results[0].(engine.ElementInstance).ProcessId)
					assert.Equal(int32(2), results[0].(engine.ElementInstance).ProcessId)
				},
			},
			{
				"by process instance ID",
				engine.ElementInstanceCriteria{ProcessInstanceId: 10},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(int32(10), results[0].(engine.ElementInstance).ProcessInstanceId)
					assert.Equal(int32(10), results[1].(engine.ElementInstance).ProcessInstanceId)
					assert.Equal(int32(10), results[2].(engine.ElementInstance).ProcessInstanceId)
				},
			},
			{
				"by BPMN element ID",
				engine.ElementInstanceCriteria{BpmnElementId: "a"},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal("a", results[0].(engine.ElementInstance).BpmnElementId)
					assert.Equal("a", results[1].(engine.ElementInstance).BpmnElementId)
				},
			},
			{
				"by states",
				engine.ElementInstanceCriteria{States: []engine.InstanceState{engine.InstanceQueued, engine.InstanceStarted}},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(engine.InstanceStarted, results[0].(engine.ElementInstance).State)
					assert.Equal(engine.InstanceQueued, results[1].(engine.ElementInstance).State)
					assert.Equal(engine.InstanceQueued, results[2].(engine.ElementInstance).State)
				},
			},
		})
	})

	t.Run("incident", func(t *testing.T) {
		runQueryTests(t, e, []queryTest{
			{
				"by partition",
				engine.IncidentCriteria{Partition: engine.Partition(date)},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(engine.Partition(date), results[0].(engine.Incident).Partition)
					assert.Equal(int32(1), results[0].(engine.Incident).Id)
					assert.Equal(engine.Partition(date), results[1].(engine.Incident).Partition)
					assert.Equal(int32(2), results[1].(engine.Incident).Id)
					assert.Equal(engine.Partition(date), results[2].(engine.Incident).Partition)
					assert.Equal(int32(3), results[2].(engine.Incident).Id)
				},
			},
			{
				"by partition and ID",
				engine.IncidentCriteria{Partition: engine.Partition(date), Id: 1},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(engine.Partition(date), results[0].(engine.Incident).Partition)
					assert.Equal(int32(1), results[0].(engine.Incident).Id)
				},
			},
			{
				"by job ID",
				engine.IncidentCriteria{JobId: 2},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(int32(2), results[0].(engine.Incident).JobId)
				},
			},
			{
				"by process instance ID",
				engine.IncidentCriteria{ProcessInstanceId: 10},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(int32(10), results[0].(engine.Incident).ProcessInstanceId)
					assert.Equal(int32(10), results[1].(engine.Incident).ProcessInstanceId)
					assert.Equal(int32(10), results[2].(engine.Incident).ProcessInstanceId)
				},
			},
			{
				"by task ID",
				engine.IncidentCriteria{TaskId: 5},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(int32(5), results[0].(engine.Incident).TaskId)
				},
			},
		})
	})

	t.Run("job", func(t *testing.T) {
		runQueryTests(t, e, []queryTest{
			{
				"by partition",
				engine.JobCriteria{Partition: engine.Partition(date)},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(engine.Partition(date), results[0].(engine.Job).Partition)
					assert.Equal(int32(1), results[0].(engine.Job).Id)
					assert.Equal(engine.Partition(date), results[1].(engine.Job).Partition)
					assert.Equal(int32(1), results[0].(engine.Job).Id)
					assert.Equal(engine.Partition(date), results[2].(engine.Job).Partition)
					assert.Equal(int32(3), results[2].(engine.Job).Id)
				},
			},
			{
				"by partition and ID",
				engine.JobCriteria{Partition: engine.Partition(date), Id: 1},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(engine.Partition(date), results[0].(engine.Job).Partition)
					assert.Equal(int32(1), results[0].(engine.Job).Id)
				},
			},
			{
				"by element instance ID",
				engine.JobCriteria{ElementInstanceId: 3},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(int32(3), results[0].(engine.Job).ElementInstanceId)
				},
			},
			{
				"by process ID",
				engine.JobCriteria{ProcessId: 2},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal(int32(2), results[0].(engine.Job).ProcessId)
					assert.Equal(int32(2), results[1].(engine.Job).ProcessId)
				},
			},
			{
				"by process instance ID",
				engine.JobCriteria{ProcessInstanceId: 10},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(int32(10), results[0].(engine.Job).ProcessInstanceId)
					assert.Equal(int32(10), results[1].(engine.Job).ProcessInstanceId)
					assert.Equal(int32(10), results[2].(engine.Job).ProcessInstanceId)
				},
			},
		})
	})

	t.Run("process", func(t *testing.T) {
		runQueryTests(t, e, []queryTest{
			{
				"by ID",
				engine.ProcessCriteria{Id: 3},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(int32(3), results[0].(engine.Process).Id)
				},
			},
			{
				"by tags",
				engine.ProcessCriteria{Tags: map[string]string{"a": "b"}},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal(int32(1), results[0].(engine.Process).Id)
					assert.Equal(int32(5), results[1].(engine.Process).Id)
				},
			},
			{
				"by tags not matching",
				engine.ProcessCriteria{Tags: map[string]string{"c": "y"}},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 0)
				},
			},
			{
				"by multiple tags",
				engine.ProcessCriteria{Tags: map[string]string{"a": "b", "x": "y"}},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(int32(5), results[0].(engine.Process).Id)
				},
			},
		})
	})

	t.Run("process instance", func(t *testing.T) {
		runQueryTests(t, e, []queryTest{
			{
				"by partition",
				engine.ProcessInstanceCriteria{Partition: engine.Partition(date)},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(engine.Partition(date), results[0].(engine.ProcessInstance).Partition)
					assert.Equal(int32(1), results[0].(engine.ProcessInstance).Id)
					assert.Equal(engine.Partition(date), results[1].(engine.ProcessInstance).Partition)
					assert.Equal(int32(2), results[1].(engine.ProcessInstance).Id)
					assert.Equal(engine.Partition(date), results[2].(engine.ProcessInstance).Partition)
					assert.Equal(int32(3), results[2].(engine.ProcessInstance).Id)
				},
			},
			{
				"by partition and ID",
				engine.ProcessInstanceCriteria{Partition: engine.Partition(date), Id: 1},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(engine.Partition(date), results[0].(engine.ProcessInstance).Partition)
					assert.Equal(int32(1), results[0].(engine.ProcessInstance).Id)
				},
			},
			{
				"by process ID",
				engine.ProcessInstanceCriteria{ProcessId: 2},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal(int32(2), results[0].(engine.ProcessInstance).ProcessId)
					assert.Equal(int32(2), results[0].(engine.ProcessInstance).ProcessId)
				},
			},
			{
				"by tags",
				engine.ProcessInstanceCriteria{Tags: map[string]string{"a": "b"}},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal(engine.Partition(date), results[0].(engine.ProcessInstance).Partition)
					assert.Equal(int32(1), results[0].(engine.ProcessInstance).Id)
					assert.Equal("b", results[0].(engine.ProcessInstance).Tags["a"])
					assert.Equal(engine.Partition(datePlus1), results[1].(engine.ProcessInstance).Partition)
					assert.Equal(int32(2), results[1].(engine.ProcessInstance).Id)
					assert.Equal("b", results[1].(engine.ProcessInstance).Tags["a"])
				},
			},
			{
				"by tags not matching",
				engine.ProcessInstanceCriteria{Tags: map[string]string{"c": "y"}},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 0)
				},
			},
			{
				"by multiple tags",
				engine.ProcessInstanceCriteria{Tags: map[string]string{"a": "b", "x": "y"}},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(engine.Partition(datePlus1), results[0].(engine.ProcessInstance).Partition)
					assert.Equal(int32(2), results[0].(engine.ProcessInstance).Id)
					assert.Equal("b", results[0].(engine.ProcessInstance).Tags["a"])
					assert.Equal("y", results[0].(engine.ProcessInstance).Tags["x"])
				},
			},
		})
	})

	t.Run("task", func(t *testing.T) {
		runQueryTests(t, e, []queryTest{
			{
				"by partition",
				engine.TaskCriteria{Partition: engine.Partition(date)},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(engine.Partition(date), results[0].(engine.Task).Partition)
					assert.Equal(int32(1), results[0].(engine.Task).Id)
					assert.Equal(engine.Partition(date), results[1].(engine.Task).Partition)
					assert.Equal(int32(2), results[1].(engine.Task).Id)
					assert.Equal(engine.Partition(date), results[2].(engine.Task).Partition)
					assert.Equal(int32(3), results[2].(engine.Task).Id)
				},
			},
			{
				"by partition and ID",
				engine.TaskCriteria{Partition: engine.Partition(date), Id: 1},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(engine.Partition(date), results[0].(engine.Task).Partition)
					assert.Equal(int32(1), results[0].(engine.Task).Id)
				},
			},
			{
				"by element instance ID",
				engine.TaskCriteria{ElementInstanceId: 4},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 1)
					assert.Equal(int32(4), results[0].(engine.Task).ElementInstanceId)
				},
			},
			{
				"by process ID",
				engine.TaskCriteria{ProcessId: 2},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal(int32(2), results[0].(engine.Task).ProcessId)
					assert.Equal(int32(2), results[1].(engine.Task).ProcessId)
				},
			},
			{
				"by process instance ID",
				engine.TaskCriteria{ProcessInstanceId: 20},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal(int32(20), results[0].(engine.Task).ProcessInstanceId)
					assert.Equal(int32(20), results[0].(engine.Task).ProcessInstanceId)
				},
			},
			{
				"by type",
				engine.TaskCriteria{Type: engine.TaskDetachPartition},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal(engine.TaskDetachPartition, results[0].(engine.Task).Type)
					assert.Equal(engine.TaskDetachPartition, results[0].(engine.Task).Type)
				},
			},
		})
	})

	t.Run("variable", func(t *testing.T) {
		runQueryTests(t, e, []queryTest{
			{
				"by partition",
				engine.VariableCriteria{Partition: engine.Partition(date)},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(engine.Partition(date), results[0].(engine.Variable).Partition)
					assert.Equal(int32(1), results[0].(engine.Variable).Id)
					assert.Equal(engine.Partition(date), results[1].(engine.Variable).Partition)
					assert.Equal(int32(2), results[1].(engine.Variable).Id)
					assert.Equal(engine.Partition(date), results[2].(engine.Variable).Partition)
					assert.Equal(int32(3), results[2].(engine.Variable).Id)
				},
			},
			{
				"by element instance ID",
				engine.VariableCriteria{ElementInstanceId: 3},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 2)
					assert.Equal(int32(3), results[0].(engine.Variable).ElementInstanceId)
					assert.Equal("a", results[0].(engine.Variable).Name)
					assert.Equal(int32(3), results[1].(engine.Variable).ElementInstanceId)
					assert.Equal("b", results[1].(engine.Variable).Name)
				},
			},
			{
				"by process instance ID",
				engine.VariableCriteria{ProcessInstanceId: 10},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal(int32(10), results[0].(engine.Variable).ProcessInstanceId)
					assert.Equal(int32(10), results[1].(engine.Variable).ProcessInstanceId)
					assert.Equal(int32(10), results[2].(engine.Variable).ProcessInstanceId)
				},
			},
			{
				"by name",
				engine.VariableCriteria{Names: []string{"a", "c"}},
				func(assert *assert.Assertions, results []any) {
					assert.Len(results, 3)
					assert.Equal("a", results[0].(engine.Variable).Name)
					assert.Equal("c", results[1].(engine.Variable).Name)
					assert.Equal("a", results[2].(engine.Variable).Name)
				},
			},
		})
	})
}

type queryOptionTest struct {
	name          string
	options       engine.QueryOptions
	expectedCount int
}

type queryTest struct {
	name     string
	criteria any
	assertFn func(*assert.Assertions, []any)
}

func runQueryOptionTests(t *testing.T, e engine.Engine, criteria any, tests []queryOptionTest) {
	assert := assert.New(t)

	for _, test := range tests {
		t.Run(fmt.Sprintf("%T %s", criteria, test.name), func(t *testing.T) {
			results, err := e.QueryWithOptions(criteria, test.options)
			if err != nil {
				t.Fatalf("failed to query results: %v", err)
			}

			assert.Len(results, test.expectedCount)
		})
	}
}

func runQueryTests(t *testing.T, e engine.Engine, tests []queryTest) {
	assert := assert.New(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			results, err := e.Query(test.criteria)
			if err != nil {
				t.Fatalf("failed to query results: %v", err)
			}

			test.assertFn(assert, results)
		})
	}
}
