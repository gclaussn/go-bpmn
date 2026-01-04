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

	mux := NewMux(e, options.SetTimeEnabled)

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

	s := Server{
		httpServer:       &httpServer,
		httpServerCtx:    httpServerCtx,
		httpServerCancel: httpServerCanel,
		options:          options,
	}

	// operations:start
	mux.HandleFunc("GET "+common.PathReadiness, s.checkReadiness)
	// operations:end

	return &s, nil
}

// NewMux creates a HTTP request multiplexer for an engine.
//
// If setTimeEnabled is false, the "setTime" operation will respond with HTTP 403 Forbidden.
func NewMux(e engine.Engine, setTimeEnabled bool) *http.ServeMux {
	h := handler{e: e, setTimeEnabled: setTimeEnabled}

	mux := http.NewServeMux()

	// operations:start
	mux.HandleFunc("POST "+common.PathElementsQuery, h.queryElements)

	mux.HandleFunc("POST "+common.PathElementInstancesQuery, h.queryElementInstances)
	mux.HandleFunc("GET "+common.PathElementInstancesVariables, h.getElementVariables)
	mux.HandleFunc("PUT "+common.PathElementInstancesVariables, h.setElementVariables)

	mux.HandleFunc("POST "+common.PathEventsMessages, h.sendMessage)
	mux.HandleFunc("POST "+common.PathEventsMessagesQuery, h.queryMessages)
	mux.HandleFunc("POST "+common.PathEventsSignals, h.sendSignal)

	mux.HandleFunc("POST "+common.PathIncidentsQuery, h.queryIncidents)
	mux.HandleFunc("PATCH "+common.PathIncidentsResolve, h.resolveIncident)

	mux.HandleFunc("PATCH "+common.PathJobsComplete, h.completeJob)
	mux.HandleFunc("POST "+common.PathJobsLock, h.lockJobs)
	mux.HandleFunc("POST "+common.PathJobsQuery, h.queryJobs)
	mux.HandleFunc("POST "+common.PathJobsUnlock, h.unlockJobs)

	mux.HandleFunc("POST "+common.PathProcesses, h.createProcess)
	mux.HandleFunc("GET "+common.PathProcessesBpmnXml, h.getBpmnXml)
	mux.HandleFunc("POST "+common.PathProcessesQuery, h.queryProcesses)

	mux.HandleFunc("POST "+common.PathProcessInstances, h.createProcessInstance)
	mux.HandleFunc("POST "+common.PathProcessInstancesQuery, h.queryProcessInstances)
	mux.HandleFunc("PATCH "+common.PathProcessInstancesResume, h.resumeProcessInstance)
	mux.HandleFunc("PATCH "+common.PathProcessInstancesSuspend, h.suspendProcessInstance)
	mux.HandleFunc("GET "+common.PathProcessInstancesVariables, h.getProcessVariables)
	mux.HandleFunc("PUT "+common.PathProcessInstancesVariables, h.setProcessVariables)

	mux.HandleFunc("POST "+common.PathTasksExecute, h.executeTasks)
	mux.HandleFunc("POST "+common.PathTasksQuery, h.queryTasks)
	mux.HandleFunc("POST "+common.PathTasksUnlock, h.unlockTasks)

	mux.HandleFunc("POST "+common.PathVariablesQuery, h.queryVariables)

	mux.HandleFunc("PATCH "+common.PathTime, h.setTime)
	// operations:end

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	return mux
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
	IdleTimeout    time.Duration // Maximum amount of time to wait for the next request, when keep-alives are enabled - see http.Server#ReadTimeout
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
}

