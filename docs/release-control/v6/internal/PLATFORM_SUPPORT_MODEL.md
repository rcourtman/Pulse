# Pulse v6 Platform Support Model

Last updated: 2026-03-30
Status: ACTIVE

This file is the canonical governed model for platform support in Pulse v6.
It exists so new platforms are admitted against one shared contract instead of
through platform-by-platform improvisation.

## Canonical Rules

1. A first-class platform is a governed Pulse platform id with one declared
   primary ingestion mode, one owned onboarding path, explicit canonical
   resource projections, and one declared support matrix across the product
   surfaces below.
2. Platform families are grouping aids, not the support unit.
   `Proxmox` is a family; `proxmox-pve`, `proxmox-pbs`, and `proxmox-pmg` are
   separate first-class platforms because they onboard differently, project
   different canonical resources, and carry different support floors.
3. Runtime variants are not top-level platforms.
   `podman` is a runtime variant inside the first-class `docker` platform.
   `qemu`, `lxc`, OCI guest/runtime details, and TrueNAS app-runtime internals
   are workload technologies inside an owning platform, not new platforms.
4. Transport-specific implementations are not platforms.
   API clients, JSON-RPC sessions, pollers, agent heartbeat schemas, install
   flags, CRUD routes, and saved-connection tests are implementation details.
5. Optional augmentation paths are not platforms.
   A unified agent installed on an API-backed host may enrich or enable
   control for that platform, but it does not replace the platform's primary
   support contract.
6. `hybrid` is an ingestion mode, not a platform category.
   Use `hybrid` only when one first-class platform or resource contract
   intentionally merges API-backed and agent-backed truth.
7. Platform work must project into canonical shared resources first.
   Do not add provider-local top-level resource types by default.

## Platform Categories

### First-Class Platform

A first-class platform must define:

1. canonical platform id
2. platform family, if any
3. primary ingestion mode: `api-backed`, `agent-backed`, or `hybrid`
4. owned onboarding path
5. canonical unified-resource projections
6. support-floor row across the product surfaces below
7. assistant read classification and assistant control classification
8. optional augmentation rules, if a secondary path is allowed

### Runtime Variant

A runtime variant changes how a platform runs but does not change the owning
platform id, onboarding path, or top-level support contract.

Examples:

1. `podman` inside `docker`
2. `qemu` versus `lxc` inside `proxmox-pve`
3. container-runtime details inside TrueNAS-managed `app-container` resources

### Transport-Specific Implementation

A transport-specific implementation is the concrete mechanism used to ingest,
test, or execute a platform path.

Examples:

1. `pkg/pbs`, `pkg/pmg`, and TrueNAS JSON-RPC clients
2. Docker and Kubernetes agent report schemas
3. `--enable-docker`, `--enable-kubernetes`, and `--proxmox-type pve|pbs`
4. platform-connections CRUD and saved-connection test routes

### Optional Augmentation Path

An optional augmentation path is a secondary governed path that enriches an
existing first-class platform without replacing its primary ingestion mode.

Examples:

1. a unified agent on a Proxmox node enabling hybrid host telemetry or guest
   control for `proxmox-pve`
2. a unified agent on a TrueNAS appliance enriching a `truenas` system that is
   already supported through the API-backed poller

## Canonical Ingestion Modes

### API-backed

Pulse polls or queries the platform API directly. Optional agent data may
augment the platform later, but API truth defines the support floor.

Current API-backed primary platforms:

1. `proxmox-pve`
2. `proxmox-pbs`
3. `proxmox-pmg`
4. `truenas`

### Agent-backed

Pulse relies on a Pulse-managed agent as the primary source of truth.
Specialized runtime modules may ride on the same host install, but the agent
path defines the support floor.

Current agent-backed primary platforms:

1. `agent` for unified-agent hosts
2. `docker`
3. `kubernetes`

### Hybrid

