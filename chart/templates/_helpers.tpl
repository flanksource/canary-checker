{{/*
Expand the name of the chart.
*/}}
{{- define "canary-checker.name" -}}
{{- default "canary-checker" .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "canary-checker.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "canary-checker.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default "canary-checker" .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "chart.labels" -}}
helm.sh/chart: {{ include "canary-checker.chart" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "canary-checker.labels" -}}
{{ include "chart.labels" . }}
{{ include "canary-checker.selectorLabels" . }}
{{- end }}

{{- define "postgresql.labels" -}}
{{ include "chart.labels" . }}
{{ include "postgresql.selectorLabels" . }}
{{- end }}


{{/*
Selector labels
*/}}
{{- define "canary-checker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "canary-checker.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: canary-checker
{{- if .Values.global.labels }}
{{.Values.global.labels | toYaml}}
{{- end }}
{{- end }}

{{- define "postgresql.selectorLabels" -}}
app.kubernetes.io/name: postgresql
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: canary-checker
{{- end }}

{{/*
Image Name
*/}}
{{- define "canary-checker.imageString" -}}
{{ tpl .Values.global.imageRegistry . }}/{{ tpl .Values.image.name . }}{{- if eq (lower .Values.image.type) "full" }}-full{{- end }}:{{ .Values.image.tag }}
{{- end }}

{{- define "canary-checker.postgres.imageString" -}}
{{ tpl .Values.global.imageRegistry . }}/supabase/postgres:14.1.0.89
{{- end }}
