{{/*
    Generates common labels
    Usage: {{ include "kubesnap.labels" .}}
*/}}
{{- define "kubesnap.labels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end}}

{{/*
Generate common annotations
Usage: {{ include "kubesnap.annotations" . }}
*/}}
{{- define "kubesnap.annotations" -}}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version }}
kubeclean/disabled: "true"
{{- end }}


{{/*
Returns full name (for consistent naming)
*/}}
{{- define "kubesnap.fullname" -}}
{{- printf "%s" .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end }}

{{/*
Returns namespace
*/}}
{{- define "kubesnap.namespace" -}}
{{- include "kubesnap.fullname" .}}-space
{{- end }}


{{/*
Returns Service account name
*/}}
{{- define "kubesnap.serviceAccount" -}}
{{- include "kubesnap.fullname" .}}-sa
{{- end }}