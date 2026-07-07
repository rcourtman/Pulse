import { describe, expect, it } from 'vitest';
import relayPairingSectionSource from '../RelayPairingSection.tsx?raw';
import relaySettingsPanelStateSource from '../useRelaySettingsPanelState.ts?raw';

const onboardingSource = Object.values(
  import.meta.glob('../../../api/onboarding.ts', {
    query: '?raw',
    eager: true,
    import: 'default',
  }),
)[0] as string;

const relaySettingsPanelSource = Object.values(
  import.meta.glob('../RelaySettingsPanel.tsx', {
    query: '?raw',
    eager: true,
    import: 'default',
  }),
)[0] as string;

describe('OnboardingAPI', () => {
  it('getQRPayload calls /api/onboarding/qr', () => {
    expect(onboardingSource).toContain("private static baseUrl = '/api/onboarding';");
    expect(onboardingSource).toContain("const url = this.baseUrl + '/qr';");
    expect(onboardingSource).toContain('throw new OnboardingNotReadyError');
  });
});

describe('Onboarding QR payload contract', () => {
  it('defines expected response shape fields', () => {
    expect(onboardingSource).toContain('export interface OnboardingRelayDetails');
    expect(onboardingSource).toContain('enabled: boolean;');
    expect(onboardingSource).toContain('url: string;');

    expect(onboardingSource).toContain('export interface OnboardingDiagnostic');
    expect(onboardingSource).toContain("severity: 'warning' | 'error';");

    expect(onboardingSource).toContain('export interface OnboardingQRResponse');
    expect(onboardingSource).toContain('schema: string;');
    expect(onboardingSource).toContain('instance_url?: string;');
    expect(onboardingSource).toContain('relay: OnboardingRelayDetails;');
    expect(onboardingSource).toContain('auth_token: string;');
    expect(onboardingSource).toContain('deep_link: string;');
    expect(onboardingSource).toContain('diagnostics?: OnboardingDiagnostic[];');
    expect(onboardingSource).toContain('export interface OnboardingNotReadyResponse');
    expect(onboardingSource).toContain('export class OnboardingNotReadyError');
  });

  it('keeps relay settings split into shell, runtime, and pairing owners', () => {
    expect(relaySettingsPanelSource).toContain('./useRelaySettingsPanelState');
    expect(relaySettingsPanelSource).toContain('./RelayPairingSection');
    expect(relaySettingsPanelSource).not.toContain('createSignal(');
    expect(relaySettingsPanelSource).not.toContain('QRCode.toDataURL(');
    expect(relaySettingsPanelStateSource).toContain('QRCode.toDataURL(payload.deep_link');
    expect(relaySettingsPanelStateSource).toContain('setInterval(() => void loadStatus(), 5000)');
    expect(relayPairingSectionSource).toContain('getRelayDiagnosticClass');
  });

  it('points pairing users at the download page for the Pulse Mobile app', () => {
    expect(relayPairingSectionSource).toContain('PULSE_PRO_DOWNLOAD_URL');
    expect(relayPairingSectionSource).toContain('RELAY_PAIRING_APP_AVAILABILITY_TEXT');
  });
});
