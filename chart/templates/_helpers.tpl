{{/*
Expand the name of the chart.
*/}}
{{- define "canary-checker.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "canary-checker.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "canary-checker.labels" -}}
helm.sh/chart: {{ include "canary-checker.chart" . }}
{{ include "canary-checker.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "canary-checker.selectorLabels" -}}
app.kubernetes.io/name: {{ include "canary-checker.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: canary-checker
{{- end }}
