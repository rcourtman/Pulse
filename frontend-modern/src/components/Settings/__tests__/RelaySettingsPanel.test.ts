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
    expect(onboardingSource).toContain("apiFetchJSON(this.baseUrl + '/qr')");
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
    expect(onboardingSource).toContain('instance_url: string;');
    expect(onboardingSource).toContain('relay: OnboardingRelayDetails;');
    expect(onboardingSource).toContain('auth_token: string;');
    expect(onboardingSource).toContain('deep_link: string;');
    expect(onboardingSource).toContain('diagnostics?: OnboardingDiagnostic[];');
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
});
