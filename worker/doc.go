// Package worker provides a SDK to implement and automate processes.
/*
worker offers a handler interface, which must be implemented to automate the execution of jobs, created within an engine.

Create a Worker

A worker requires an engine.
The engine can be an embedded engine (pg, or mem for testing) or a remote engine (HTTP client).

	w, err := worker.New(e, func(o *worker.Options) {
		o.OnJobExecutionFailure = func(job engine.Job, err error) {
			log.Printf("failed to execute job %s: %v", job, err)
		}
	})
	if err != nil {
		log.Fatalf("failed to create worker: %v", err)
	}

Implement a Process

A process automation must implement the [Handler] interface.

	type exampleHandler struct {}

	func (h exampleHandler) CreateProcessCmd() (engine.CreateProcessCmd, error) {
		bpmnFile, err := os.Open("./example.bpmn")
		if err != nil {
			return engine.CreateProcessCmd{}, err
		}

		defer bpmnFile.Close()

		bpmnXml, err := io.ReadAll(bpmnFile)
		if err != nil {
			return engine.CreateProcessCmd{}, err
		}

		return engine.CreateProcessCmd{
			BpmnProcessId: "example",
			BpmnXml:       string(bpmnXml),
			Version:       "1",
		}, nil
	}

	func (h exampleHandler) Handle(mux worker.JobMux) error {
		mux.Execute("serviceTask", d.executeServiceTask)
		return nil
	}

	func (h exampleHandler) executeServiceTask(jc worker.JobContext) error {
		// ...
		return nil
	}

Run a Worker

Before a worker is started, implemented handlers must be registered.

	exampleProcess, err := w.Register(exampleHandler{})
	if err != nil {
		log.Fatalf("failed to register example handler: %v", err)
	}

	w.Start()

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt, syscall.SIGTERM)

	<-signalC

	w.Stop()

Create a Process Instance

A process handle can be used to create a new process instance with variables.

	variables := worker.Variables{}
	variables.PutVariable("a", "value a")

	processInstance, err := exampleProcess.CreateProcessInstance(variables)
	if err != nil {
		log.Printf("failed to create process instance: %v", err)
	}
*/
package worker
