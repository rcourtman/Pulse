import type { RawOverrideConfig } from '@/types/alerts';

import type { Override } from './types';

export const upsertOverride = (overrides: Override[], override: Override): Override[] => {
  const existingIndex = overrides.findIndex((entry) => entry.id === override.id);
  if (existingIndex >= 0) {
    const nextOverrides = [...overrides];
    nextOverrides[existingIndex] = override;
    return nextOverrides;
  }

  return [...overrides, override];
};

export const withThresholdEntries = (
  rawConfig: RawOverrideConfig,
  thresholds: Record<string, number | undefined>,
): RawOverrideConfig => {
  const next = { ...rawConfig };

  Object.entries(thresholds).forEach(([metric, value]) => {
    if (value !== undefined && value !== null) {
      next[metric] = {
        clear: Math.max(0, value - 5),
        trigger: value,
      };
    }
  });

  return next;
};

export const stripStateKeys = (
  thresholds: Record<string, number>,
): Record<string, number> => {
  const next = { ...thresholds };
  delete (next as Record<string, unknown>).disabled;
  delete (next as Record<string, unknown>).disableConnectivity;
  delete (next as Record<string, unknown>).poweredOffSeverity;
  return next;
};

export const removeOverrideState = (
  overrides: Override[],
  rawOverridesConfig: Record<string, RawOverrideConfig>,
  resourceId: string,
) => {
  const nextRawConfig = { ...rawOverridesConfig };
  delete nextRawConfig[resourceId];

  return {
    nextOverrides: overrides.filter((override) => override.id !== resourceId),
    nextRawConfig,
  };
};
