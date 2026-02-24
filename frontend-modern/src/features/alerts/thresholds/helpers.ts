import { formatTemperature } from '@/utils/temperature';
import type { PMGThresholdDefaults } from '@/types/alerts';

export const normalizeThresholdLabel = (label: string): string =>
  label
    .trim()
    .toLowerCase()
    .replace(' %', '')
    .replace(' °c', '')
    .replace(' mb/s', '')
    .replace('disk r', 'diskRead')
    .replace('disk w', 'diskWrite')
    .replace('net in', 'networkIn')
    .replace('net out', 'networkOut')
    .replace('disk temp', 'diskTemperature');

export const pmgColumn = (key: keyof PMGThresholdDefaults, label: string) => ({
  key,
  label,
  normalized: normalizeThresholdLabel(label),
});

export const normalizeDockerIgnoredInput = (value: string): string[] =>
  value
    .split('\n')
    .map((entry) => entry.trim())
    .filter((entry) => entry.length > 0);

export const formatMetricValue = (metric: string, value: number | undefined): string => {
  if (value === undefined || value === null) return '0';

  // Show "Off" for disabled thresholds (0 or negative values)
  if (value <= 0) return 'Off';

  // Percentage-based metrics
  if (
    metric === 'cpu' ||
    metric === 'memory' ||
    metric === 'disk' ||
    metric === 'usage' ||
    metric === 'memoryWarnPct' ||
    metric === 'memoryCriticalPct'
  ) {
    return `${value}%`;
  }

  // Temperature
  if (metric === 'temperature' || metric === 'diskTemperature') {
    return formatTemperature(value);
  }

  if (metric === 'restartWindow') {
    return `${value}s`;
  }

  if (metric === 'restartCount') {
    return String(value);
  }

  if (metric === 'warningSizeGiB' || metric === 'criticalSizeGiB') {
    const rounded = Math.round(value * 10) / 10;
    return `${rounded} GiB`;
  }

  // MB/s metrics
  if (
    metric === 'diskRead' ||
    metric === 'diskWrite' ||
    metric === 'networkIn' ||
    metric === 'networkOut'
  ) {
    return `${value} MB/s`;
  }

  return String(value);
};
