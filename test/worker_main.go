package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/http/client"
	"github.com/gclaussn/go-bpmn/worker"
)

func main() {
	url := os.Getenv("GO_BPMN_URL")
	if url == "" {
		log.Fatalf("missing environment variable GO_BPMN_URL")
	}

	authorization := os.Getenv("GO_BPMN_AUTHORIZATION")
	if authorization == "" {
		log.Fatalf("missing environment variable GO_BPMN_AUTHORIZATION")
	}

	e, err := client.New(url, authorization)
	if err != nil {
		log.Fatalf("failed to create HTTP client: %v", err)
	}

	defer e.Shutdown()

	w, err := worker.New(e, func(o *worker.Options) {
		o.OnJobExecutionFailure = func(job engine.Job, err error) {
			log.Printf("failed to execute job %s: %v", job, err)
		}
	})
	if err != nil {
		log.Fatalf("failed to create worker: %v", err)
	}

	serviceTaskProcess, err := w.Register(serviceTaskDelegate{})
	if err != nil {
		log.Fatalf("failed to register service task delegate: %v", err)
	}

	w.Start()

	tickerCtx, tickerCancel := context.WithCancel(context.Background())
	ticker := time.NewTicker(time.Second * 10)

	go func() {
		for {
			select {
			case <-ticker.C:
				variables := worker.Variables{}
				variables.PutVariable("a", "b")

				processInstance, err := serviceTaskProcess.CreateProcessInstance(context.Background(), variables)
				if err != nil {
					log.Printf("failed to create process instance: %v", err)
					continue
				}

				log.Printf("created process instance %s", processInstance)
			case <-tickerCtx.Done():
				return
			}
		}
	}()

	signalC := make(chan os.Signal, 1)
	signal.Notify(signalC, os.Interrupt, syscall.SIGTERM)

	<-signalC

	ticker.Stop()
	tickerCancel()

	w.Stop()
}
