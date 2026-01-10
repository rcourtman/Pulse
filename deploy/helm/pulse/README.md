# pulse

![Version: 5.0.13](https://img.shields.io/badge/Version-5.0.13-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 5.0.13](https://img.shields.io/badge/AppVersion-5.0.13-informational?style=flat-square)

Helm chart for deploying the Pulse hub and optional Docker monitoring agent.

**Homepage:** <https://github.com/rcourtman/Pulse>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Pulse Maintainers | <pulse@rcourtman.dev> |  |

## Source Code

* <https://github.com/rcourtman/Pulse>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| agent.affinity | object | `{}` |  |
| agent.args | list | `[]` |  |
| agent.dockerSocket.enabled | bool | `true` |  |
| agent.dockerSocket.hostPathType | string | `"Socket"` |  |
| agent.dockerSocket.path | string | `"/var/run/docker.sock"` |  |
| agent.enabled | bool | `false` |  |
| agent.envFrom | list | `[]` |  |
| agent.env[0].name | string | `"PULSE_URL"` |  |
| agent.env[0].value | string | `"http://pulse:7655"` |  |
| agent.extraEnv | list | `[]` |  |
| agent.extraEnvFrom | list | `[]` |  |
| agent.extraVolumeMounts | list | `[]` |  |
| agent.extraVolumes | list | `[]` |  |
| agent.healthPort | int | `9191` |  |
| agent.image.pullPolicy | string | `"IfNotPresent"` |  |
| agent.image.repository | string | `"ghcr.io/rcourtman/pulse-docker-agent"` |  |
| agent.image.tag | string | `""` |  |
| agent.kind | string | `"DaemonSet"` |  |
| agent.livenessProbe.enabled | bool | `true` |  |
| agent.livenessProbe.failureThreshold | int | `3` |  |
| agent.livenessProbe.initialDelaySeconds | int | `5` |  |
| agent.livenessProbe.path | string | `"/healthz"` |  |
| agent.livenessProbe.periodSeconds | int | `10` |  |
| agent.livenessProbe.timeoutSeconds | int | `3` |  |
| agent.nodeSelector | object | `{}` |  |
| agent.podAnnotations | object | `{}` |  |
| agent.podLabels | object | `{}` |  |
| agent.podSecurityContext | object | `{}` |  |
| agent.readinessProbe.enabled | bool | `true` |  |
| agent.readinessProbe.failureThreshold | int | `3` |  |
| agent.readinessProbe.initialDelaySeconds | int | `5` |  |
| agent.readinessProbe.path | string | `"/readyz"` |  |
| agent.readinessProbe.periodSeconds | int | `5` |  |
| agent.readinessProbe.timeoutSeconds | int | `3` |  |
| agent.replicaCount | int | `1` |  |
| agent.resources | object | `{}` |  |
| agent.secretEnv.create | bool | `false` |  |
| agent.secretEnv.data | object | `{}` |  |
| agent.secretEnv.keys | list | `[]` |  |
| agent.secretEnv.name | string | `""` |  |
| agent.securityContext.privileged | bool | `false` |  |
| agent.securityContext.runAsGroup | int | `0` |  |
| agent.securityContext.runAsUser | int | `0` |  |
| agent.serviceAccount.annotations | object | `{}` |  |
| agent.serviceAccount.create | bool | `false` |  |
| agent.serviceAccount.name | string | `""` |  |
| agent.tolerations | list | `[]` |  |
| containerSecurityContext.enabled | bool | `true` |  |
| containerSecurityContext.runAsGroup | int | `1000` |  |
| containerSecurityContext.runAsNonRoot | bool | `true` |  |
| containerSecurityContext.runAsUser | int | `1000` |  |
| fullnameOverride | string | `""` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"rcourtman/pulse"` |  |
| image.tag | string | `""` |  |
| imagePullSecrets | list | `[]` |  |
| ingress.annotations | object | `{}` |  |
| ingress.className | string | `""` |  |
| ingress.enabled | bool | `false` |  |
| ingress.hosts[0].host | string | `"pulse.local"` |  |
| ingress.hosts[0].paths[0].path | string | `"/"` |  |
| ingress.hosts[0].paths[0].pathType | string | `"Prefix"` |  |
| ingress.tls | list | `[]` |  |
| monitoring.serviceMonitor.enabled | bool | `false` |  |
| monitoring.serviceMonitor.interval | string | `"30s"` |  |
| monitoring.serviceMonitor.labels | object | `{}` |  |
| monitoring.serviceMonitor.metricRelabelings | list | `[]` |  |
| monitoring.serviceMonitor.path | string | `"/metrics"` |  |
| monitoring.serviceMonitor.relabelings | list | `[]` |  |
| monitoring.serviceMonitor.scrapeTimeout | string | `"10s"` |  |
| nameOverride | string | `""` |  |
| persistence.accessModes[0] | string | `"ReadWriteOnce"` |  |
| persistence.annotations | object | `{}` |  |
| persistence.enabled | bool | `true` |  |
| persistence.existingClaim | string | `""` |  |
| persistence.size | string | `"8Gi"` |  |
| persistence.storageClass | string | `""` |  |
| podAnnotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| podSecurityContext.enabled | bool | `true` |  |
| podSecurityContext.fsGroup | int | `1000` |  |
| replicaCount | int | `1` |  |
| server.affinity | object | `{}` |  |
| server.envFrom | list | `[]` |  |
| server.env[0].name | string | `"TZ"` |  |
| server.env[0].value | string | `"UTC"` |  |
| server.extraEnv | list | `[]` |  |
| server.extraEnvFrom | list | `[]` |  |
| server.extraVolumeMounts | list | `[]` |  |
| server.extraVolumes | list | `[]` |  |
| server.livenessProbe.enabled | bool | `true` |  |
| server.livenessProbe.failureThreshold | int | `3` |  |
| server.livenessProbe.initialDelaySeconds | int | `20` |  |
| server.livenessProbe.path | string | `"/"` |  |
| server.livenessProbe.periodSeconds | int | `30` |  |
| server.livenessProbe.timeoutSeconds | int | `5` |  |
| server.nodeSelector | object | `{}` |  |
| server.podAnnotations | object | `{}` |  |
| server.podLabels | object | `{}` |  |
| server.podSecurityContext | object | `{}` |  |
| server.readinessProbe.enabled | bool | `true` |  |
| server.readinessProbe.failureThreshold | int | `3` |  |
| server.readinessProbe.initialDelaySeconds | int | `10` |  |
| server.readinessProbe.path | string | `"/"` |  |
| server.readinessProbe.periodSeconds | int | `10` |  |
| server.readinessProbe.timeoutSeconds | int | `5` |  |
| server.resources | object | `{}` |  |
| server.secretEnv.create | bool | `false` |  |
| server.secretEnv.data | object | `{}` |  |
| server.secretEnv.keys | list | `[]` |  |
| server.secretEnv.name | string | `""` |  |
| server.securityContext | object | `{}` |  |
| server.tolerations | list | `[]` |  |
| service.annotations | object | `{}` |  |
| service.externalTrafficPolicy | string | `"Cluster"` |  |
| service.loadBalancerIP | string | `""` |  |
| service.port | int | `7655` |  |
| service.type | string | `"ClusterIP"` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| strategy.type | string | `"RollingUpdate"` |  |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
