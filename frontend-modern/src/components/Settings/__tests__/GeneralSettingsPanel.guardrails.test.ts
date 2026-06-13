import { describe, expect, it } from 'vitest';
import generalSettingsPanelSource from '@/components/Settings/GeneralSettingsPanel.tsx?raw';
import dockerRuntimeSettingsCardSource from '@/components/Settings/DockerRuntimeSettingsCard.tsx?raw';
import { EN_MESSAGES } from '@/i18n/messages';

const migratedGeneralSettingsCopy = [
  'Usage data and privacy',
  'Anonymous outbound telemetry',
  'Preview payload',
  'Refresh payload',
  'Reset ID',
  'Copy JSON',
  'Current heartbeat payload',
  'Telemetry payload preview',
  'Monitoring cadence',
  'Current cadence:',
  'Custom polling interval',
  'Managed via environment variable',
];

describe('GeneralSettingsPanel guardrails', () => {
  it('routes telemetry disclosure through the shipped privacy doc URL', () => {
    expect(generalSettingsPanelSource).toContain('PRIVACY_DOC_URL');
    expect(generalSettingsPanelSource).not.toContain(
      'https://github.com/rcourtman/Pulse/blob/main/docs/PRIVACY.md',
    );
  });

  it('keeps telemetry preview and reset controls in the general settings panel', () => {
    expect(generalSettingsPanelSource).toContain('settings.general.telemetry.section.title');
    expect(generalSettingsPanelSource).toContain('settings.general.telemetry.previewPayload');
    expect(generalSettingsPanelSource).toContain('settings.general.telemetry.resetId');
    expect(generalSettingsPanelSource).toContain('settings.general.telemetry.payloadAriaLabel');
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
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'retained for up to 90 days',
    );
    expect(EN_MESSAGES['settings.general.telemetry.description']).toContain(
      'IP addresses are not stored in telemetry rows',
    );
  });

  it('prevents migrated general settings copy from reverting to hardcoded English', () => {
    for (const copy of migratedGeneralSettingsCopy) {
      expect(generalSettingsPanelSource).not.toContain(copy);
    }
    expect(generalSettingsPanelSource).toContain(
      'settings.general.monitoringCadence.section.title',
    );
    expect(generalSettingsPanelSource).toContain('getPvePollingCadenceSummary');
    expect(dockerRuntimeSettingsCardSource).toContain('getDockerUpdateActionsPresentation');
    expect(dockerRuntimeSettingsCardSource).not.toContain('Docker / Podman updates');
    expect(dockerRuntimeSettingsCardSource).not.toContain('Hide update buttons');
  });
});
