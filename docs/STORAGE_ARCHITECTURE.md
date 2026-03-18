# Storage Architecture Proposal

This document defines the intended storage model for Pulse beyond the current "show storage resources and raw S.M.A.R.T. fields" behavior.

The goal is to make storage genuinely useful for operators, not merely visible.

## Problem

Today Pulse can surface storage-adjacent data from several sources:

- Proxmox storage pools
- Proxmox physical disks
- Ceph
- host-agent disk inventories
- host-agent S.M.A.R.T. data
- TrueNAS pools/datasets/disks

That is useful, but it is not yet a coherent storage product.

The current gaps are:

- disk data is source-shaped instead of operator-shaped
- S.M.A.R.T. attributes are visible, but risk is not modeled
- topology is weak: disk -> pool/array/host/workload impact is incomplete
- agent-only hosts need first-class storage treatment, not second-class fallback behavior
- storage alerting is mostly threshold-oriented rather than consequence-oriented

## Product Principle

Operators do not want "S.M.A.R.T. monitoring."

They want answers to:

- Which disks are at risk?
- Which pools/arrays are at risk because of those disks?
- Is redundancy still intact?
- Is this getting worse?
- What needs action now?

Pulse should therefore treat S.M.A.R.T. as one input signal inside a broader storage health model.

## Primary User Jobs

### Homelab / power users

- Identify failing disks before data loss
- See parity/cache/array issues clearly
- Map a bad disk to a specific device/serial/path
- Understand whether replacement is urgent or watch-only

### SMB / business operators

- See storage risk by host, cluster, site, and business impact
- Know whether backup targets and primary storage remain healthy
- Detect degraded redundancy, not just degraded disks
- Track long-term degradation trends and maintenance windows

## Canonical Storage Model

Pulse should model storage in four layers.

### 1. Physical Disk

This is the actual block device.

Canonical resource type:

- `physical_disk`

Identity signals, strongest first:

- serial
- WWN / EUI
- controller-specific stable disk ID
- source-scoped fallback `(host, device path)`

Core fields:

- serial, WWN, device path
- model, vendor, firmware
- transport / type (`sata`, `sas`, `nvme`, `usb`, etc.)
- size
- health / risk / confidence
- temperature
- wear indicators
- media / pending / reallocated / CRC / unsafe-shutdown style counters
- telemetry freshness

### 2. Storage Membership

This is the topology layer.

A disk is often only meaningful in context:

- member of mdraid array
- member of ZFS vdev/pool
- Unraid parity/data/cache assignment
- Ceph OSD backing device
- PBS datastore backing disk set

Pulse should model storage membership as first-class relationships, not implicit text fields.

Examples:

- disk -> host
- disk -> array
- disk -> pool
- disk -> OSD
- pool -> workloads
- datastore -> backup jobs / recovery points

### 3. Logical Storage Object

These are the operator-facing objects:

- pool
- datastore
- filesystem
- dataset
- share
- Ceph cluster / pool
- backup repository

Canonical resource types already mostly exist:

- `storage`
- `datastore`
- `ceph`

These resources should carry:

- capacity
- health
- redundancy state
- rebuild/resilver/scrub state
- impacted children

### 4. Consumer Impact

This is the "why should I care" layer.

Storage objects should be traceable to:

- VMs
- LXCs
- app containers / pods
- backup jobs
- recovery points

This allows Pulse to answer:

- a degraded mirror affects these VMs
- this backup datastore is filling and will affect these protection jobs
- this failed disk left this array with no redundancy

## S.M.A.R.T. Model

### Raw telemetry

Pulse should ingest raw S.M.A.R.T. data when available, including vendor-specific subsets.

Raw attributes remain important in the detail view, but they should not be the primary UX.

### Derived model

Pulse should derive a normalized disk health model from raw telemetry:

- `health_state`
  - healthy
  - watch
  - degraded
  - critical
  - unknown
- `risk_score`
  - 0-100
- `confidence`
  - low / medium / high
- `reason_codes`
  - `pending_sectors_nonzero`
  - `reallocated_sectors_rising`
  - `nvme_spare_low`
  - `temperature_sustained_high`
  - `smart_failed`
  - `telemetry_missing`

### Trend model

Current values are not enough.

Pulse should preserve time series for:

- temperature
- reallocated sectors
- pending sectors
- media errors
- NVMe percentage used
- available spare
- unsafe shutdowns

Trend direction matters:

- stable
- improving
- slowly worsening
- sharply worsening

## Source Strategy

### Proxmox

Use Proxmox for:

- storage pools
- physical disks when available
- Ceph
- host/node topology

Use agent linkage to enrich Proxmox disks with:

- better temperature coverage
- richer S.M.A.R.T. attributes
- better device identity

### Unified host agent

