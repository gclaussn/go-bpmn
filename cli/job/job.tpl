Partition: {{ .Partition }}
Id: {{ .Id }}
ElementId: {{ .ElementId }}
ElementInstanceId: {{ .ElementInstanceId }}
ProcessId: {{ .ProcessId }}
ProcessInstanceId: {{ .ProcessInstanceId }}
BpmnElementId: {{ .BpmnElementId }}
CompletedAt: {{ .CompletedAt | formatTime }}
CorrelationKey: {{ .CorrelationKey }}
CreatedAt: {{ .CreatedAt | formatTime }}
CreatedBy: {{ .CreatedBy }}
DueAt: {{ .DueAt | formatTime }}
Error: {{ .Error }}
LockedAt: {{ .LockedAt | formatTime }}
LockedBy: {{ .LockedBy }}
RetryCount: {{ .RetryCount }}
State: {{ .State }}
Type: {{ .Type }}
