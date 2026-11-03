import { describe, expect, it } from 'vitest';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';
import { EN_MESSAGES } from '@/i18n/messages';

describe('GeneralSettingsPanel guardrails', () => {
  it('routes telemetry disclosure through the shipped privacy doc URL', () => {
    expect(generalSettingsPanelSource).toContain('PRIVACY_DOC_URL');
    expect(generalSettingsPanelSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md',
    );
  });

  it('summarizes telemetry retention and non-storage guarantees in-product', () => {
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'retained for up to 90 days',
    );
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'request IP addresses are used only transiently for rate limiting',
    );
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'not stored in telemetry rows',
    );
  });
});
