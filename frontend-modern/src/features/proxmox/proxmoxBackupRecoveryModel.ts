import type { BackupTask, GuestSnapshot, PBSBackup, StorageBackup } from '@/types/api';
import type { ProtectionPosture, ProtectionState } from '@/types/recovery';
import type { Resource } from '@/types/resource';

import {
  getProxmoxArchiveSourceTitle,
  getProxmoxBackupSourcePresentation,
  type ProxmoxBackupSourceKind,
} from './proxmoxBackupSourcePresentation';

export type RecoverableSourceKind = ProxmoxBackupSourceKind;

export type WorkloadRecoveryPosture = ProtectionState;

export interface WorkloadReference {
  key: string;
  resourceId?: string;
  type: 'vm' | 'ct' | 'host' | 'unknown';
  typeLabel: string;
  vmid: string;
  label: string;
  name?: string;
  node?: string;
  instance?: string;
}

export interface RecoverableArtifact {
  id: string;
  nativeId: string;
  sourceKind: RecoverableSourceKind;
  sourceLabel: string;
  sourceTitle?: string;
  workload: WorkloadReference;
  createdAt: string;
  createdMs?: number;
  size?: number;
  location: string;
  detail: string;
  protected: boolean;
  verified?: boolean;
  fileCount?: number;
}

export interface CoverageTask {
  id: string;
  status: string;
  label: string;
  startedAt: string;
  startedMs?: number;
  durationSeconds?: number;
  error?: string;
}

export interface WorkloadCoverageRow {
  key: string;
  workload: WorkloadReference;
  artifacts: RecoverableArtifact[];
  latestRecovery?: RecoverableArtifact;
  latestPBS?: RecoverableArtifact;
  latestArchive?: RecoverableArtifact;
  latestSnapshot?: RecoverableArtifact;
  latestTask?: CoverageTask;
  pbsCount: number;
  archiveCount: number;
  snapshotCount: number;
  posture: WorkloadRecoveryPosture;
  postureRank: number;
  protectionPosture?: ProtectionPosture;
  // True only when a VM/LXC row exists because a backup/task referenced a VMID
  // with no matching live inventory guest. Host backups can also carry a
  // `backup:` key, but they are first-class backup targets, not orphaned guests.
  isOrphaned: boolean;
}

export interface ProxmoxBackupRecoveryModel {
  coverageRows: WorkloadCoverageRow[];
  recoverableArtifacts: RecoverableArtifact[];
  coverageSummary: {
    totalWorkloads: number;
    protected: number;
    attention: number;
    unprotected: number;
    unknown: number;
    withPBS: number;
    recoverableArtifacts: number;
    totalBytes: number;
  };
}

interface BuildModelInput {
  workloads: readonly Resource[];
  pbsBackups: readonly PBSBackup[];
  archives: readonly StorageBackup[];
  snapshots: readonly GuestSnapshot[];
  tasks: readonly BackupTask[];
  nowMs: number;
  protectionPostures?: ReadonlyMap<string, ProtectionPosture>;
}

interface WorkloadCandidate extends WorkloadReference {
  nodeKey?: string;
  instanceKey?: string;
}

type WorkloadRowDraft = Omit<WorkloadCoverageRow, 'posture' | 'postureRank' | 'isOrphaned'>;

const DAY_MS = 24 * 60 * 60 * 1000;
export const CURRENT_RECOVERY_MS = 7 * DAY_MS;
export const STALE_RECOVERY_MS = 30 * DAY_MS;

export type RecoveryAgeBand = 'current' | 'aging' | 'stale' | 'unknown';

export function getRecoveryAgeBand(
  createdMs: number | undefined,
  nowMs: number = Date.now(),
): RecoveryAgeBand {
  if (createdMs === undefined || !Number.isFinite(createdMs)) return 'unknown';
  const ageMs = nowMs - createdMs;
  if (!Number.isFinite(ageMs)) return 'unknown';
  if (ageMs <= CURRENT_RECOVERY_MS) return 'current';
  if (ageMs <= STALE_RECOVERY_MS) return 'aging';
  return 'stale';
}

function parseTimestampMs(value: string | undefined): number | undefined {
  if (!value) return undefined;
  const ms = Date.parse(value);
  return Number.isFinite(ms) ? ms : undefined;
}

function pbsBackupFileDetailLabel(fileCount: number): string {
  if (fileCount <= 0) return 'PBS files not listed';
  if (fileCount === 1) return '1 PBS file';
  return `${fileCount} PBS files`;
}

