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
    expect(generalSettingsPanelSource).toContain('Disable local-only upgrade events');
    expect(generalSettingsPanelSource).toContain('Preview payload');
    expect(generalSettingsPanelSource).toContain('Reset ID');
    expect(generalSettingsPanelSource).toContain('Telemetry payload preview');
  });

  it('summarizes telemetry retention and non-storage guarantees in-product', () => {
    expect(normalizedGeneralSettingsPanelSource).toContain('retained for up to 90 days');
    expect(normalizedGeneralSettingsPanelSource).toContain(
      'IP addresses are not stored in telemetry rows',
    );
  });
});
