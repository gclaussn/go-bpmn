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
EndedAt: {{ .EndedAt | formatTime }}
StartedAt: {{ .StartedAt | formatTime }}
State: {{ .State }}
{{- if .Tags }}
Tags:
  {{- range $tag := .Tags }}
  {{ $tag.Name }}: {{ $tag.Value }}
  {{- end }}
{{- end }}
Version: {{ .Version }}
