{{/*
Expand the name of the chart.
*/}}
{{- define "pulse.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "pulse.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart label.
*/}}
{{- define "pulse.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels.
*/}}
{{- define "pulse.labels" -}}
helm.sh/chart: {{ include "pulse.chart" . }}
{{ include "pulse.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Selector labels.
*/}}
{{- define "pulse.selectorLabels" -}}
app.kubernetes.io/name: {{ include "pulse.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Return the name of the service account to use.
*/}}
{{- define "pulse.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (printf "%s-sa" (include "pulse.fullname" .)) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
Return the server secret name (Pulse hub env vars).
*/}}
{{- define "pulse.serverSecretName" -}}
{{- $secret := .Values.server.secretEnv -}}
{{- if $secret.name -}}
{{- $secret.name -}}
{{- else -}}
{{- printf "%s-server-env" (include "pulse.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
Return the agent secret name.
*/}}
{{- define "pulse.agentSecretName" -}}
{{- $secret := .Values.agent.secretEnv -}}
{{- if $secret.name -}}
{{- $secret.name -}}
{{- else -}}
{{- printf "%s-agent-env" (include "pulse.fullname" .) -}}
{{- end -}}
{{- end -}}

{{/*
Return the agent service account name.
*/}}
{{- define "pulse.agentServiceAccountName" -}}
{{- if .Values.agent.serviceAccount.create -}}
{{- default (printf "%s-agent" (include "pulse.fullname" .)) .Values.agent.serviceAccount.name -}}
{{- else if .Values.agent.serviceAccount.name -}}
{{- .Values.agent.serviceAccount.name -}}
{{- else -}}
{{- include "pulse.serviceAccountName" . -}}
{{- end -}}
{{- end -}}
