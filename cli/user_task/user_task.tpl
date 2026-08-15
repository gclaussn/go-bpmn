Partition: {{ .Partition }}
Id: {{ .Id }}
Revision: {{ .Revision }}
ElementId: {{ .ElementId }}
ElementInstanceId: {{ .ElementInstanceId }}
ProcessId: {{ .ProcessId }}
ProcessInstanceId: {{ .ProcessInstanceId }}
BpmnElementId: {{ .BpmnElementId }}
CorrelationKey: {{ .CorrelationKey }}
CreatedAt: {{ .CreatedAt | formatTime }}
CreatedBy: {{ .CreatedBy }}
State: {{ .State }}
{{- if .Tags }}
Tags:
  {{- range $tag := .Tags }}
  {{ $tag.Name }}: {{ $tag.Value }}
  {{- end }}
{{- end }}
UpdatedAt: {{ .UpdatedAt | formatTime }}
UpdatedBy: {{ .UpdatedBy }}
