{{- define "setResources" }}
{{- if .resources }}
{{- if or .resources.requests .resources.limits }}

{{- $requestsValues := .resources.requests | values | compact }}
{{- $limitsValues   := .resources.limits   | values | compact }}

{{- if $limitsValues }}
limits:
  {{- if .resources.limits.memory }}
  memory: {{ .resources.limits.memory }}
  {{- end }}
  {{- if .resources.limits.cpu }}
  cpu: {{ .resources.limits.cpu }}
  {{- end }}
{{- end }}

{{- if $requestsValues }}
requests:
  {{- if .resources.requests.memory }}
  memory: {{ .resources.requests.memory }}
  {{- end }}
  {{- if .resources.requests.cpu }}
  cpu: {{ .resources.requests.cpu }}
  {{- end }}
{{- end }}

{{- end }}
{{- end }}
{{- end }}
