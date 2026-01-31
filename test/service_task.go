package main

import (
	"io"
	"os"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/worker"
)

type serviceTask struct {
}

func (h serviceTask) CreateProcessCmd() (engine.CreateProcessCmd, error) {
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

func (h serviceTask) Handle(mux worker.JobMux) error {
	mux.Execute("serviceTask", h.executeServiceTask)
	return nil
}

func (h serviceTask) executeServiceTask(jc worker.JobContext) error {
	processVariables := worker.Variables{}
	processVariables.Delete("a")

	jc.SetProcessVariables(processVariables)
	return nil
}