function normalizeKey(value: string | number | undefined | null): string {
  return String(value ?? '')
    .trim()
    .toLowerCase();
}

function isZeroWorkloadId(vmid: string): boolean {
  return normalizeKey(vmid) === '0';
}

function backupTypeLabel(type: string | undefined): WorkloadReference['type'] {
  const normalized = normalizeKey(type);
  if (normalized === 'vm' || normalized === 'qemu') return 'vm';
  if (normalized === 'ct' || normalized === 'lxc') return 'ct';
  if (normalized === 'host') return 'host';
  return 'unknown';
}

function typeLabel(type: WorkloadReference['type']): string {
  if (type === 'vm') return 'VM';
  if (type === 'ct') return 'LXC';
  if (type === 'host') return 'Host';
  return 'Guest';
}

function firstHostHint(hints: readonly (string | undefined)[]): string | undefined {
  return hints
    .map((hint) => hint?.trim())
    .find((hint) => hint && normalizeKey(hint) !== 'root' && normalizeKey(hint) !== '(root)');
}

function fallbackWorkloadId(
  type: WorkloadReference['type'],
  vmid: string,
  hints: readonly (string | undefined)[],
): string {
  const cleanVmid = vmid.trim();
  if (type === 'host') {
    if (cleanVmid && !isZeroWorkloadId(cleanVmid)) return cleanVmid;
    return firstHostHint(hints) ?? '';
  }
  if (cleanVmid && !isZeroWorkloadId(cleanVmid)) return cleanVmid;
  return '';
}

function workloadFallbackLabel(type: WorkloadReference['type'], id: string): string {
  const label = typeLabel(type);
  const cleanId = id.trim();
  if (cleanId) return `${label} ${cleanId}`;
  if (type === 'host') return 'Host backup';
  return type === 'unknown' ? label : `${label} backup`;
}

function resourceVmid(resource: Resource): string {
  const fromMeta = resource.proxmox?.vmid;
  if (typeof fromMeta === 'number' && Number.isFinite(fromMeta)) return String(fromMeta);
  const platformProxmox = resource.platformData?.proxmox;
  if (typeof platformProxmox === 'object' && platformProxmox !== null) {
    const value = (platformProxmox as Record<string, unknown>).vmid;
    if (typeof value === 'number' && Number.isFinite(value)) return String(value);
    if (typeof value === 'string' && value.trim()) return value.trim();
  }
  return '';
}

function resourceBackupType(resource: Resource): WorkloadReference['type'] {
  if (resource.type === 'vm') return 'vm';
  if (resource.type === 'system-container' || resource.type === 'oci-container') return 'ct';
  return 'unknown';
}

function resourceNode(resource: Resource): string | undefined {
  return resource.proxmox?.nodeName || resource.proxmox?.node || resource.parentName || undefined;
}

function resourceInstance(resource: Resource): string | undefined {
  const fromMeta = resource.proxmox?.instance;
  if (fromMeta) return fromMeta;
  const platformProxmox = resource.platformData?.proxmox;
  if (typeof platformProxmox === 'object' && platformProxmox !== null) {
    const value = (platformProxmox as Record<string, unknown>).instance;
    if (typeof value === 'string' && value.trim()) return value.trim();
  }
  return undefined;
}

function buildWorkloadLabel(
  name: string | undefined,
  type: WorkloadReference['type'],
  vmid: string,
) {
  const fallback = workloadFallbackLabel(type, vmid);
  const cleanName = name?.trim();
  if (!cleanName || cleanName === fallback || cleanName === vmid) return fallback;
  return `${cleanName} (${fallback})`;
}

function buildCandidateFromResource(resource: Resource): WorkloadCandidate | null {
  const vmid = resourceVmid(resource);
  const type = resourceBackupType(resource);
  if (!vmid || isZeroWorkloadId(vmid) || type === 'unknown') return null;
  const name = resource.displayName || resource.name || undefined;
  const node = resourceNode(resource);
  const instance = resourceInstance(resource);
  return {
    key: `resource:${resource.id}`,
    resourceId: resource.id,
    type,
    typeLabel: typeLabel(type),
    vmid,
    label: buildWorkloadLabel(name, type, vmid),
    name,
    node,
    instance,
    nodeKey: normalizeKey(node),
    instanceKey: normalizeKey(instance),
  };
}

