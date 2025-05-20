package server

const (
	ContentTypeJson        = "application/json"
	ContentTypeProblemJson = "application/problem+json"
	ContentTypeXml         = "text/xml"

	HeaderAuthorization = "Authorization"
	HeaderContentType   = "Content-Type"

	PathElementsQuery = "/elements/query"

	PathElementInstancesQuery     = "/element-instances/query"
	PathElementInstancesVariables = "/element-instances/{partition}/{id}/variables"

	PathIncidentsQuery   = "/incidents/query"
	PathIncidentsResolve = "/incidents/{partition}/{id}/resolve"

	PathJobsComplete = "/jobs/{partition}/{id}/complete"
	PathJobsLock     = "/jobs/lock"
	PathJobsQuery    = "/jobs/query"
	PathJobsUnlock   = "/jobs/unlock"

	PathProcesses        = "/processes"
	PathProcessesBpmnXml = "/processes/{id}/bpmn-xml"
	PathProcessesQuery   = "/processes/query"

	PathProcessInstances          = "/process-instances"
	PathProcessInstancesQuery     = "/process-instances/query"
	PathProcessInstancesResume    = "/process-instances/{partition}/{id}/resume"
	PathProcessInstancesSuspend   = "/process-instances/{partition}/{id}/suspend"
	PathProcessInstancesVariables = "/process-instances/{partition}/{id}/variables"

	PathTasksExecute = "/tasks/execute"
	PathTasksQuery   = "/tasks/query"
	PathTasksUnlock  = "/tasks/unlock"

	PathVariablesQuery = "/variables/query"

	PathReadiness = "/readiness"
	PathTime      = "/time"

	QueryLimit  = "limit"
	QueryNames  = "names"
	QueryOffset = "offset"
)
