{{/*
Expand the name of the chart.
*/}}
{{- define "ytsaurus-excel-chart.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "ytsaurus-excel-chart.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "ytsaurus-excel-chart.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}

{{- define "ytsaurus-excel-chart.commonLabels" -}}
helm.sh/chart: {{ include "ytsaurus-excel-chart.chart" . }}
{{ include "ytsaurus-excel-chart.commonSelectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}





{{- define "ytsaurus-excel-chart.exporterLabels" -}}
{{ include "ytsaurus-excel-chart.exporterSelectorLabels" . }}
{{ include "ytsaurus-excel-chart.commonLabels" .}}
{{- end }}

{{- define "ytsaurus-excel-chart.uploaderLabels" -}}
{{ include "ytsaurus-excel-chart.uploaderSelectorLabels" . }}
{{ include "ytsaurus-excel-chart.commonLabels" .}}
{{- end }}

{{/*
Selector labels
*/}}

{{- define "ytsaurus-excel-chart.commonSelectorLabels" -}}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "ytsaurus-excel-chart.exporterSelectorLabels" -}}
app.kubernetes.io/name: "ytsaurus-excel-exporter"
{{ include "ytsaurus-excel-chart.commonSelectorLabels" . }}
{{- end }}

{{- define "ytsaurus-excel-chart.uploaderSelectorLabels" -}}
app.kubernetes.io/name: "ytsaurus-excel-uploader"
{{ include "ytsaurus-excel-chart.commonSelectorLabels" . }}
{{- end }}


