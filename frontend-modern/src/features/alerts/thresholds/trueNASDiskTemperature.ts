import { formatMetricValue } from './helpers';
import { FACTORY_AGENT_DEFAULTS } from '@/utils/alertThresholdDefaults';

// The thresholds editor's view of the TrueNAS disk temperature trigger. It
// mirrors the backend (trueNASDiskTemperatureDefaultNoLock): a TrueNAS-wide
// value the user saved applies to every TrueNAS disk; unset, each disk follows
// Disk temperature by type, which the agent Disk Temp default switches off for
// every type when it is off. It reads the unsaved editor state, so a row shows
// what saving would apply.

export const DISK_TEMPERATURE_TYPE_FIELDS: readonly { key: string; label: string }[] = [
  { key: 'nvme', label: 'NVMe' },
  { key: 'sas', label: 'SAS' },
  { key: 'sata', label: 'SATA' },
];

export interface DiskTemperaturePolicyState {
  /** The agent Disk Temp default trigger; zero or less is off. */
  agentDiskTemperature: number | undefined;
  diskTempByType: Record<string, number>;
}

/** The disk temperature policy's trigger for a disk type; zero or less is off. */
export function resolveDiskTemperatureTriggerForType(
  policy: DiskTemperaturePolicyState,
  diskType: string | null | undefined,
): number {
  const base = policy.agentDiskTemperature ?? FACTORY_AGENT_DEFAULTS.diskTemperature;
  if (base <= 0) return 0;
  const normalizedType = (diskType ?? '').trim().toLowerCase();
  const byType = normalizedType ? policy.diskTempByType[normalizedType] : undefined;
  return typeof byType === 'number' && byType > 0 ? byType : base;
}

/** The trigger a TrueNAS disk without its own override inherits. */
export function resolveTrueNASDiskTemperatureDefault(
  trueNASDiskTemperature: number | undefined,
  policy: DiskTemperaturePolicyState,
  diskType: string | null | undefined,
): number {
  if (trueNASDiskTemperature !== undefined) return trueNASDiskTemperature;
  return resolveDiskTemperatureTriggerForType(policy, diskType);
}

/** "NVMe 70°C", one per disk type, or none when the policy is off. */
export function getDiskTemperatureByTypeItems(policy: DiskTemperaturePolicyState): string[] {
  return DISK_TEMPERATURE_TYPE_FIELDS.flatMap((field) => {
    const trigger = resolveDiskTemperatureTriggerForType(policy, field.key);
    return trigger > 0 ? [`${field.label} ${formatMetricValue('temperature', trigger)}`] : [];
  });
}
