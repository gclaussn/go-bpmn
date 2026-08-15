{{range $variable := . -}}
Variable:
  BpmnElementId: {{ $variable.BpmnElementId }}
  Name: {{ $variable.Name }}
  Data:
    Encoding: {{ $variable.Data.Encoding }}
    IsEncrypted: {{ $variable.Data.IsEncrypted }}
    Value: {{ $variable.Data.Value }}
{{ end -}}
