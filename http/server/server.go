package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/pg"
	"github.com/gclaussn/go-bpmn/http/common"
)

func New(e engine.Engine, customizers ...func(*Options)) (*Server, error) {
	options := NewOptions()
	for _, customizer := range customizers {
		customizer(&options)
	}

	if err := options.Validate(); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	var handler http.Handler
	if options.ApiKeyManager != nil {
		handler = &authHandler{
			apiKeyManager: options.ApiKeyManager,
			handler:       mux,
		}
	} else {
		handler = &basicAuthHandler{
			username: options.BasicAuthUsername,
			password: options.BasicAuthPassword,
			handler:  mux,
		}
	}

	// server-wide context for incoming requests
	httpServerCtx, httpServerCanel := context.WithCancel(context.Background())

	httpServer := http.Server{
		Addr: options.BindAddress,
		BaseContext: func(_ net.Listener) context.Context {
			return httpServerCtx
		},
		Handler:      http.TimeoutHandler(handler, options.HandlerTimeout, "handler timed out"),
		IdleTimeout:  options.IdleTimeout,
		ReadTimeout:  options.ReadTimeout,
		WriteTimeout: options.WriteTimeout,
	}

	if options.Configure != nil {
		options.Configure(&httpServer)
	}

	server := Server{
		e:                e,
		httpServer:       &httpServer,
		httpServerCtx:    httpServerCtx,
		httpServerCancel: httpServerCanel,
		options:          options,
	}

	// operations:start
	mux.HandleFunc("POST "+common.PathElementsQuery, server.queryElements)

	mux.HandleFunc("POST "+common.PathElementInstancesQuery, server.queryElementInstances)
	mux.HandleFunc("GET "+common.PathElementInstancesVariables, server.getElementVariables)
	mux.HandleFunc("PUT "+common.PathElementInstancesVariables, server.setElementVariables)

	mux.HandleFunc("POST "+common.PathEventsMessages, server.sendMessage)
	mux.HandleFunc("POST "+common.PathEventsMessagesQuery, server.queryMessages)
	mux.HandleFunc("POST "+common.PathEventsSignals, server.sendSignal)

	mux.HandleFunc("POST "+common.PathIncidentsQuery, server.queryIncidents)
	mux.HandleFunc("PATCH "+common.PathIncidentsResolve, server.resolveIncident)

	mux.HandleFunc("PATCH "+common.PathJobsComplete, server.completeJob)
	mux.HandleFunc("POST "+common.PathJobsLock, server.lockJobs)
	mux.HandleFunc("POST "+common.PathJobsQuery, server.queryJobs)
	mux.HandleFunc("POST "+common.PathJobsUnlock, server.unlockJobs)

	mux.HandleFunc("POST "+common.PathProcesses, server.createProcess)
	mux.HandleFunc("GET "+common.PathProcessesBpmnXml, server.getBpmnXml)
	mux.HandleFunc("POST "+common.PathProcessesQuery, server.queryProcesses)

	mux.HandleFunc("POST "+common.PathProcessInstances, server.createProcessInstance)
	mux.HandleFunc("POST "+common.PathProcessInstancesQuery, server.queryProcessInstances)
	mux.HandleFunc("PATCH "+common.PathProcessInstancesResume, server.resumeProcessInstance)
	mux.HandleFunc("PATCH "+common.PathProcessInstancesSuspend, server.suspendProcessInstance)
	mux.HandleFunc("GET "+common.PathProcessInstancesVariables, server.getProcessVariables)
	mux.HandleFunc("PUT "+common.PathProcessInstancesVariables, server.setProcessVariables)

	mux.HandleFunc("POST "+common.PathTasksExecute, server.executeTasks)
	mux.HandleFunc("POST "+common.PathTasksQuery, server.queryTasks)
	mux.HandleFunc("POST "+common.PathTasksUnlock, server.unlockTasks)

	mux.HandleFunc("POST "+common.PathVariablesQuery, server.queryVariables)

	mux.HandleFunc("GET "+common.PathReadiness, server.checkReadiness)
	mux.HandleFunc("PATCH "+common.PathTime, server.setTime)
	// operations:end

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	return &server, nil
}

