---
description: Tutorial on how to automate a prcess.
---

# Automate a process

The following tutorial shows the automation of an example process.

The process consists of an exclusive gateway and 2 service tasks.
A decision must be made, if either service task `doX` or `doY` should be executed.
After the gateway is evaluated, one service task must be executed to end an instance of the example process.

<p align="center" style="background-color: white">
  <img src="/example.svg" alt="example.bpmn"></img>
</p>

::: details example.bpmn

```xml
<definitions>
  <process id="example" isExecutable="true">
    <startEvent id="startEvent" name="Start">
      <outgoing>f1</outgoing>
    </startEvent>
    <sequenceFlow id="f1" sourceRef="startEvent" targetRef="fork" />
    <exclusiveGateway id="fork" name="X or Y?">
      <incoming>f1</incoming>
      <outgoing>f2</outgoing>
      <outgoing>f3</outgoing>
    </exclusiveGateway>
    <sequenceFlow id="f2" sourceRef="fork" targetRef="doX" />
    <serviceTask id="doX" name="Do X">
      <incoming>f2</incoming>
      <outgoing>f4</outgoing>
    </serviceTask>
    <sequenceFlow id="f4" sourceRef="doX" targetRef="join" />
    <sequenceFlow id="f3" sourceRef="fork" targetRef="doY" />
    <serviceTask id="doY" name="Do Y">
      <incoming>f3</incoming>
      <outgoing>f5</outgoing>
    </serviceTask>
    <sequenceFlow id="f5" sourceRef="doY" targetRef="join" />
    <exclusiveGateway id="join">
      <incoming>f4</incoming>
      <incoming>f5</incoming>
      <outgoing>f6</outgoing>
    </exclusiveGateway>
    <sequenceFlow id="f6" sourceRef="join" targetRef="endEvent" />
    <endEvent id="endEvent" name="End">
      <incoming>f6</incoming>
    </endEvent>
  </process>
</definitions>
```

:::

::: warning

To reproduce this example, a running process engine and an API key is required - please have a look at the guides on how to [install](installation-all) and [run a process engine](run-process-engine).

:::

## Create process

To execute an instance of the example process, the process must first be created at the engine, using the BPMN 2.0 XML (see file `example.bpmn`).

::: code-group

```sh [curl]
curl \
-H "Authorization: ${GO_BPMN_AUTHORIZATION}" \
-H "Content-Type: application/json" \
-X POST http://127.0.0.1:8080/processes \
-d "$(jq -n --arg bpmnXml "$(cat example.bpmn)" '{"bpmnProcessId": "example", "bpmnXml": $bpmnXml, "version": "1", "workerId": "curl"}')"
```

```sh [CLI]
go-bpmn process create \
--bpmn-file example.bpmn \
--bpmn-process-id example \
--version 1
```

```go [go]
// create process engine, using a PostgreSQL database URL
e, err := pg.New("postgres://username:password@127.0.0.1:5432/database?search_path=schema")
if err != nil {
  log.Fatalf("failed to create process engine: %v", err)
}

defer e.Shutdown()

// read BPMN XML from file
bpmnFile, err := os.Open("example.bpmn")
if err != nil {
  log.Fatalf("failed to open BPMN file: %v", err)
}

defer bpmnFile.Close()

bpmnXml, err := io.ReadAll(bpmnFile)
if err != nil {
  log.Fatalf("failed to read BPMN XML: %v", err)
}

// create process
process, err := e.CreateProcess(context.Background(), engine.CreateProcessCmd{
  BpmnProcessId: "example",
  BpmnXml:       string(bpmnXml),
  Version:       "1",
  WorkerId:      "go",
})
if err != nil {
  log.Fatalf("failed to create process: %v", err)
}
```

:::

## Create process instance

Based on an existing process, identified by BPMN process ID and version, instances can be created.
A process instance can hold any data in form of string encoded process variables.

In this example, the initial process data is provided as variable `xory`, a JSON encoded string with value `x`.

::: code-group

```sh [curl]
curl \
-H "Authorization: ${GO_BPMN_AUTHORIZATION}" \
-H "Content-Type: application/json" \
-X POST http://127.0.0.1:8080/process-instances \
-d '{"bpmnProcessId": "example", "variables": [{"name": "xory", "data": {"encoding": "json", "value": "x"}}], "version": "1", "workerId": "curl"}'
```

```sh [CLI]
go-bpmn process-instance create \
--bpmn-process-id example \
--variable-value xory="x" \
--variable-encoding xory="json" \
--version 1
```

```go [go]
processInstance, err := e.CreateProcessInstance(context.Background(), engine.CreateProcessInstanceCmd{
  BpmnProcessId: "example",
  Variables: []engine.VariableData{
    {Name: "xory", Data: &engine.Data{Encoding: "json", Value: "x"}},
  },
  Version:  "1",
  WorkerId: "go",
})
if err != nil {
  log.Fatalf("failed to create process instance: %v", err)
}
```

:::

## Evaluate exclusive gateway

When the process engine reaches an exclusive gateway, a job of type `EVALUATE_EXCLUSIVE_GATEWAY` is created.
It must be locked, executed and completed to continue the execution.

