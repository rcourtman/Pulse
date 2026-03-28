import { describe, expect, it } from 'vitest';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';

describe('GeneralSettingsPanel guardrails', () => {
  it('routes telemetry disclosure through the shipped privacy doc URL', () => {
    expect(generalSettingsPanelSource).toContain('PRIVACY_DOC_URL');
    expect(generalSettingsPanelSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md',
    );
  });
});
