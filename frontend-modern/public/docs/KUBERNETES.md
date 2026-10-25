# Pulse on Kubernetes

This guide explains how to deploy the Pulse Server (Hub) and Pulse Agents on Kubernetes clusters, including immutable distributions like Talos Linux.

> **Navigation note (v6):** Kubernetes has its own page at `/kubernetes`, with tabs for nodes, workloads (pods), services, storage, configuration, and events. Pods appear on the **Workloads** tab (`/kubernetes/workloads`).

## Prerequisites

- A Kubernetes cluster (v1.19+)
- `helm` (v3+) installed locally
- `kubectl` configured to talk to your cluster

## Proxmox backup safety

**A Kubernetes-hosted Pulse server can still send QEMU Guest Agent requests to
Proxmox VMs, including with API-only monitoring.** The Kubernetes-only collector
below is a separate component: stopping its DaemonSet does not stop the server's
Proxmox polling. Read [Backup safety](VM_DISK_MONITORING.md#backup-safety) before
changing an affected installation.

For an affected server, prepare a controller-aware pause **before a planned
freeze-enabled backup**, using your cluster's normal maintenance procedure.
This is not incident recovery or an automatic backup hook. If a backup is
already in progress, a guest is unresponsive or an operation's state is unknown,
do not restart pods or send guest-agent probes to diagnose it. Use the
guest/platform's recovery procedure and preserve the original evidence.

1. **Identify every Pulse server and its owner.** Confirm the cluster,
   namespace, controller and existing pods for each server polling the affected
   guests, including servers outside this cluster. Record the actual desired
   replica count and whether each deployment controller, autoscaler, GitOps
   reconciler and automatic updater was active. Keep that record private.
   The chart's default is not evidence of your installed state. An unknown
   owner or another polling server prevents a confirmed pause.
2. **Prevent replacement, then stop the server through its owning controller.**
   Use the controller's supported maintenance controls, including suspending
   reconciliation or autoscaling where needed, before deliberately scaling the
   server to zero. Deleting a pod only invites its controller to replace it;
   pausing a Deployment rollout does not stop existing pods. Confirm the
   desired count remains zero, every previous server process has terminated
   normally and no replacement is polling. A force-deleted, terminating or
   unreachable pod, a failed shutdown or missing termination evidence is not
   a confirmed stop. Do not force deletion, uninstall Helm, delete PVCs or
   recreate Secrets to satisfy this check. **Pulse monitoring and alerts are
   unavailable while stopped**; arrange independent outage coverage.
3. **Keep the pause until independent recovery checks pass.** Stopping the
   server does not cancel a guest-agent request already issued. Let existing
   operations finish normally; if their state is unknown, do not start the
   planned backup on the strength of zero replicas or an elapsed wait. After
   the backup ends, established checks independent of Pulse and the QEMU Guest
   Agent must confirm **thaw, fresh successful workload writes to every
   filesystem covered by the backup, and workload liveness**. An OK task, an
   absent lock, a console connection or a write to only the OS disk is not
   enough. If any check fails or is unavailable, keep the pause and use the
   guest/platform's recovery procedure, without a new backup or probe.
4. **Restore only the recorded prior state after all checks pass.** Restore
   each server's original desired count and only the reconciliation,
   autoscaling and updater controls that were active beforehand. A previously
   zero-replica server stays at zero; an unknown prior state is not permission
   to start it. Preserve the existing image, PVCs, configuration, identities
   and credentials. Confirm the intended server is ready and monitoring is
   fresh before relying on alerts again. Do not use an install command from
   this page as a restart or recovery shortcut.

If your cluster cannot establish these conditions, do not treat a deleted pod
or a successful scale request as permission to start the planned backup. This
guidance does not establish that a native freeze/thaw defect has been fixed.

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

### Prepare an agent Secret

Complete setup on the running Pulse server, then create a token in **API Access**
with `kubernetes:report`. Add `agent:report` only if you also enable host metrics.
Do not reuse an administrator token. Pulse v6 does not provision API tokens from
the legacy `API_TOKENS` server environment variable.

On your kubectl workstation, save only the token in a private, regular file
outside your GitOps repository. Do not paste it into a shell assignment, a
`--from-literal` argument, a Helm value or the DaemonSet YAML. Run these Bash
steps as the credential's owner, not with `sudo`. Stop if preparation or the
editor fails; do not continue to Secret creation. Existing token contents are
preserved until you deliberately edit them. Symlinked paths and non-regular
token files are refused before opening the editor:

```bash
(
  set -eu
  umask 077
  config_dir="$HOME/.config/pulse"
  token_file="$config_dir/kubernetes-agent-token"
  if [ -L "$HOME/.config" ] || [ -L "$config_dir" ] || [ -L "$token_file" ]; then
    printf 'Refusing a symlinked credential path.\n' >&2
    exit 1
  fi
  for directory in "$HOME/.config" "$config_dir"; do
    if [ -e "$directory" ] && { [ ! -d "$directory" ] || [ ! -O "$directory" ]; }; then
      printf 'Credential directories must belong to the current user.\n' >&2
      exit 1
    fi
  done
  if [ -e "$token_file" ] && { [ ! -f "$token_file" ] || [ ! -O "$token_file" ]; }; then
    printf 'Token file must be a regular file owned by the current user.\n' >&2
    exit 1
  fi
  mkdir -p "$config_dir"
  chmod 700 "$config_dir"
  touch "$token_file"
  chmod 600 "$token_file"
  vi "$token_file"
)
```

After saving the token in the editor, create the Secret in the same namespace
as the agent. This is a provisioning operation that can change cluster state,
not a diagnostic for a missing reading. Check your current kubectl context and
intended namespace privately before proceeding. The generated Secret goes
directly to kubectl, not to your terminal or a checked-in manifest. Recheck the
credential path before either pipeline; an empty, unreadable or unsafe file
stops before contacting the cluster. The subshell stops on either pipeline's
failure; do not continue to the agent deployment if it fails:

```bash
(
  set -euo pipefail
  umask 077
  config_dir="$HOME/.config/pulse"
  token_file="$config_dir/kubernetes-agent-token"
  if [ -L "$HOME/.config" ] || [ -L "$config_dir" ] || [ -L "$token_file" ]; then
    printf 'Refusing a symlinked credential path.\n' >&2
    exit 1
  fi
  for directory in "$HOME/.config" "$config_dir"; do
    if [ ! -d "$directory" ] || [ ! -O "$directory" ]; then
      printf 'Credential directories must belong to the current user.\n' >&2
      exit 1
    fi
  done
  if [ ! -f "$token_file" ] || [ ! -O "$token_file" ] || [ ! -r "$token_file" ] || [ ! -s "$token_file" ]; then
    printf 'Token file must be owned, readable, regular and non-empty.\n' >&2
    exit 1
  fi
  chmod 700 "$config_dir"
  chmod 600 "$token_file"
  kubectl create namespace pulse --dry-run=client -o yaml | \
    kubectl apply --server-side --field-manager=pulse-agent-setup -f -
  kubectl -n pulse create secret generic pulse-agent-env \
    --from-file=PULSE_TOKEN="$token_file" \
    --dry-run=client -o yaml | \
    kubectl apply --server-side --field-manager=pulse-agent-setup -f -
)
```

Server-side apply avoids copying the token into a last-applied annotation. If
another manager owns the Secret fields, reconcile that ownership instead of
forcing the conflict. Kubernetes Secrets are base64-encoded, **not inherently
encrypted**: restrict Secret access with RBAC and configure cluster encryption
at rest. Never share Secret YAML, Helm secret values or the local token file in
diagnostics. Keep the file private or remove it after provisioning; revoke a
token in Pulse when it is no longer needed. Secret rotation requires restarting
the agent pods because these examples read it into the environment at startup.

### OpenShift profile (Helm)

The chart has an SCC-compatible OpenShift profile for the Pulse server and an
optional cluster-level Kubernetes collector:

For a new server, deploy the server first and complete its browser setup before
creating the scoped agent token:

```bash
helm upgrade --install pulse pulse/pulse \
  --namespace pulse --create-namespace \
  --set openShift.enabled=true
```

Create `pulse-agent-env` using the private-file steps above, then enable the
collector:

```bash
helm upgrade --install pulse pulse/pulse \
  --namespace pulse \
  --set openShift.enabled=true \
  --set openShift.kubernetesAgent.enabled=true \
  --set openShift.kubernetesAgent.clusterID=my-openshift-cluster \
  --set agent.secretEnv.name=pulse-agent-env \
  --set 'agent.secretEnv.keys[0]=PULSE_TOKEN'
```

Using a pre-created Secret keeps the token out of Helm release values. For an
agent reporting to an existing external Pulse server, create the token there
and override `agent.env[0].value` with that server's reachable HTTPS `PULSE_URL`.
Keep certificate verification enabled.

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
- **Node and pod CPU/memory usage**: the Kubernetes module reads the cluster's
  `metrics.k8s.io` API. It does not require host mounts or privileged mode;
  the [read-only RBAC example](#rbac) includes the necessary metrics permissions.
- **Additional host-local telemetry**: a separate, deliberate host-agent
  deployment, with a distinct identity for each node. See
  [host metrics](#add-host-metrics-optional); do not enable it merely to repair
  missing API usage.

#### Minimal DaemonSet Example

This uses the main `rcourtman/pulse` image but runs the `pulse-agent` binary directly.
Create the `pulse-agent-env` Secret above and the RBAC resources below before
applying the DaemonSet. The YAML contains only the Secret reference, not the token.

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
              value: "http://pulse.pulse.svc.cluster.local:7655"
            - name: PULSE_TOKEN
              valueFrom:
                secretKeyRef:
                  name: pulse-agent-env
                  key: PULSE_TOKEN
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

Keep the cluster example **Kubernetes-only**. Its shared `PULSE_AGENT_ID` is
not an identity for every host: enabling host reporting under that same ID can
combine different nodes' host readings. Do not turn on `PULSE_ENABLE_HOST` or
add privileged mode and host-root mounts to obtain Kubernetes CPU/memory usage.
Use the metrics API and its read permissions instead.

Host-local disks, sensors and other host readings are a different collection
path. If you deliberately need them, plan a separate node-local deployment
through the [Unified Agent configuration](UNIFIED_AGENT.md#configuration),
with only the access those collectors need, persistent agent state and a
distinct, stable identity for each node. A Kubernetes-only `kubernetes:report`
token does not authorise host reporting; that path also needs `agent:report`.
It is not a reason to reuse an administrator token.

For an existing installation, preserve the current IDs, credentials and state;
do not rename, re-enrol or restart agents just to diagnose missing usage. Use
the [identity checks](UNIFIED_AGENT.md#duplicate-agents) before repairing a
confirmed host-identity collision. Read-only host mounts do not remove the
security exposure of a privileged pod, and neither changes Kubernetes API RBAC.

#### RBAC

The Kubernetes module uses the pod's service account to read the cluster API.
The minimal role below covers node, pod and Deployment inventory, their
CPU/memory usage from `metrics.k8s.io`, and optional recovery records. It is
not a complete role for every Kubernetes tab or resource type Pulse can collect.
Other inventory needs its own explicit read permissions; a running pod or one
populated tab does not establish complete collection.

For a new deployment, create the service account, role and binding in the
intended cluster before applying the agent workload. For an existing one,
reconcile the exact denied API group/resource with its managed role through
normal review, not by replacing a working binding or adding `cluster-admin`.
The service account and binding must use the agent pod's actual namespace.

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
  - apiGroups: ["metrics.k8s.io"]
    resources: ["nodes", "pods"]
    verbs: ["get", "list"]
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

These permissions do not install a metrics provider. The cluster must already
serve `metrics.k8s.io`; inventory can remain available when usage collection
is unavailable, and missing usage is not measured zero.

This role grants no Secret reads, `nodes/proxy`, writes or workload execution.
The collector's additional kubelet-summary path uses `nodes/proxy`, so network
and ephemeral-storage usage from that path can remain unavailable under this
minimal role even when API CPU/memory works. Do not broaden access or enable
host privileges merely to fill an empty field. Follow the
[failure-specific checks](#4-troubleshooting) using existing observations.

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

For a Proxmox backup, freeze/thaw or unresponsive-guest incident, the
[backup safety checks](#proxmox-backup-safety) take precedence over setup or
pod restarts. A Kubernetes pod's Ready state does not prove guest thaw.

Start with the affected pod's existing status and a bounded, locally reviewed
log excerpt around the original failure. Missing usage, missing inventory and
a rejected Pulse report are different failures:

| Observation | What to check without widening access |
| --- | --- |
| Pulse rejects reporting with HTTP 401/403 | Compare the intended Pulse server, token scopes and cluster identity with the existing configuration. `kubernetes:report` authorises Kubernetes reporting; host reporting also needs `agent:report`. Use the [agent authentication checks](UNIFIED_AGENT.md#agent-lookup-returns-http-401-or-403), not an administrator token or a new installation. |
| Kubernetes API reports Forbidden | Check the pod's service account and the exact denied API resource against its Role/ClusterRole. Running a pod as root or privileged does not grant Kubernetes API permissions. Do not add `cluster-admin`, Secret reads or `nodes/proxy` merely to diagnose a missing reading. |
| Inventory appears but CPU/memory usage is absent | Check whether the cluster's `metrics.k8s.io` service and its intended read permissions are available. Pulse can continue with inventory-only reporting when usage collection fails; missing usage is not zero. Host mounts and privileged mode are a separate, deliberate node-telemetry choice, not a repair for API metrics access. |
| Connection refused or TLS validation fails | Check the existing `PULSE_URL` and network path from the agent pod, not just your browser. A refusal is not an authentication error; retain certificate verification and do not replace a token or expose Pulse to bypass it. |

An applied Secret or running pod is not proof that Pulse accepted a fresh report
or collected every resource. Keep expected inventory and usage separate when
checking the ordinary view. Do not reapply the Secret, restart pods, enable
Debug, change cluster permissions or rerun provisioning just to complete a
report. A deliberate token rotation is separate maintenance and needs the
restart described above.

Keep tokens, Secret YAML, kubeconfigs, Helm values, pod environments and full
logs private. Use the original time, status, API resource and a manually
redacted error if help is needed; use consistent aliases for cluster, namespace
and pod names. Do not publish `kubectl get secret -o yaml`, full pod descriptions
or a broad label-selected log dump. Existing observations or unknowns are
valid evidence; see [safe evidence collection](TROUBLESHOOTING.md#collect-diagnostics-safely).
