import { describe, expect, it } from 'vitest';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';

const normalizedGeneralSettingsPanelSource = generalSettingsPanelSource.replace(/\s+/g, ' ');

describe('GeneralSettingsPanel guardrails', () => {
  it('routes telemetry disclosure through the shipped privacy doc URL', () => {
    expect(generalSettingsPanelSource).toContain('PRIVACY_DOC_URL');
    expect(generalSettingsPanelSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md',
    );
  });

  it('keeps telemetry preview and reset controls in the general settings panel', () => {
    expect(generalSettingsPanelSource).toContain('Usage data and privacy');
    expect(generalSettingsPanelSource).toContain('Preview payload');
    expect(generalSettingsPanelSource).toContain('Reset ID');
    expect(generalSettingsPanelSource).toContain('Telemetry payload preview');
  });

  it('routes telemetry command controls through the shared Button primitive', () => {
    expect(generalSettingsPanelSource).toContain(
      "import { Button } from '@/components/shared/Button';",
    );
    expect(generalSettingsPanelSource).toContain('size="settingsActionXs"');
    expect(generalSettingsPanelSource).not.toContain(
      'inline-flex items-center rounded-md border border-border bg-surface px-3 py-2 text-xs font-medium text-base-content transition hover:bg-surface-hover',
    );
  });

  it('keeps maintainer commercial event controls out of customer settings', () => {
    expect(generalSettingsPanelSource).not.toContain('Disable local-only commercial events');
    expect(generalSettingsPanelSource).not.toContain('commercial handoff events');
    expect(generalSettingsPanelSource).not.toContain('PULSE_DISABLE_LOCAL_UPGRADE_METRICS');
  });

  it('summarizes telemetry retention and non-storage guarantees in-product', () => {
    expect(normalizedGeneralSettingsPanelSource).toContain('retained for up to 90 days');
    expect(normalizedGeneralSettingsPanelSource).toContain(
      'IP addresses are not stored in telemetry rows',
    );
  });
});
