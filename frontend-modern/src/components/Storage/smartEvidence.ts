import type { SMARTAttributes } from '@/types/api';

export type SmartEvidence = {
  key: keyof SMARTAttributes;
  label: string;
  value: number;
  warning: boolean;
};

const SMART_COUNTERS: Array<{
  key: keyof SMARTAttributes;
  label: string;
  warning: (value: number) => boolean;
}> = [
  { key: 'reallocatedSectors', label: 'Reallocated sectors', warning: (value) => value > 0 },
  { key: 'pendingSectors', label: 'Pending sectors', warning: (value) => value > 0 },
  { key: 'offlineUncorrectable', label: 'Offline uncorrectable', warning: (value) => value > 0 },
  { key: 'udmaCrcErrors', label: 'UDMA CRC errors', warning: (value) => value > 0 },
  { key: 'percentageUsed', label: 'Life used', warning: (value) => value > 90 },
  { key: 'availableSpare', label: 'Available spare', warning: (value) => value < 20 },
  { key: 'mediaErrors', label: 'Media errors', warning: (value) => value > 0 },
];

export function smartEvidence(attrs?: SMARTAttributes): SmartEvidence[] {
  if (!attrs) return [];

  return SMART_COUNTERS.flatMap((counter) => {
    const value = attrs[counter.key];
    if (typeof value !== 'number') return [];
    return [
      {
        key: counter.key,
        label: counter.label,
        value,
        warning: counter.warning(value),
      },
    ];
  });
}

export function smartWarningEvidence(attrs?: SMARTAttributes): SmartEvidence[] {
  return smartEvidence(attrs).filter((entry) => entry.warning);
}

export function hasSmartWarning(attrs?: SMARTAttributes): boolean {
  return smartWarningEvidence(attrs).length > 0;
}

export function smartWarningTitle(attrs?: SMARTAttributes): string {
  const warnings = smartWarningEvidence(attrs);
  if (warnings.length === 0) return 'SMART counters normal';
  return `SMART warning: ${warnings
    .map((entry) => `${entry.label}=${entry.value}`)
    .join(', ')}`;
}
