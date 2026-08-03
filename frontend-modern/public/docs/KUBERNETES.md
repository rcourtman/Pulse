# Pulse on Kubernetes

This guide explains how to deploy the Pulse Server (Hub) and Pulse Agents on Kubernetes clusters, including immutable distributions like Talos Linux.

> **Navigation note (v6):** Kubernetes cluster and node resources appear on the **Infrastructure** page, while pods appear on the **Workloads** page. The legacy `/kubernetes` URL redirects to `/workloads?type=k8s`.

## Prerequisites

- A Kubernetes cluster (v1.19+)
- `helm` (v3+) installed locally
- `kubectl` configured to talk to your cluster

## 1. Deploying the Pulse Server

The Pulse Server is the central hub that collects metrics and manages agents.

### Option A: Using Helm (Recommended)

1.  Add the Pulse Helm repository:
    ```bash
    helm repo add pulse https://rcourtman.github.io/Pulse
    helm repo update
    ```

2.  Install the chart:
    ```bash
    helm upgrade --install pulse pulse/pulse \
      --namespace pulse \
      --create-namespace \
      --set persistence.enabled=true \
      --set persistence.size=10Gi
    ```

    > **Note**: For production, ensure you configure a proper `persistence.storageClass` or `strategy.type=Recreate` if using ReadWriteOnce (RWO) volumes. The chart's default `strategy.type` is `RollingUpdate`, which can hit Multi-Attach errors with RWO PVCs during upgrade.

### Option B: Generating Static Manifests (For Talos / GitOps)

If you cannot use Helm directly on the cluster (e.g., restricted Talos environment), you can generate standard Kubernetes YAML manifests:

```bash
helm repo add pulse https://rcourtman.github.io/Pulse
helm repo update
helm template pulse pulse/pulse \
  --namespace pulse \
  --set persistence.enabled=true \
  > pulse-server.yaml
```

You can then apply this file:

```bash
kubectl apply -f pulse-server.yaml
```

## 2. Deploying the Pulse Agent

### Helm Chart Agent Mode

The Helm chart includes an optional `agent` section that deploys the unified `pulse-agent`.
By default, this workload runs in container-monitoring mode (`--enable-docker --enable-host=false`).

For Kubernetes monitoring, use a custom DaemonSet as shown below.

### OpenShift profile (Helm)

The chart has an SCC-compatible OpenShift profile for the Pulse server and an
optional cluster-level Kubernetes collector:

```bash
export PULSE_TOKEN='replace-with-a-kubernetes-report-token'

kubectl create namespace pulse --dry-run=client -o yaml | kubectl apply -f -
kubectl -n pulse create secret generic pulse-server-env \
  --from-literal=API_TOKENS="${PULSE_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f -
kubectl -n pulse create secret generic pulse-agent-env \
  --from-literal=PULSE_TOKEN="${PULSE_TOKEN}" \
  --dry-run=client -o yaml | kubectl apply -f -

helm upgrade --install pulse pulse/pulse \
  --namespace pulse \
  --set openShift.enabled=true \
  --set openShift.kubernetesAgent.enabled=true \
  --set openShift.kubernetesAgent.clusterID=my-openshift-cluster \
  --set server.secretEnv.name=pulse-server-env \
  --set 'server.secretEnv.keys[0]=API_TOKENS' \
  --set agent.secretEnv.name=pulse-agent-env \
  --set 'agent.secretEnv.keys[0]=PULSE_TOKEN'
```

Using pre-created Secrets keeps the token out of Helm release values. For an
agent reporting to an existing external Pulse server, omit the server Secret
and override `agent.env[0].value` with that server's reachable `PULSE_URL`.

The profile deliberately:

- lets the OpenShift SCC assign the server and agent UID, GID, and filesystem
  group instead of pinning `1000` or `0`;
- runs one non-privileged agent replica with `--enable-kubernetes` and
  `--enable-host=false`;
- does not mount `/var/run/docker.sock` (OpenShift uses CRI-O);
- creates a dedicated service account and read-only ClusterRole/Binding for
  the Kubernetes objects Pulse collects; and
- uses a stable cluster agent ID, configurable through
  `openShift.kubernetesAgent.clusterID`.

The default role intentionally does not grant Kubernetes `secrets` or
`nodes/proxy`.
Secret metadata inventory and direct kubelet-summary fallback therefore remain
unavailable under this least-privilege profile; OpenShift's metrics API supplies
the normal node and pod usage path.

Standard Kubernetes resources—including nodes, pods, Deployments, StatefulSets,
DaemonSets, Jobs, Services, Ingresses, storage, policy, RBAC summaries, events,
and metrics—are collected. OpenShift-native Routes and DeploymentConfigs are
not yet modeled; workloads managed only by those APIs may appear as pods
without their OpenShift controller.

### Unified Agent on Kubernetes (DaemonSet)

To monitor Kubernetes resources, run the unified agent as a DaemonSet and enable the Kubernetes module.

**Recommended options:**
- **Kubernetes-only monitoring**: `PULSE_ENABLE_KUBERNETES=true` and `PULSE_ENABLE_HOST=false` (no host mounts required).
- **Kubernetes + node metrics**: `PULSE_ENABLE_KUBERNETES=true` and `PULSE_ENABLE_HOST=true` (requires host mounts and privileged mode).

#### Minimal DaemonSet Example

This uses the main `rcourtman/pulse` image but runs the `pulse-agent` binary directly.

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: pulse-agent
  namespace: pulse