function fallbackWorkload(
  type: WorkloadReference['type'],
  vmid: string,
  hints: readonly (string | undefined)[],
): WorkloadReference {
  const id = fallbackWorkloadId(type, vmid, hints);
  const scope = hints.map(normalizeKey).find(Boolean) || 'unknown';
  const keyScope = type === 'host' && id ? 'host' : scope;
  return {
    key: `backup:${type}:${id || 'unknown'}:${keyScope}`,
    type,
    typeLabel: typeLabel(type),
    vmid: id,
    label: workloadFallbackLabel(type, id),
    node: type === 'host' ? undefined : hints.find((hint) => !!hint?.trim()),
  };
}

function isCoverageWorkload(workload: WorkloadReference): boolean {
  if (workload.type === 'vm' || workload.type === 'ct') {
    return Boolean(workload.vmid.trim()) && !isZeroWorkloadId(workload.vmid);
  }
  return workload.type === 'host' && Boolean(workload.vmid.trim());
}

function resolveWorkload(
  candidates: readonly WorkloadCandidate[],
  type: WorkloadReference['type'],
  vmid: string,
  hints: readonly (string | undefined)[],
): WorkloadReference {
  const typed = candidates.filter(
    (candidate) => candidate.type === type && candidate.vmid === vmid,
  );
  const normalizedHints = hints.map(normalizeKey).filter(Boolean);
  if (typed.length > 0 && normalizedHints.length > 0) {
    const exact = typed.find((candidate) =>
      normalizedHints.some(
        (hint) =>
          hint === candidate.nodeKey ||
          hint === candidate.instanceKey ||
          normalizeKey(candidate.node).includes(hint) ||
          normalizeKey(candidate.instance).includes(hint),
      ),
    );
    if (exact) return exact;
  }
  if (typed.length === 1) return typed[0];
  return fallbackWorkload(type, vmid, hints);
}

function matchWorkloadByHints<T extends WorkloadReference>(
  workloads: readonly T[],
  hints: readonly (string | undefined)[],
): T | undefined {
  const normalizedHints = hints.map(normalizeKey).filter(Boolean);
  if (workloads.length > 0 && normalizedHints.length > 0) {
    const exact = workloads.find((workload) =>
      normalizedHints.some(
        (hint) =>
          hint === normalizeKey(workload.node) ||
          hint === normalizeKey(workload.instance) ||
          normalizeKey(workload.node).includes(hint) ||
          normalizeKey(workload.instance).includes(hint),
      ),
    );
    if (exact) return exact;
  }
  if (workloads.length === 1) return workloads[0];
  return undefined;
}

function resolveTaskWorkload(
  candidates: readonly WorkloadCandidate[],
  rows: ReadonlyMap<string, WorkloadRowDraft>,
  task: BackupTask,
): WorkloadReference | undefined {
  const vmid = String(task.vmid);
  if (!vmid || isZeroWorkloadId(vmid)) return undefined;

  const explicitType = backupTypeLabel(task.type);
  const hints = [task.instance, task.node];
  if (explicitType !== 'unknown') return resolveWorkload(candidates, explicitType, vmid, hints);

  const candidate = matchWorkloadByHints(
    candidates.filter((item) => item.vmid === vmid),
    hints,
  );
  if (candidate) return candidate;

  return matchWorkloadByHints(
    Array.from(rows.values())
      .map((row) => row.workload)
      .filter((workload) => workload.vmid === vmid && workload.type !== 'unknown'),
    hints,
  );
}

function newest<T>(items: readonly T[], getMs: (item: T) => number | undefined): T | undefined {
  let best: T | undefined;
  let bestMs = Number.NEGATIVE_INFINITY;
  for (const item of items) {
    const ms = getMs(item);
    if (ms !== undefined && ms > bestMs) {
      best = item;
      bestMs = ms;
    }
  }
  return best;
}

function taskLabel(task: BackupTask): string {
  const normalized = normalizeKey(task.status);
  if (normalized === 'ok' || normalized === 'success' || normalized === 'completed') return 'OK';
  if (normalized === 'failed' || normalized === 'error') return 'Failed';
  if (normalized === 'running') return 'Running';
  return task.status || 'Unknown';
}

function taskDurationSeconds(task: BackupTask): number | undefined {
  const start = parseTimestampMs(task.startTime);
  const end = parseTimestampMs(task.endTime);
  if (start === undefined || end === undefined || end <= start) return undefined;
  return Math.round((end - start) / 1000);
}

function protectionPostureRank(posture: WorkloadRecoveryPosture): number {
  if (posture === 'attention') return 0;
  if (posture === 'unprotected') return 1;
  if (posture === 'unknown') return 2;
  return 3;
}