Hybrid means one admitted platform deliberately merges API-backed and
agent-backed truth into one canonical resource contract. Hybrid is valid only
when the primary owner and the augmentation rule are both explicit.

Current governed hybrid-capable platforms:

1. `proxmox-pve`
2. `proxmox-pbs`
3. `truenas`

`docker` and `kubernetes` do not become hybrid platforms merely because they
run on a machine that also reports as `agent`; those are parallel first-class
platforms sharing one physical host.

## Canonical Resource Projection Rules

1. Host-like systems should project as canonical `agent` resources plus
   `platformType`, not as provider-local host types.
2. Current top-level exceptions are `pbs` and `pmg`, which remain dedicated
   canonical resource types because their product semantics are not reducible
   to a generic host row.
3. Proxmox guest workloads project as `vm` and `system-container`.
4. OCI and application workloads project as `app-container`, including
   TrueNAS-managed apps.
5. Docker Swarm service topology projects as `docker-service`.
6. Kubernetes projects as `k8s-cluster`, `k8s-node`, `pod`, and
   `k8s-deployment`.
7. Storage projects through shared `storage`, `ceph`, and `physical-disk`
   resources instead of provider-local storage types.
8. Recovery artifacts stay in `internal/recovery` and reference canonical
   platform ids plus canonical resource ids. Recovery provider strings are
   forward-compatible vocabulary, not support declarations by themselves.

## Support Floor

Every first-class platform must declare one matrix row covering these product
surfaces:

1. onboarding/setup
2. infrastructure visibility
3. workloads, if the platform projects workload resources
4. storage, if the platform projects storage or disk resources
5. recovery, if the platform emits protected-item or recovery-artifact truth
6. alerts, if the platform contributes operator-significant health state
7. assistant read
8. assistant control

A platform counts as supported only when every applicable surface above is
either `supported` or explicitly `n/a`. Blank, implied, or hand-wavy coverage
is not acceptable.

`assistant control` must be classified explicitly as one of:

1. `supported`
2. `augmentation-only`
3. `read-only`
4. `n/a`

## Current Classification

### First-class platforms

1. `agent` for unified-agent hosts
2. `docker`
3. `kubernetes`
4. `proxmox-pve`
5. `proxmox-pbs`
6. `proxmox-pmg`
7. `truenas`

### Runtime variants

1. `podman` is a runtime variant inside `docker`, surfaced through runtime
   metadata such as `containerRuntime`, not as a top-level platform.
2. `qemu`, `lxc`, and OCI guest/runtime details are workload technologies
   inside `proxmox-pve`, not first-class platforms.
3. TrueNAS app runtime internals are implementation details of
   `truenas`-owned `app-container` resources, not `docker` adoption.

### Transport-specific implementations

1. Proxmox, PBS, PMG, and TrueNAS connection CRUD and test routes
2. PBS/PMG API clients and the TrueNAS JSON-RPC poller stack
3. Docker and Kubernetes agent report contracts
4. install-command helpers and setup-script flags
5. platform badge and filter helpers

### Optional augmentation paths

1. unified agent on a Proxmox node
2. unified agent on a TrueNAS appliance
3. host-level agent support that enables shell or guest control for an already
   admitted API-backed platform

## Current Support Matrix

| Platform | Family | Primary mode | Optional augmentation | Canonical projections |
| --- | --- | --- | --- | --- |
| `agent` | Pulse-managed host | agent-backed | none | `agent`, `storage`, `physical-disk` |
| `docker` | container runtime | agent-backed | none | `agent`, `app-container`, `docker-service` |
| `kubernetes` | cluster runtime | agent-backed | none | `k8s-cluster`, `k8s-node`, `pod`, `k8s-deployment` |
| `proxmox-pve` | Proxmox | api-backed | host agent may augment into hybrid | `agent`, `vm`, `system-container`, `storage`, `ceph`, `physical-disk` |
| `proxmox-pbs` | Proxmox | api-backed | host agent may augment into hybrid | `pbs`, `storage` |
| `proxmox-pmg` | Proxmox | api-backed | none today | `pmg` |
| `truenas` | TrueNAS | api-backed | host agent may augment into hybrid | `agent`, `app-container`, `storage`, `physical-disk` |