spec:
  selector:
    matchLabels:
      app: pulse-agent
  template:
    metadata:
      labels:
        app: pulse-agent
    spec:
      serviceAccountName: pulse-agent
      containers:
        - name: pulse-agent
          image: rcourtman/pulse:latest
          # /usr/local/bin/pulse-agent is an arch-resolved symlink in the
          # main Pulse image, so this manifest works on both amd64 and
          # arm64 nodes without changes.
          command: ["/usr/local/bin/pulse-agent"]
          args:
            - --enable-kubernetes
          env:
            - name: PULSE_URL
              value: "http://pulse-server.pulse.svc.cluster.local:7655"
            - name: PULSE_TOKEN
              value: "YOUR_API_TOKEN_HERE"
            - name: PULSE_AGENT_ID
              value: "my-k8s-cluster"
            - name: PULSE_ENABLE_HOST
              value: "false"
            - name: PULSE_KUBE_INCLUDE_ALL_PODS
              value: "true"
            - name: PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS
              value: "true"
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
          resources:
            requests:
              cpu: 50m
              memory: 128Mi
            limits:
              memory: 512Mi
      tolerations:
        - operator: Exists
```

> **Note for ARM64 clusters**: The `/usr/local/bin/pulse-agent` symlink in the
> main image resolves to the correct bundled binary for both amd64 and arm64.

Use a token scoped for the agent:
- `kubernetes:report` for Kubernetes reporting
- `agent:report` if you enable host metrics

#### Important DaemonSet Configuration

##### PULSE_AGENT_ID (Required for DaemonSets)

When running as a DaemonSet, all pods share the same API token but need a unified identity. Without `PULSE_AGENT_ID`, each pod auto-generates a unique ID (e.g., `mac-xxxxx`), causing token conflicts:

```text
API token is already in use by agent "mac-aa5496fed726". Each Kubernetes agent must use a unique API token.
```

Set `PULSE_AGENT_ID` to a shared cluster name so all pods report as one logical agent:

```yaml
- name: PULSE_AGENT_ID
  value: "my-k8s-cluster"
```

##### Resource Visibility Flags

By default, Pulse only shows resources with problems (unhealthy pods, failing deployments). To see all resources:

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `PULSE_KUBE_INCLUDE_ALL_PODS` | Show all non-succeeded pods, not just problematic ones | `false` |
| `PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS` | Show all deployments, not just those with issues | `false` |

For most monitoring use cases, set both to `true`:

```yaml
- name: PULSE_KUBE_INCLUDE_ALL_PODS
  value: "true"
- name: PULSE_KUBE_INCLUDE_ALL_DEPLOYMENTS
  value: "true"
```

See [UNIFIED_AGENT.md](UNIFIED_AGENT.md) for all available configuration options.

#### Add Host Metrics (Optional)

If you want node CPU/memory/disk metrics, add privileged mode plus host mounts:

```yaml
          env:
            - name: PULSE_ENABLE_HOST
              value: "true"
            - name: HOST_PROC
              value: "/host/proc"
            - name: HOST_SYS
              value: "/host/sys"
            - name: HOST_ETC
              value: "/host/etc"
          securityContext:
            privileged: true
          volumeMounts:
            - name: host-proc
              mountPath: /host/proc
              readOnly: true
            - name: host-sys
              mountPath: /host/sys
              readOnly: true
            - name: host-root
              mountPath: /host/root
              readOnly: true
      volumes:
        - name: host-proc
          hostPath:
            path: /proc
        - name: host-sys
          hostPath:
            path: /sys
        - name: host-root
          hostPath:
            path: /
```

#### RBAC

The Kubernetes agent uses the in-cluster API and needs read access to cluster resources (nodes, pods, deployments, etc.). Create a read-only `ClusterRole` and bind it to the `pulse-agent` service account.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: pulse-agent
  namespace: pulse
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pulse-agent-read
rules:
  - apiGroups: [""]
    resources: ["nodes", "pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments"]
    verbs: ["get", "list", "watch"]
  # Optional (Recovery): VolumeSnapshots and Velero backups.
  # These rules are safe to include even if the APIs are not installed; the agent will
  # feature-detect and ignore 404/403 responses.
  - apiGroups: ["snapshot.storage.k8s.io"]
    resources: ["volumesnapshots"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["velero.io"]
    resources: ["backups"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: pulse-agent-read
subjects:
  - kind: ServiceAccount
    name: pulse-agent
    namespace: pulse
roleRef:
  kind: ClusterRole
  name: pulse-agent-read
  apiGroup: rbac.authorization.k8s.io
```

## 3. Talos Linux Specifics

Talos Linux is immutable, so you cannot install the agent via the shell script. Use the DaemonSet approach above.

### Agent Configuration for Talos
- **Storage**: Talos mounts the ephemeral OS on `/`. Persistent data is usually in `/var`. The Pulse agent generally doesn't store state, but if it did, ensure it maps to a persistent path.
- **Network**: The agent will report the Pod IP by default. To report the Node IP, set `PULSE_REPORT_IP` using the Downward API:

    Add this to the DaemonSet `env` section:
    ```yaml
    - name: PULSE_REPORT_IP
      valueFrom:
        fieldRef:
          fieldPath: status.hostIP
    ```

## 4. Troubleshooting

- **Agent not showing in UI**: Check logs for the DaemonSet pods, for example: `kubectl logs -l app=pulse-agent -n pulse`.
- **"Permission Denied" on metrics**: Ensure `securityContext.privileged: true` is set or proper capabilities are added.
- **Connection Refused**: Ensure `PULSE_URL` is correct and reachable from the agent pods.
