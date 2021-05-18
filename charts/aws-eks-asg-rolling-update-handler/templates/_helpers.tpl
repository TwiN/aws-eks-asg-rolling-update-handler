{{/*
Create a default app name.
*/}}
{{- define "aws-eks-asg-rolling-update-handler.name" -}}
{{- .Chart.Name -}}
{{- end -}}

{{/*
Create a default namespace.
*/}}
{{- define "aws-eks-asg-rolling-update-handler.namespace" -}}
{{- .Release.Namespace -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "aws-eks-asg-rolling-update-handler.labels" -}}
app.kubernetes.io/name: {{ include "aws-eks-asg-rolling-update-handler.name" . }}
{{- end -}}

{{/*
Create the name of the service account to use.
*/}}
{{- define "aws-eks-asg-rolling-update-handler.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "aws-eks-asg-rolling-update-handler.name" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}