| Platform | Setup | Visibility | Workloads | Storage | Recovery | Alerts | Assistant read | Assistant control |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `agent` | install workspace | supported | `n/a` | supported | `n/a` | supported | supported | supported |
| `docker` | install workspace / runtime enablement | supported | supported | `n/a` | `n/a` | supported | supported | supported |
| `kubernetes` | install workspace / runtime enablement | supported | supported | `n/a` | supported | supported | supported | supported |
| `proxmox-pve` | platform connections | supported | supported | supported | supported | supported | supported | augmentation-only |
| `proxmox-pbs` | platform connections | supported | `n/a` | supported | supported | supported | supported | read-only |
| `proxmox-pmg` | platform connections | supported | `n/a` | `n/a` | `n/a` | supported | supported | read-only |
| `truenas` | platform connections | supported | supported | supported | supported | supported | supported | supported |

## Current Inconsistencies To Treat Explicitly

1. `frontend-modern/src/utils/sourcePlatforms.ts` already carries future labels
   such as `vmware-vsphere`, `microsoft-hyperv`, `aws`, `azure`, `gcp`,
   `unraid`, and `synology-dsm`. Those are presentation vocabulary only. They
   are not admitted first-class platforms until governance says so here.
2. Recovery provider strings are intentionally forward-compatible and already
   include values such as `docker`, `agent`, and `proxmox-pmg`. Those strings
   do not mean recovery support exists until the platform matrix above marks
   recovery as supported.
3. Frontend compatibility types still expose some legacy or presentation-local
   resource aliases. The canonical backend truth remains the shared
   unified-resource model plus this platform matrix.

## Future Platform Admission

A new platform may be admitted as first-class only after governance answers
all of these questions before implementation starts:

1. What is the canonical platform id, and what platform family does it belong
   to, if any?
2. Is the primary ingestion mode `api-backed`, `agent-backed`, or `hybrid`?
   If hybrid, what is primary and what is only augmentation?
3. What is the canonical onboarding path: platform connections, install
   workspace, or both?
4. What existing canonical resource types does it project into?
   New resource types require explicit justification; provider-local host types
   are forbidden by default.
5. Which support-floor surfaces are `supported`, `augmentation-only`,
   `read-only`, or `n/a`?
6. What stable identities drive dedupe, monitored-system counting, and
   cross-source merge with `agent`, `docker`, `kubernetes`, or other existing
   platforms?
7. What transport/security boundary owns credentials, polling cadence,
   reconnect semantics, and disabled/default behavior?
8. What proof demonstrates the declared floor is real in onboarding,
   visibility, applicable domain surfaces, alerts, and assistant behavior?

If those answers are not yet stable, the work must stop at a governed
open-decision or resolved-decision update instead of starting runtime code.

## Likely vSphere Path

If VMware vSphere is added in v6 or later, the default evaluation path is:

1. treat it as a separate first-class platform id such as `vmware-vsphere`,
   not as another Proxmox subtype and not as a generic future-label shortcut
2. assume API-backed primary ingestion unless governance explicitly chooses an
   agent-first or hybrid contract
3. reuse canonical `agent` for host-like top-level systems when that model is
   appropriate, `vm` for guests, and `storage` for datastores; do not invent
   `esxi-host` or `vsphere-vm` types by default
4. require explicit answers for host-versus-cluster top-level identity,
   vCenter-versus-direct-ESXi support, storage/datastore scope, recovery
   truth, alerts floor, assistant read, assistant control, and any agent
   augmentation rule before implementation starts

Adding `vmware-vsphere` to a label helper, filter list, or presentation badge
does not admit the platform. Admission happens only when this model, the
owning contracts, and the required proof surfaces are updated together.
