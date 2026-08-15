Id: {{ .Id }}
CorrelationKey: {{ .CorrelationKey }}
CreatedAt: {{ .CreatedAt | formatTime }}
CreatedBy: {{ .CreatedBy }}
ExpiresAt: {{ .ExpiresAt | formatTimeOrNil }}
IsCorrelated: {{ .IsCorrelated }}
Name: {{ .Name }}
UniqueKey: {{ .UniqueKey }}
