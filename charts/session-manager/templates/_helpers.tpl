{{/*
Expand the name of the chart.
*/}}
{{- define "session-manager.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "session-manager.fullname" -}}
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
Namespace for all resources. Defaults to the release namespace.
See https://github.com/helm/helm/issues/5358
*/}}
{{- define "session-manager.namespace" -}}
{{- .Values.namespace | default .Release.Namespace }}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "session-manager.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Version of the deployed application: the image tag if set, else the chart's appVersion.
*/}}
{{- define "session-manager.version" -}}
{{- (.Values.image.tag | default .Chart.AppVersion) | toString | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels. Each workload adds its app.kubernetes.io/component label.
*/}}
{{- define "session-manager.labels" -}}
helm.sh/chart: {{ include "session-manager.chart" . }}
{{ include "session-manager.selectorLabels" . }}
{{- with include "session-manager.version" . }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end }}

{{/*
Selector labels, shared by all workloads of the release. Each workload's selector adds
app.kubernetes.io/component. Do not change them: a Deployment's selector cannot be changed
on upgrade.
*/}}
{{- define "session-manager.selectorLabels" -}}
app.kubernetes.io/name: {{ include "session-manager.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Name of the service account to use.
*/}}
{{- define "session-manager.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "session-manager.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Image reference: registry and repository, then the digest if set, else the tag. The tag
defaults to the chart's appVersion, the image version this chart release is made for.
Call it with (tuple .Values.image .Chart.AppVersion).
*/}}
{{- define "session-manager.image" -}}
{{- $image := index . 0 -}}
{{- $appVersion := index . 1 -}}
{{- $repository := $image.repository -}}
{{- if $image.registry -}}{{- $repository = printf "%s/%s" $image.registry $image.repository -}}{{- end -}}
{{- if $image.digest -}}
{{- printf "%s@%s" $repository (toString $image.digest) -}}
{{- else -}}
{{- printf "%s:%s" $repository (toString ($image.tag | default $appVersion)) -}}
{{- end -}}
{{- end }}

{{/*
Container command: image.command as a string or a list. Empty: the image's entrypoint.
*/}}
{{- define "session-manager.command" -}}
{{- with .Values.image.command }}
command:
  {{- if kindIs "string" . }}
  - {{ . | quote }}
  {{- else }}
  {{- toYaml . | nindent 2 }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
Environment of all Session Manager containers: the pod's addresses for logs and telemetry,
then extraEnvs and extraEnvsFrom.
*/}}
{{- define "session-manager.env" -}}
env:
  - name: MY_POD_IP
    valueFrom:
      fieldRef:
        apiVersion: v1
        fieldPath: status.podIP
  - name: K8S_NODE_NAME
    valueFrom:
      fieldRef:
        fieldPath: spec.nodeName
  - name: K8S_NODE_IP
    valueFrom:
      fieldRef:
        fieldPath: status.hostIP
  {{- with .Values.extraEnvs }}
  {{- toYaml . | nindent 2 }}
  {{- end }}
{{- with .Values.extraEnvsFrom }}
envFrom:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{/*
Volume mounts of all Session Manager containers. The binary reads its configuration from
/etc/session-manager, whatever the chart or release name.
*/}}
{{- define "session-manager.volumeMounts" -}}
volumeMounts:
  - name: config
    mountPath: /etc/session-manager
    readOnly: true
  {{- if .Values.tmpVolume.enabled }}
  - name: tmp
    mountPath: /tmp
  {{- end }}
  {{- with .Values.extraVolumeMounts }}
  {{- toYaml . | nindent 2 }}
  {{- end }}
{{- end }}

{{/*
Volumes of all pods, given the name of the configuration ConfigMap.
Usage: include "session-manager.volumes" (dict "ctx" . "configMap" "<name>")
*/}}
{{- define "session-manager.volumes" -}}
{{- $ctx := .ctx -}}
volumes:
  - name: config
    configMap:
      name: {{ .configMap }}
  {{- if $ctx.Values.tmpVolume.enabled }}
  - name: tmp
    emptyDir:
      {{- with $ctx.Values.tmpVolume.sizeLimit }}
      sizeLimit: {{ . }}
      {{- end }}
  {{- end }}
  {{- with $ctx.Values.extraVolumes }}
  {{- toYaml . | nindent 2 }}
  {{- end }}
{{- end }}

{{/*
Annotations with every value rendered as a string. Values given with --set can be numbers
or booleans, which Kubernetes rejects in annotations.
*/}}
{{- define "session-manager.annotations" -}}
{{- range $key, $value := . }}
{{ $key }}: {{ $value | toString | quote }}
{{- end }}
{{- end }}

{{/*
Pod-level settings shared by the API server and the housekeeper.
*/}}
{{- define "session-manager.podSettings" -}}
{{- with .Values.imagePullSecrets }}
imagePullSecrets:
  {{- toYaml . | nindent 2 }}
{{- end }}
serviceAccountName: {{ include "session-manager.serviceAccountName" . }}
# Session Manager does not call the Kubernetes API.
automountServiceAccountToken: false
enableServiceLinks: {{ .Values.pod.enableServiceLinks }}
{{- with .Values.pod.priorityClassName }}
priorityClassName: {{ . }}
{{- end }}
{{- with .Values.pod.terminationGracePeriodSeconds }}
terminationGracePeriodSeconds: {{ . }}
{{- end }}
{{- with .Values.pod.securityContext }}
securityContext:
  {{- toYaml . | nindent 2 }}
{{- end }}
{{- end }}

{{/*
Whether the configuration ConfigMap is immutable. configMap.immutable is preferred;
config.isImmutable is still honoured for compatibility with older values files.
*/}}
{{- define "session-manager.configImmutable" -}}
{{- if or .Values.configMap.immutable .Values.config.isImmutable -}}
true
{{- end -}}
{{- end }}

{{/*
Name of the configuration ConfigMap of the API server and the housekeeper. An immutable
ConfigMap gets a hash in its name, so that a changed configuration creates a new ConfigMap
instead of failing the upgrade. config.yaml (templates/configmap.yaml) is made of the values
under config and the chart version, so the hash covers these.
The name differs from "<fullname>-config", which chart versions before 0.11.0 created as a
Helm hook: Helm cannot adopt that object, so reusing the name would fail the upgrade.
*/}}
{{- define "session-manager.configMapName" -}}
{{- if include "session-manager.configImmutable" . -}}
{{- $hash := printf "%s\n%s" .Chart.Version (toYaml .Values.config) | sha256sum | trunc 10 }}
{{- printf "%s-%s" (include "session-manager.fullname" . | trunc 52 | trimSuffix "-") $hash }}
{{- else -}}
{{- include "session-manager.fullname" . }}
{{- end -}}
{{- end }}

{{/*
Name of the ConfigMap of the migration job. The job runs as a Helm hook before the
release's own resources exist, so it gets its own copy of the configuration, also as a hook.
*/}}
{{- define "session-manager.migrateConfigMapName" -}}
{{- printf "%s-config" (include "session-manager.fullname" . | trunc 56 | trimSuffix "-") }}
{{- end }}
