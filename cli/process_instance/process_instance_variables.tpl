{{range $variable := . -}}
Variable:
  Name: {{ $variable.Name }}
  Data:
    Encoding: {{ $variable.Data.Encoding }}
    IsEncrypted: {{ $variable.Data.IsEncrypted }}
    Value: {{ $variable.Data.Value }}
{{ end -}}