In case of an exclusive gateway, a job needs to be completed with a decision that provides the ID of an BPMN element to continue with after the gateway.

Lock job to allow an exclusive job execution:

::: code-group

```sh [curl]
curl \
-H "Authorization: ${GO_BPMN_AUTHORIZATION}" \
-H "Content-Type: application/json" \
-X POST http://127.0.0.1:8080/jobs/lock \
-d '{"limit": 1, "workerId": "curl"}'
```

```sh [CLI]
go-bpmn job lock
```

```go [go]
lockedJobs, err := e.LockJobs(context.Background(), engine.LockJobsCmd{
  WorkerId: "go",
})
if err != nil {
  log.Fatalf("failed to lock job: %v", err)
}
if len(lockedJobs) == 0 {
  log.Fatal("no job locked")
}
```

:::

Get variables of process instance (to make an decision):

::: code-group

```sh [curl]
curl \
-H "Authorization: ${GO_BPMN_AUTHORIZATION}" \
"http://127.0.0.1:8080/process-instances/$(date -I)/1/variables"
```

```sh [CLI]
go-bpmn process-instance get-variables --partition $(date -I) --id 1
```

```go [go]
variables, err := e.GetProcessVariables(context.Background(), engine.GetProcessVariablesCmd{
  Partition:         engine.Partition{},
  ProcessInstanceId: 1,
})
if err != nil {
  log.Fatalf("failed to get process variables: %v", err)
}
```

:::

Complete job with an exclusive gateway decision:

::: code-group

```sh [curl]
curl \
-H "Authorization: ${GO_BPMN_AUTHORIZATION}" \
-H "Content-Type: application/json" \
-X PATCH http://127.0.0.1:8080/jobs/$(date -I)/1/complete \
-d '{"completion": {"exclusiveGatewayDecision": "doX"}, "workerId": "curl"}'
```

```sh [CLI]
go-bpmn job complete \
--exclusive-gateway-decision doX \
--partition $(date -I) \
--id 1
```

```go [go]
completedJob, err := e.CompleteJob(context.Background(), engine.CompleteJobCmd{
  Partition: lockedJobs[0].Partition,
  Id:        lockedJobs[0].Id,
  Completion: &engine.JobCompletion{
    ExclusiveGatewayDecision: "doX",
  },
  WorkerId: "go",
})
if err != nil {
  log.Fatalf("failed to complete job: %v", err)
}
```

:::

## Execute service task

When the process engine reaches a service task, a job of type `EXECUTE` is created.
It must be locked, executed and completed to continue the execution.

In case of a service task, no type-specific completion is required.

Lock job to allow an exclusive job execution:

::: code-group

```sh [curl]
curl \
-H "Authorization: ${GO_BPMN_AUTHORIZATION}" \
-H "Content-Type: application/json" \
-X POST http://127.0.0.1:8080/jobs/lock \
-d '{"limit": 1, "workerId": "curl"}'
```

```sh [CLI]
go-bpmn job lock
```

```go [go]
lockedJobs, err := e.LockJobs(context.Background(), engine.LockJobsCmd{
  WorkerId: "go",
})
if err != nil {
  log.Fatalf("failed to lock job: %v", err)
}
if len(lockedJobs) == 0 {
  log.Fatal("no job locked")
}
```

:::

Complete job and set process variable `result`:

::: code-group

```sh [curl]
curl \
-H "Authorization: ${GO_BPMN_AUTHORIZATION}" \
-H "Content-Type: application/json" \
-X PATCH http://127.0.0.1:8080/jobs/$(date -I)/2/complete \
-d '{"processVariables": [{"name": "result", "data": {"encoding": "json", "value": "x done"}}], "workerId": "curl"}'
```

```sh [CLI]
go-bpmn job complete \
--partition $(date -I) \
--process-variable-value result="x done" \
--process-variable-encoding result="json" \
--id 2
```

```go [go]
completedJob, err := e.CompleteJob(context.Background(), engine.CompleteJobCmd{
  Partition: lockedJobs[0].Partition,
  Id:        lockedJobs[0].Id,
  ProcessVariables: []engine.VariableData{
    {Name: "result", Data: &engine.Data{Encoding: "json", Value: "x done"}},
  },
  WorkerId: "go",
})
if err != nil {
  log.Fatalf("failed to complete job: %v", err)
}
```

:::

Verify that process instance is in state `COMPLETED`:

::: code-group

```sh [curl]
curl \
-H "Authorization: ${GO_BPMN_AUTHORIZATION}" \
-H "Content-Type: application/json" \
-X POST http://127.0.0.1:8080/process-instances/query \
-d '{}'
```

```sh [CLI]
go-bpmn process-instance query
```

```go [go]
q := e.CreateQuery()

results, err := q.QueryProcessInstances(context.Background(), engine.ProcessInstanceCriteria{
  Partition: engine.Partition{},
  Id:        1,
})
if err != nil {
  log.Fatalf("failed to query process instance: %v", err)
}

if len(results) != 1 {
  log.Fatalf("expected one process instance, but got %d", len(results))
}

processInstance := results[0]
if processInstance.State != engine.InstanceCompleted {
  log.Fatalf("expected process instance to be completed, but is not")
}
```

:::
