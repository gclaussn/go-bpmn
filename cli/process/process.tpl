Id: {{ .Id }}
{{- if .BpmnCollaborationId }}
BpmnCollaborationId: {{ .BpmnCollaborationId }}
BpmnParticipantId: {{ .BpmnParticipantId }}
BpmnParticipantName: {{ .BpmnParticipantName }}
{{- end }}
BpmnProcessId: {{ .BpmnProcessId }}
CreatedAt: {{ .CreatedAt | formatTime }}
CreatedBy: {{ .CreatedBy }}
Parallelism: {{ .Parallelism }}
{{- if .Tags }}
Tags:
  {{- range $tag := .Tags }}
  {{ $tag.Name }}: {{ $tag.Value }}
  {{- end }}
{{- end }}
Version: {{ .Version }}