The host agent must be a first-class storage source, not only an enrichment source.

For agent-backed hosts, Pulse should directly create:

- `physical_disk` resources from agent S.M.A.R.T.
- logical storage resources when the agent can report them
- storage topology when the platform supports it

This matters for:

- Unraid
- generic Linux servers
- bare-metal NAS boxes
- non-Proxmox storage hosts

### Unraid

Unraid deserves explicit treatment, not generic-Linux treatment forever.

Pulse should ultimately understand:

- array state
- parity devices
- cache pools
- disk disabled / missing / emulated state
- rebuild progress
- filesystem status
- share impact

Initial fallback can still be generic host-agent disk ingestion, but the end state should be Unraid-aware topology.

### ZFS / TrueNAS

Pulse should normalize:

- pool health
- vdev health
- read/write/checksum errors
- scrub status and age
- resilver status and age
- per-disk membership

### Generic Linux

Even without a rich platform API, Pulse should still provide value:

- agent physical disks
- mdraid state if available
- mount/device correlation
- filesystem usage
- telemetry coverage warnings

## Alerts

Storage alerts should be layered.

### Disk alerts

Examples:

- S.M.A.R.T. failed
- pending sectors non-zero
- reallocated sectors rising
- NVMe spare below threshold
- sustained high temperature

### Redundancy alerts

Examples:

- pool degraded but still redundant
- array has lost redundancy
- parity invalid / parity missing
- OSD count below safe threshold

### Capacity alerts

Examples:

- pool nearing full
- backup datastore nearing full
- cache pool under pressure

### Telemetry coverage alerts

Examples:

- disk telemetry missing for previously known disk
- controller blocks S.M.A.R.T. visibility
- host stopped reporting disk inventory

This category is important because silent storage blind spots are dangerous.

## UX Proposal

The storage surface should be organized around three questions.

### 1. What is at risk?

Top-level storage page should prioritize:

- disks needing attention
- degraded pools/arrays
- rebuilds/resilvers in progress
- backup repositories at risk

### 2. Where is the risk?

Every disk or pool should show context:

- host
- platform
- array / pool / vdev / parity role
- impacted workloads / backups

### 3. What should I do?

Each finding should have a recommended action:

- replace now
- schedule maintenance
- monitor trend
- investigate controller / cable / cooling
- improve telemetry coverage

## Recommended Page Structure

### Fleet summary

- disks at risk
- degraded storage objects
- active rebuild/resilver operations
- storage capacity hotspots

### Disk view

Grouped and filterable by:

- host
- pool / array
- risk state
- platform
- disk type

Columns:

- device / serial
- host
- role
- health
- risk
- temperature
- wear
- trend
- last seen

### Topology view

For a selected disk:

- parent host
- array / pool / vdev membership
- redundancy state
- affected storage objects
- affected workloads / backups

### Detail drawer

Include:

- normalized summary
- risk reasons
- trend charts
- raw S.M.A.R.T. attributes
- source provenance
- telemetry freshness

## Data Model Requirements

The canonical unified resource model should support:

- `physical_disk` from every valid source
- disk identity merge across sources
- parent/child relationships between host, disk, pool, workload
- source provenance per disk field when signals disagree
- storage topology edges, not just flat metadata blobs
- freshness per source and per sub-signal

## Rollout Plan

### Phase 1: Canonical disk coverage

- ensure every agent-backed host can emit `physical_disk`
- unify disk identity across agent / Proxmox / TrueNAS sources
- show agent-only disks in storage
- attach disk metrics targets consistently

### Phase 2: Disk health model

- add derived S.M.A.R.T. health / risk / confidence
- add reason codes
- add telemetry freshness semantics
- improve disk alerts

### Phase 3: Topology

- model disk -> pool/array/vdev membership
- model redundancy state
- propagate impact to workloads / backups

### Phase 4: Platform specialization

- Unraid-aware storage model
- deeper ZFS / TrueNAS topology
- mdraid normalization
- controller-specific enrichments where feasible

### Phase 5: Operator UX

- risk-first storage landing page
- action-oriented recommendations
- maintenance-friendly detail workflows

## Near-Term Priority

If I were sequencing this immediately, I would prioritize:

1. agent-only physical disk coverage
2. canonical disk identity merge by serial / WWN
3. disk metrics and S.M.A.R.T. trend persistence for agent-backed disks
4. derived disk risk model
5. topology edges for arrays/pools/parity

That gives Pulse a strong storage foundation before investing in more UI complexity.

## Definition of "Useful"

Pulse storage is useful when an operator can answer, in under a minute:

- what is unhealthy
- what is merely noisy
- what is losing redundancy
- what will impact workloads or backups
- what needs action now

If the user still has to mentally decode raw S.M.A.R.T. tables to get there, the storage model is not finished.
