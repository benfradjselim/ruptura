{{/*
Expand the name of the chart.
*/}}
{{- define "ruptura.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "ruptura.fullname" -}}
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
Create chart label.
*/}}
{{- define "ruptura.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels.
*/}}
{{- define "ruptura.labels" -}}
helm.sh/chart: {{ include "ruptura.chart" . }}
{{ include "ruptura.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels.
*/}}
{{- define "ruptura.selectorLabels" -}}
app.kubernetes.io/name: {{ include "ruptura.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Engine selector labels (selectorLabels plus the engine component, so engine
selectors never match UI pods).
*/}}
{{- define "ruptura.engineSelectorLabels" -}}
{{ include "ruptura.selectorLabels" . }}
app.kubernetes.io/component: engine
{{- end }}

{{/*
Engine labels (common labels plus the engine component).
*/}}
{{- define "ruptura.engineLabels" -}}
{{ include "ruptura.labels" . }}
app.kubernetes.io/component: engine
{{- end }}

{{/*
Engine pod security context; runAsUser and fsGroup are omitted on OpenShift,
where the restricted SCC assigns them.
*/}}
{{- define "ruptura.podSecurityContext" -}}
{{- if .Values.openshift.enabled -}}
{{- toYaml (omit .Values.podSecurityContext "runAsUser" "fsGroup") -}}
{{- else -}}
{{- toYaml .Values.podSecurityContext -}}
{{- end -}}
{{- end }}

{{/*
Thanos querier URL: explicit value wins, OpenShift gets the in-cluster default.
*/}}
{{- define "ruptura.thanosUrl" -}}
{{- if .Values.thanos.url -}}
{{- .Values.thanos.url -}}
{{- else if .Values.openshift.enabled -}}
https://thanos-querier.openshift-monitoring.svc:9091
{{- end -}}
{{- end }}

{{/*
ServiceAccount name.
*/}}
{{- define "ruptura.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "ruptura.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image reference.
*/}}
{{- define "ruptura.image" -}}
{{- if .Values.image.digest -}}
{{ .Values.image.repository }}@{{ .Values.image.digest }}
{{- else -}}
{{- $tag := .Values.image.tag | default (printf "v%s" .Chart.AppVersion) -}}
{{ .Values.image.repository }}:{{ $tag }}
{{- end }}
{{- end }}

{{/*
UI image reference.
*/}}
{{- define "ruptura.uiImage" -}}
{{- $tag := .Values.ui.image.tag | default (printf "ui-v%s" .Chart.AppVersion) -}}
{{ .Values.ui.image.repository }}:{{ $tag }}
{{- end }}