export function getWorkloadRecoveryPostureLabel(posture: WorkloadRecoveryPosture): string {
  switch (posture) {
    case 'protected':
      return 'Protected';
    case 'attention':
      return 'Needs attention';
    case 'unprotected':
      return 'Unprotected';
    case 'unknown':
      return 'Unknown';
  }
}

export function isCoverageAttention(posture: WorkloadRecoveryPosture): boolean {
  return posture === 'attention';
}

export function buildProxmoxBackupRecoveryModel(
  input: BuildModelInput,
): ProxmoxBackupRecoveryModel {
  const candidates = input.workloads
    .map(buildCandidateFromResource)
    .filter((candidate): candidate is WorkloadCandidate => candidate !== null);
  const rows = new Map<string, WorkloadRowDraft>();

  const ensureRow = (workload: WorkloadReference) => {
    const existing = rows.get(workload.key);
    if (existing) return existing;
    const row: WorkloadRowDraft = {
      key: workload.key,
      workload,
      artifacts: [],
      pbsCount: 0,
      archiveCount: 0,
      snapshotCount: 0,
    };
    rows.set(workload.key, row);
    return row;
  };

  for (const candidate of candidates) ensureRow(candidate);

  const artifacts: RecoverableArtifact[] = [];
  const pbsSource = getProxmoxBackupSourcePresentation('pbs');
  const archiveSource = getProxmoxBackupSourcePresentation('archive');
  const snapshotSource = getProxmoxBackupSourcePresentation('snapshot');
  const addArtifact = (artifact: RecoverableArtifact) => {
    artifacts.push(artifact);
    if (!isCoverageWorkload(artifact.workload)) return;
    const row = ensureRow(artifact.workload);
    row.artifacts.push(artifact);
    if (artifact.sourceKind === 'pbs') row.pbsCount += 1;
    else if (artifact.sourceKind === 'archive') row.archiveCount += 1;
    else row.snapshotCount += 1;
  };

  for (const backup of input.pbsBackups) {
    const type = backupTypeLabel(backup.backupType);
    const workload = resolveWorkload(candidates, type, backup.vmid, [
      backup.namespace,
      backup.datastore,
    ]);
    const createdMs = parseTimestampMs(backup.backupTime);
    addArtifact({
      id: `pbs:${backup.id}`,
      nativeId: backup.id,
      sourceKind: 'pbs',
      sourceLabel: pbsSource.badgeLabel,
      sourceTitle: pbsSource.sourceTitle,
      workload,
      createdAt: backup.backupTime,
      createdMs,
      size: backup.size,
      location: `${backup.datastore || '—'} / ${backup.namespace?.trim() || '(root)'}`,
      detail: pbsBackupFileDetailLabel(backup.files.length),
      protected: backup.protected,
      verified: backup.verified,
      fileCount: backup.files.length,
    });
  }

  for (const archive of input.archives) {
    const type = backupTypeLabel(archive.type);
    const workload = resolveWorkload(candidates, type, String(archive.vmid), [
      archive.instance,
      archive.node,
    ]);
    const createdMs = parseTimestampMs(archive.time);
    addArtifact({
      id: `archive:${archive.id}`,
      nativeId: archive.id,
      sourceKind: 'archive',
      sourceLabel: archiveSource.badgeLabel,
      sourceTitle: getProxmoxArchiveSourceTitle(Boolean(archive.isPBS)),
      workload,
      createdAt: archive.time,
      createdMs,
      size: archive.size,
      location: archive.storage || archive.node || '—',
      detail: archive.volid || archive.format || archiveSource.detailFallbackLabel,
      protected: archive.protected,
      verified: archive.isPBS ? archive.verified : undefined,
    });
  }

  for (const snapshot of input.snapshots) {
    const type = backupTypeLabel(snapshot.type);
    const workload = resolveWorkload(candidates, type, String(snapshot.vmid), [
      snapshot.instance,
      snapshot.node,
    ]);
    const createdMs = parseTimestampMs(snapshot.time);
    addArtifact({
      id: `snapshot:${snapshot.id}`,
      nativeId: snapshot.id,
      sourceKind: 'snapshot',
      sourceLabel: snapshotSource.badgeLabel,
      sourceTitle: snapshotSource.sourceTitle,
      workload,
      createdAt: snapshot.time,
      createdMs,
      size: snapshot.sizeBytes,
      location: snapshot.node || snapshot.instance || '—',
      detail: snapshot.description || snapshot.name || snapshotSource.detailFallbackLabel,
      protected: false,
    });
  }

  for (const task of input.tasks) {
    const workload = resolveTaskWorkload(candidates, rows, task);
    if (!workload) continue;
    const row = ensureRow(workload);
    const candidateTask: CoverageTask = {
      id: task.id,
      status: task.status,
      label: taskLabel(task),
      startedAt: task.startTime,
      startedMs: parseTimestampMs(task.startTime),
      durationSeconds: taskDurationSeconds(task),
      error: task.error,
    };
    if (!row.latestTask || (candidateTask.startedMs ?? 0) > (row.latestTask.startedMs ?? 0)) {
      row.latestTask = candidateTask;
    }
  }

  for (const row of rows.values()) {
    row.latestPBS = newest(
      row.artifacts.filter((artifact) => artifact.sourceKind === 'pbs'),
      (artifact) => artifact.createdMs,
    );
    row.latestArchive = newest(
      row.artifacts.filter((artifact) => artifact.sourceKind === 'archive'),
      (artifact) => artifact.createdMs,
    );
    row.latestSnapshot = newest(
      row.artifacts.filter((artifact) => artifact.sourceKind === 'snapshot'),
      (artifact) => artifact.createdMs,
    );
    row.latestRecovery = newest(row.artifacts, (artifact) => artifact.createdMs);
  }

  const coverageRows = Array.from(rows.values()).map((row) => {
    const protectionPosture = row.workload.resourceId
      ? input.protectionPostures?.get(row.workload.resourceId)
      : undefined;
    const posture = protectionPosture?.state ?? 'unknown';
    return {
      ...row,
      posture,
      postureRank: protectionPostureRank(posture),
      protectionPosture,
      isOrphaned:
        !row.key.startsWith('resource:') &&
        (row.workload.type === 'vm' || row.workload.type === 'ct'),
    };
  });

  coverageRows.sort((left, right) => {
    if (left.postureRank !== right.postureRank) return left.postureRank - right.postureRank;
    return (right.latestRecovery?.createdMs ?? 0) - (left.latestRecovery?.createdMs ?? 0);
  });
  artifacts.sort((left, right) => (right.createdMs ?? 0) - (left.createdMs ?? 0));

  const totalBytes = artifacts.reduce((sum, artifact) => sum + (artifact.size ?? 0), 0);
  return {
    coverageRows,
    recoverableArtifacts: artifacts,
    coverageSummary: {
      totalWorkloads: coverageRows.length,
      protected: coverageRows.filter((row) => row.posture === 'protected').length,
      attention: coverageRows.filter((row) => isCoverageAttention(row.posture)).length,
      unprotected: coverageRows.filter((row) => row.posture === 'unprotected').length,
      unknown: coverageRows.filter((row) => row.posture === 'unknown').length,
      withPBS: coverageRows.filter((row) => row.pbsCount > 0).length,
      recoverableArtifacts: artifacts.length,
      totalBytes,
    },
  };
}

