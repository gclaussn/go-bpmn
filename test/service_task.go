package main

import (
	"io"
	"os"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type serviceTaskDelegate struct {
}

func (d serviceTaskDelegate) CreateProcessCmd() (engine.CreateProcessCmd, error) {
	bpmnFile, err := os.Open("./bpmn/task/service.bpmn")
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	defer bpmnFile.Close()

	bpmnXml, err := io.ReadAll(bpmnFile)
	if err != nil {
		return engine.CreateProcessCmd{}, err
	}

	return engine.CreateProcessCmd{
		BpmnProcessId: "serviceTest",
		BpmnXml:       string(bpmnXml),
		Version:       "1",
	}, nil
}

func (d serviceTaskDelegate) Delegate(delegator worker.Delegator) error {
	delegator.Execute("serviceTask", d.executeServiceTask)
	return nil
}

func (d serviceTaskDelegate) executeServiceTask(jc worker.JobContext) error {
	processVariables := worker.Variables{}
	processVariables.DeleteVariable("a")

	jc.SetProcessVariables(processVariables)
	return nil
}
