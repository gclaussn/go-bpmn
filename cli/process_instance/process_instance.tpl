Partition: {{ .Partition }}
Id: {{ .Id }}
{{- if .ParentId }}
ParentId: {{ .ParentId }}
{{- end }}
{{- if .RootId }}
RootId: {{ .RootId }}
{{- end }}
ProcessId: {{ .ProcessId }}
BpmnProcessId: {{ .BpmnProcessId }}
CorrelationKey: {{ .CorrelationKey }}
CreatedAt: {{ .CreatedAt | formatTime }}
CreatedBy: {{ .CreatedBy }}
EndedAt: {{ .EndedAt | formatTimeOrNil }}
StartedAt: {{ .StartedAt | formatTimeOrNil }}
State: {{ .State }}
{{- if .Tags }}
Tags:
  {{- range $tag := .Tags }}
  {{ $tag.Name }}: {{ $tag.Value }}
  {{- end }}
{{- end }}
Version: {{ .Version }}