func (s *Server) checkReadiness(w http.ResponseWriter, r *http.Request) {
	if s.isShuttingDown.Load() {
		http.Error(w, "server is shutting down", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ready"))
}

type handler struct {
	e              engine.Engine
	setTimeEnabled bool
}

// command handler

func (h handler) completeJob(w http.ResponseWriter, r *http.Request) {
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

	job, err := h.e.CompleteJob(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, job, http.StatusOK)
}

func (h handler) createProcess(w http.ResponseWriter, r *http.Request) {
	var cmd engine.CreateProcessCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	process, err := h.e.CreateProcess(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, process, http.StatusCreated)
}

func (h handler) createProcessInstance(w http.ResponseWriter, r *http.Request) {
	var cmd engine.CreateProcessInstanceCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	processInstance, err := h.e.CreateProcessInstance(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, processInstance, http.StatusCreated)
}

func (h handler) executeTasks(w http.ResponseWriter, r *http.Request) {
	var cmd engine.ExecuteTasksCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	completedTasks, failedTasks, err := h.e.ExecuteTasks(r.Context(), cmd)
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

func (h handler) getBpmnXml(w http.ResponseWriter, r *http.Request) {
	id, err := parseId(r)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	bpmnXml, err := h.e.GetBpmnXml(r.Context(), engine.GetBpmnXmlCmd{ProcessId: id})
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.Header().Set(common.HeaderContentType, common.ContentTypeXml)
	w.Write([]byte(bpmnXml))
}

func (h handler) getElementVariables(w http.ResponseWriter, r *http.Request) {
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

	variables, err := h.e.GetElementVariables(r.Context(), engine.GetElementVariablesCmd{
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

func (h handler) getProcessVariables(w http.ResponseWriter, r *http.Request) {
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

	variables, err := h.e.GetProcessVariables(r.Context(), engine.GetProcessVariablesCmd{
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

func (h handler) lockJobs(w http.ResponseWriter, r *http.Request) {
	var cmd engine.LockJobsCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	jobs, err := h.e.LockJobs(r.Context(), cmd)
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

func (h handler) resolveIncident(w http.ResponseWriter, r *http.Request) {
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

	if err := h.e.ResolveIncident(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h handler) resumeProcessInstance(w http.ResponseWriter, r *http.Request) {
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

	if err := h.e.ResumeProcessInstance(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h handler) sendMessage(w http.ResponseWriter, r *http.Request) {
	var cmd engine.SendMessageCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	messageCorrelation, err := h.e.SendMessage(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, messageCorrelation, http.StatusOK)
}

func (h handler) sendSignal(w http.ResponseWriter, r *http.Request) {
	var cmd engine.SendSignalCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	signal, err := h.e.SendSignal(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, signal, http.StatusOK)
}

func (h handler) setElementVariables(w http.ResponseWriter, r *http.Request) {
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

	if err := h.e.SetElementVariables(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h handler) setProcessVariables(w http.ResponseWriter, r *http.Request) {
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

	if err := h.e.SetProcessVariables(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h handler) setTime(w http.ResponseWriter, r *http.Request) {
	if !h.setTimeEnabled {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	var cmd engine.SetTimeCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	if err := h.e.SetTime(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h handler) suspendProcessInstance(w http.ResponseWriter, r *http.Request) {
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

	if err := h.e.SuspendProcessInstance(r.Context(), cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h handler) unlockJobs(w http.ResponseWriter, r *http.Request) {
	var cmd engine.UnlockJobsCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	count, err := h.e.UnlockJobs(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, common.CountRes{Count: count}, http.StatusOK)
}

func (h handler) unlockTasks(w http.ResponseWriter, r *http.Request) {
	var cmd engine.UnlockTasksCmd
	if err := decodeJSONRequestBody(w, r, &cmd); err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	count, err := h.e.UnlockTasks(r.Context(), cmd)
	if err != nil {
		encodeJSONProblemResponseBody(w, r, err)
		return
	}

	encodeJSONResponseBody(w, r, common.CountRes{Count: count}, http.StatusOK)
}

// query handler

func (h handler) queryElements(w http.ResponseWriter, r *http.Request) {
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

	q := h.e.CreateQuery()
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

func (h handler) queryElementInstances(w http.ResponseWriter, r *http.Request) {
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

	q := h.e.CreateQuery()
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

func (h handler) queryIncidents(w http.ResponseWriter, r *http.Request) {
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

	q := h.e.CreateQuery()
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

func (h handler) queryJobs(w http.ResponseWriter, r *http.Request) {
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

	q := h.e.CreateQuery()
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

func (h handler) queryMessages(w http.ResponseWriter, r *http.Request) {
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

	q := h.e.CreateQuery()
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

func (h handler) queryProcesses(w http.ResponseWriter, r *http.Request) {
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

	q := h.e.CreateQuery()
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

func (h handler) queryProcessInstances(w http.ResponseWriter, r *http.Request) {
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

	q := h.e.CreateQuery()
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

func (h handler) queryTasks(w http.ResponseWriter, r *http.Request) {
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

	q := h.e.CreateQuery()
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

func (h handler) queryVariables(w http.ResponseWriter, r *http.Request) {
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

	q := h.e.CreateQuery()
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