func NewOptions() Options {
	return Options{
		BindAddress: "127.0.0.1:8080",

		HandlerTimeout: 30 * time.Second,
		IdleTimeout:    60 * time.Second,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   35 * time.Second,

		ShutdownDelay:       5 * time.Second,
		ShutdownPeriod:      30 * time.Second,
		ShutdownForcePeriod: 5 * time.Second,
	}
}

type Options struct {
	BindAddress string // TCP address for the server to listen on.

	HandlerTimeout time.Duration // Time limit for HTTP handler - when reached, the handler responds with HTTP 503.
	IdleTimeout    time.Duration // Maximum amount of time to wait for the next request, when keep-alives are enabled - - see http.Server#ReadTimeout
	ReadTimeout    time.Duration // Maximum duration for reading the entire request - see http.Server#ReadTimeout
	WriteTimeout   time.Duration // Maximum duration before timing out writing the response - see http.Server#WriteTimeout

	ShutdownDelay       time.Duration // Delay between the shutdown signal and the actual shutdown, used to propagate readiness.
	ShutdownPeriod      time.Duration // Period for a graceful shutdown without interrupting ongoging requests.
	ShutdownForcePeriod time.Duration // Period for a forced shutdown, where ongoging requests are canceled.

	ApiKeyManager     pg.ApiKeyManager // Used for API key based authorization.
	BasicAuthUsername string           // Only required if ApiKeyManager is not configured.
	BasicAuthPassword string           // Only required if ApiKeyManager is not configured.

	SetTimeEnabled bool // Determines if the set time operation is permitted.

	Configure func(*http.Server) // Optional function, used to configure the underlying HTTP server if needed.
}

func (o Options) Validate() error {
	if o.ApiKeyManager == nil && (o.BasicAuthUsername == "" || o.BasicAuthPassword == "") {
		return errors.New("api key manager or basic auth username and password must be provided")
	}

	return nil
}

type Server struct {
	e                engine.Engine
	httpServer       *http.Server
	httpServerCtx    context.Context    // server-wide base context for incoming requests
	httpServerCancel context.CancelFunc // invoked after server shutdown to cancel to ongoing requests
	isShuttingDown   atomic.Bool
	options          Options
}

func (s *Server) ListenAndServe() {
	go func() {
		log.Printf("server listening on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("failed to listen and serve HTTP: %v", err)
		}
	}()
}

func (s *Server) Shutdown() {
	s.isShuttingDown.Store(true)
	log.Println("server is shutting down")

	time.Sleep(s.options.ShutdownDelay)
	log.Println("server is shutting down gracefully")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.options.ShutdownPeriod)
	defer shutdownCancel()

	err := s.httpServer.Shutdown(shutdownCtx)
	s.httpServerCancel()
	if err != nil {
		log.Printf("failed to shutdown HTTP server: %v", err)
		time.Sleep(s.options.ShutdownForcePeriod)
	}

	s.e.Shutdown()
	log.Println("server shut down")
}

// command handler

