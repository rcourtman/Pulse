import {
  isAlertResourceMetricOff,
  normalizeAlertResourceMetricKey,
} from '@/components/Alerts/alertResourceTableModel';
import { formatMetricValue } from '@/features/alerts/thresholds/helpers';

export const ALERT_THRESHOLDS_SECTION_DISABLED_LABEL = 'Disabled';
export const ALERT_THRESHOLDS_SECTION_UNSAVED_CHANGES_TITLE = 'Unsaved changes';

export function getAlertThresholdsSectionDisabledLabel() {
  return ALERT_THRESHOLDS_SECTION_DISABLED_LABEL;
}

export function getAlertThresholdsSectionUnsavedChangesTitle() {
  return ALERT_THRESHOLDS_SECTION_UNSAVED_CHANGES_TITLE;
}

const ALERT_THRESHOLDS_DEFAULTS_SUMMARY_MAX_ITEMS = 4;

type DefaultsSummaryUnit = {
  suffix: string;
  format: (key: string, value: number, label: string) => string;
};

// Unit suffixes as they appear in threshold column labels. The label loses the
// suffix and the value gains it, so "CPU %" at 80 reads "CPU 80%".
const DEFAULTS_SUMMARY_UNITS: DefaultsSummaryUnit[] = [
  { suffix: ' %', format: (key, value, label) => `${label} ${formatPercent(key, value)}` },
  { suffix: ' °C', format: (key, value, label) => `${label} ${formatMetricValue(key, value)}` },
  { suffix: ' MB/s', format: (_key, value, label) => `${label} ${value} MB/s` },
  { suffix: ' (GiB)', format: (key, value, label) => `${label} ${formatMetricValue(key, value)}` },
  { suffix: ' (s)', format: (_key, value, label) => `${label} ${value}s` },
  { suffix: ' (min)', format: (_key, value, label) => `${label} ${value} min` },
  { suffix: ' Days', format: (_key, value, label) => `${label} ${value} ${plural(value, 'day')}` },
  {
    suffix: ' Hours',
    format: (_key, value, label) => `${label} ${value} ${plural(value, 'hour')}`,
  },
];

const DEFAULTS_SUMMARY_LABELS: Record<string, string> = {
  'Disk R': 'Disk read',
  'Disk W': 'Disk write',
  'Net In': 'Net in',
  'Net Out': 'Net out',
  'Disk Temp': 'Disk temp',
};

const formatPercent = (key: string, value: number) => {
  const formatted = formatMetricValue(key, value);
  return formatted.endsWith('%') ? formatted : `${formatted}%`;
};

const plural = (value: number, unit: string) => (value === 1 ? unit : `${unit}s`);

const describeDefault = (column: string, value: number): string => {
  const key = normalizeAlertResourceMetricKey(column);
  const trimmed = column.trim();
  for (const unit of DEFAULTS_SUMMARY_UNITS) {
    if (trimmed.endsWith(unit.suffix)) {
      const base = trimmed.slice(0, -unit.suffix.length);
      return unit.format(key, value, DEFAULTS_SUMMARY_LABELS[base] ?? base);
    }
  }
  return `${DEFAULTS_SUMMARY_LABELS[trimmed] ?? trimmed} ${formatMetricValue(key, value)}`;
};

// One line naming what a threshold group alerts on by default, so the page
// answers "what will alert me" before any group is opened. It reads the same
// columns and defaults the group's table edits. Metrics the engine treats as
// off (isAlertResourceMetricOff, trigger <= 0) are left out. Columns with no
// numeric default, such as the Backup and Snapshot toggles, are skipped.
// `ruleItems` carries rules that are not plain per-metric limits and are
// already described in words (see getDockerContainerRuleSummaryItems).
export function getAlertThresholdsDefaultsSummary(
  columns: readonly string[],
  defaults: Record<string, number | undefined> | undefined,
  options: { ruleItems?: readonly string[]; maxItems?: number } = {},
): string | undefined {
  if (!defaults) return undefined;
  const ruleItems = options.ruleItems ?? [];
  const numericColumns = columns.filter((column) => {
    const value = defaults[normalizeAlertResourceMetricKey(column)];
    return typeof value === 'number' && Number.isFinite(value);
  });
  if (numericColumns.length === 0 && ruleItems.length === 0) return undefined;
  const items = [
    ...numericColumns
      .filter(
        (column) => !isAlertResourceMetricOff(defaults[normalizeAlertResourceMetricKey(column)]),
      )
      .map((column) => describeDefault(column, defaults[normalizeAlertResourceMetricKey(column)]!)),
    ...ruleItems,
  ];
  if (items.length === 0) return 'Defaults: all metric alerts off';
  const maxItems = options.maxItems ?? ALERT_THRESHOLDS_DEFAULTS_SUMMARY_MAX_ITEMS;
  const shown = items.slice(0, maxItems);
  const hidden = items.length - shown.length;
  return `Defaults: ${shown.join(' · ')}${hidden > 0 ? ` · +${hidden} more` : ''}`;
}

// Docker's restart-loop and memory-limit rules always run: config
// normalization (internal/alerts/config/normalize.go, NormalizeDockerDefaults)
// replaces any value <= 0 with these fallbacks. So the summary states the
// effective rule rather than treating zero as Off, and pairs the restart count
// with its window because neither means anything alone.
const DOCKER_RULE_FALLBACKS = {
  restartCount: 3,
  restartWindow: 300,
  memoryWarnPct: 90,
  memoryCriticalPct: 95,
} as const;

const effectiveDockerRule = (value: number | undefined, fallback: number) =>
  typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : fallback;

const formatRestartWindow = (seconds: number) =>
  seconds % 60 === 0 ? `${seconds / 60} min` : `${seconds}s`;

export function getDockerContainerRuleSummaryItems(defaults: {
  restartCount?: number;
  restartWindow?: number;
  memoryWarnPct?: number;
  memoryCriticalPct?: number;
}): string[] {
  const restarts = effectiveDockerRule(defaults.restartCount, DOCKER_RULE_FALLBACKS.restartCount);
  const window = effectiveDockerRule(defaults.restartWindow, DOCKER_RULE_FALLBACKS.restartWindow);
  const warn = effectiveDockerRule(defaults.memoryWarnPct, DOCKER_RULE_FALLBACKS.memoryWarnPct);
  const critical = effectiveDockerRule(
    defaults.memoryCriticalPct,
    DOCKER_RULE_FALLBACKS.memoryCriticalPct,
  );
  return [
    `Restart loop ${restarts} in ${formatRestartWindow(window)}`,
    `Memory limit ${warn}% warn, ${critical}% critical`,
  ];
}