export function coverageRowMatchesSearch(row: WorkloadCoverageRow, term: string): boolean {
  const normalizedTerm = term.trim().toLowerCase();
  if (!normalizedTerm) return true;
  const haystack = [
    row.workload.label,
    row.workload.name,
    row.workload.typeLabel,
    row.workload.vmid,
    row.workload.node,
    row.workload.instance,
    getWorkloadRecoveryPostureLabel(row.posture),
    row.latestTask?.label,
    row.latestTask?.error,
    ...row.artifacts.flatMap((artifact) => [
      artifact.sourceLabel,
      artifact.sourceTitle,
      artifact.location,
      artifact.detail,
    ]),
  ];
  return haystack.filter(Boolean).join(' ').toLowerCase().includes(normalizedTerm);
}

export function recoverableArtifactMatchesSearch(
  artifact: RecoverableArtifact,
  term: string,
): boolean {
  const normalizedTerm = term.trim().toLowerCase();
  if (!normalizedTerm) return true;
  const haystack = [
    artifact.workload.label,
    artifact.workload.name,
    artifact.workload.typeLabel,
    artifact.workload.vmid,
    artifact.workload.node,
    artifact.workload.instance,
    artifact.sourceLabel,
    artifact.sourceTitle,
    artifact.location,
    artifact.detail,
    artifact.verified === true
      ? 'verified'
      : artifact.verified === false
        ? 'unverified'
        : undefined,
    artifact.protected ? 'protected' : 'unprotected',
  ];
  return haystack.filter(Boolean).join(' ').toLowerCase().includes(normalizedTerm);
}