func (s *Server) completeJob(w http.ResponseWriter, r *http.Request) {
	partition, id, err := parsePartitionId(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var cmd engine.CompleteJobCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	cmd.Partition = partition
	cmd.Id = id

	job, err := s.e.CompleteJob(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, job, http.StatusOK)
}

func (s *Server) createProcess(w http.ResponseWriter, r *http.Request) {
	var cmd engine.CreateProcessCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	process, err := s.e.CreateProcess(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, process, http.StatusCreated)
}

func (s *Server) createProcessInstance(w http.ResponseWriter, r *http.Request) {
	var cmd engine.CreateProcessInstanceCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	processInstance, err := s.e.CreateProcessInstance(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, processInstance, http.StatusCreated)
}

func (s *Server) executeTasks(w http.ResponseWriter, r *http.Request) {
	var cmd engine.ExecuteTasksCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	completedTasks, failedTasks, err := s.e.ExecuteTasks(r.Context(), cmd)
	if err != nil && completedTasks == nil && failedTasks == nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.ExecuteTasksRes{
		Locked:    len(completedTasks) + len(failedTasks),
		Completed: len(completedTasks),
		Failed:    len(failedTasks),

		CompletedTasks: completedTasks,
		FailedTasks:    failedTasks,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) getBpmnXml(w http.ResponseWriter, r *http.Request) {
	id, err := parseId(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	bpmnXml, err := s.e.GetBpmnXml(r.Context(), engine.GetBpmnXmlCmd{ProcessId: id})
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.Header().Set(common.HeaderContentType, common.ContentTypeXml)
	w.Write([]byte(bpmnXml))
}

func (s *Server) getElementVariables(w http.ResponseWriter, r *http.Request) {
	partition, id, err := parsePartitionId(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var names []string
	if namesValues, ok := r.URL.Query()[common.QueryNames]; ok {
		for _, namesValue := range namesValues {
			names = append(names, strings.Split(namesValue, ",")...)
		}
	}

	variables, err := s.e.GetElementVariables(r.Context(), engine.GetElementVariablesCmd{
		Partition:         partition,
		ElementInstanceId: id,

		Names: names,
	})
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.GetVariablesRes{
		Count:     len(variables),
		Variables: variables,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) getProcessVariables(w http.ResponseWriter, r *http.Request) {
	partition, id, err := parsePartitionId(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var names []string
	if namesValues, ok := r.URL.Query()[common.QueryNames]; ok {
		for _, namesValue := range namesValues {
			names = append(names, strings.Split(namesValue, ",")...)
		}
	}

	variables, err := s.e.GetProcessVariables(r.Context(), engine.GetProcessVariablesCmd{
		Partition:         partition,
		ProcessInstanceId: id,

		Names: names,
	})
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.GetVariablesRes{
		Count:     len(variables),
		Variables: variables,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) lockJobs(w http.ResponseWriter, r *http.Request) {
	var cmd engine.LockJobsCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	jobs, err := s.e.LockJobs(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.LockJobsRes{
		Count: len(jobs),
		Jobs:  jobs,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) resolveIncident(w http.ResponseWriter, r *http.Request) {
	partition, id, err := parsePartitionId(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var cmd engine.ResolveIncidentCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	cmd.Partition = partition
	cmd.Id = id

	if err := s.e.ResolveIncident(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) resumeProcessInstance(w http.ResponseWriter, r *http.Request) {
	partition, id, err := parsePartitionId(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var cmd engine.ResumeProcessInstanceCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	cmd.Partition = partition
	cmd.Id = id

	if err := s.e.ResumeProcessInstance(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) sendMessage(w http.ResponseWriter, r *http.Request) {
	var cmd engine.SendMessageCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	messageCorrelation, err := s.e.SendMessage(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, messageCorrelation, http.StatusOK)
}

func (s *Server) sendSignal(w http.ResponseWriter, r *http.Request) {
	var cmd engine.SendSignalCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	signal, err := s.e.SendSignal(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, signal, http.StatusOK)
}

func (s *Server) setElementVariables(w http.ResponseWriter, r *http.Request) {
	partition, id, err := parsePartitionId(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var cmd engine.SetElementVariablesCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	cmd.Partition = partition
	cmd.ElementInstanceId = id

	if err := s.e.SetElementVariables(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setProcessVariables(w http.ResponseWriter, r *http.Request) {
	partition, id, err := parsePartitionId(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var cmd engine.SetProcessVariablesCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	cmd.Partition = partition
	cmd.ProcessInstanceId = id

	if err := s.e.SetProcessVariables(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setTime(w http.ResponseWriter, r *http.Request) {
	if !s.options.SetTimeEnabled {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	var cmd engine.SetTimeCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	if err := s.e.SetTime(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) suspendProcessInstance(w http.ResponseWriter, r *http.Request) {
	partition, id, err := parsePartitionId(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var cmd engine.SuspendProcessInstanceCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	cmd.Partition = partition
	cmd.Id = id

	if err := s.e.SuspendProcessInstance(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) unlockJobs(w http.ResponseWriter, r *http.Request) {
	var cmd engine.UnlockJobsCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	count, err := s.e.UnlockJobs(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, common.CountRes{Count: count}, http.StatusOK)
}

func (s *Server) unlockTasks(w http.ResponseWriter, r *http.Request) {
	var cmd engine.UnlockTasksCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	count, err := s.e.UnlockTasks(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, common.CountRes{Count: count}, http.StatusOK)
}

// query handler

func (s *Server) queryElements(w http.ResponseWriter, r *http.Request) {
	options, err := parseQueryOptions(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var criteria engine.ElementCriteria
	if err := decodeJSONRequestBody(w, r, &criteria); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	q := s.e.CreateQuery()
	q.SetOptions(options)

	results, err := q.QueryElements(r.Context(), criteria)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.ElementRes{
		Count:   len(results),
		Results: results,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) queryElementInstances(w http.ResponseWriter, r *http.Request) {
	options, err := parseQueryOptions(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var criteria engine.ElementInstanceCriteria
	if err := decodeJSONRequestBody(w, r, &criteria); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	q := s.e.CreateQuery()
	q.SetOptions(options)

	results, err := q.QueryElementInstances(r.Context(), criteria)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.ElementInstanceRes{
		Count:   len(results),
		Results: results,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) queryIncidents(w http.ResponseWriter, r *http.Request) {
	options, err := parseQueryOptions(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var criteria engine.IncidentCriteria
	if err := decodeJSONRequestBody(w, r, &criteria); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	q := s.e.CreateQuery()
	q.SetOptions(options)

	results, err := q.QueryIncidents(r.Context(), criteria)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.IncidentRes{
		Count:   len(results),
		Results: results,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) queryJobs(w http.ResponseWriter, r *http.Request) {
	options, err := parseQueryOptions(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var criteria engine.JobCriteria
	if err := decodeJSONRequestBody(w, r, &criteria); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	q := s.e.CreateQuery()
	q.SetOptions(options)

	results, err := q.QueryJobs(r.Context(), criteria)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.JobRes{
		Count:   len(results),
		Results: results,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) queryMessages(w http.ResponseWriter, r *http.Request) {
	options, err := parseQueryOptions(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var criteria engine.MessageCriteria
	if err := decodeJSONRequestBody(w, r, &criteria); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	q := s.e.CreateQuery()
	q.SetOptions(options)

	results, err := q.QueryMessages(r.Context(), criteria)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.MessageRes{
		Count:   len(results),
		Results: results,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) queryProcesses(w http.ResponseWriter, r *http.Request) {
	options, err := parseQueryOptions(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var criteria engine.ProcessCriteria
	if err := decodeJSONRequestBody(w, r, &criteria); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	q := s.e.CreateQuery()
	q.SetOptions(options)

	results, err := q.QueryProcesses(r.Context(), criteria)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.ProcessRes{
		Count:   len(results),
		Results: results,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) queryProcessInstances(w http.ResponseWriter, r *http.Request) {
	options, err := parseQueryOptions(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var criteria engine.ProcessInstanceCriteria
	if err := decodeJSONRequestBody(w, r, &criteria); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	q := s.e.CreateQuery()
	q.SetOptions(options)

	results, err := q.QueryProcessInstances(r.Context(), criteria)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.ProcessInstanceRes{
		Count:   len(results),
		Results: results,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) queryTasks(w http.ResponseWriter, r *http.Request) {
	options, err := parseQueryOptions(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var criteria engine.TaskCriteria
	if err := decodeJSONRequestBody(w, r, &criteria); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	q := s.e.CreateQuery()
	q.SetOptions(options)

	results, err := q.QueryTasks(r.Context(), criteria)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.TaskRes{
		Count:   len(results),
		Results: results,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

func (s *Server) queryVariables(w http.ResponseWriter, r *http.Request) {
	options, err := parseQueryOptions(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	var criteria engine.VariableCriteria
	if err := decodeJSONRequestBody(w, r, &criteria); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	q := s.e.CreateQuery()
	q.SetOptions(options)

	results, err := q.QueryVariables(r.Context(), criteria)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	resBody := common.VariableRes{
		Count:   len(results),
		Results: results,
	}

	encodeJSONResponseBody(w, r, resBody, http.StatusOK)
}

// management

func (s *Server) checkReadiness(w http.ResponseWriter, r *http.Request) {
	if s.isShuttingDown.Load() {
		http.Error(w, "server is shutting down", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ready"))
}
