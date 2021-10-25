{{- define "setResources" -}}

requests:
  {{- if .resources.requests.memory }}
  memory: {{ .resources.requests.memory }}
  {{- end }}
  {{- if .resources.requests.cpu }}
  cpu: {{ .resources.requests.cpu }}
  {{- end }}
limits:
  {{- if .resources.limits.memory }}
  memory: {{ .resources.limits.memory }}
  {{- end }}
  {{- if .resources.limits.cpu }}
  cpu: {{ .resources.limits.cpu }}
  {{- end }}

{{- end -}}
