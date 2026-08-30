import { readFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { beforeEach, describe, expect, it } from 'vitest';
import { PRIVACY_DOC_URL } from '@/utils/docsLinks';
import { EN_MESSAGES } from '@/i18n/messages';
import {
  areSystemSettingsLoaded,
  clearRuntimeBranding,
  markSystemSettingsLoadedWithDefaults,
  runtimeBranding,
  shouldHideDockerUpdateActions,
  shouldReduceProUpsellNoise,
  updateRuntimeBrandingFromResponse,
  updateRuntimeDisplayFromResponse,
} from '@/stores/systemSettings';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const frontendRoot = path.resolve(__dirname, '..', '..', '..');
const repoRoot = path.resolve(frontendRoot, '..');

describe('systemSettings store', () => {
  beforeEach(() => {
    markSystemSettingsLoadedWithDefaults();
    clearRuntimeBranding();
  });

  it('applies route and docker feature flags from API response', () => {
    updateRuntimeDisplayFromResponse({
      theme: 'dark',
      disableDockerUpdateActions: true,
      reduceProUpsellNoise: true,
    });

    expect(shouldHideDockerUpdateActions()).toBe(true);
    expect(shouldReduceProUpsellNoise()).toBe(true);
  });

  it('resets route and docker feature flags to safe defaults', () => {
    updateRuntimeDisplayFromResponse({
      theme: 'dark',
      disableDockerUpdateActions: true,
      reduceProUpsellNoise: true,
    });

    markSystemSettingsLoadedWithDefaults();
    expect(shouldHideDockerUpdateActions()).toBe(false);
    expect(shouldReduceProUpsellNoise()).toBe(false);
  });

  it('keeps privacy and local paid-flow defaults safe when flags are omitted', () => {
    updateRuntimeDisplayFromResponse({
      theme: 'dark',
    });

    expect(areSystemSettingsLoaded()).toBe(true);
    expect(shouldHideDockerUpdateActions()).toBe(false);
    expect(shouldReduceProUpsellNoise()).toBe(false);
  });

  it('normalizes the entitlement-filtered runtime branding payload', () => {
    updateRuntimeBrandingFromResponse({
      enabled: true,
      displayName: '  Acme Operations  ',
      logoDataUrl: 'data:image/png;base64,YWJj',
    });

    expect(runtimeBranding()).toEqual({
      enabled: true,
      displayName: 'Acme Operations',
      logoDataUrl: 'data:image/png;base64,YWJj',
    });

    clearRuntimeBranding();
    expect(runtimeBranding()).toEqual({ enabled: false, displayName: '', logoDataUrl: '' });
  });

  it('drops branding values from a disabled runtime response', () => {
    updateRuntimeBrandingFromResponse({
      enabled: false,
      displayName: 'Must not render',
      logoDataUrl: 'data:image/png;base64,YWJj',
    });

    expect(runtimeBranding()).toEqual({ enabled: false, displayName: '', logoDataUrl: '' });
  });

  it('keeps the telemetry disclosure on the shipped local privacy doc', () => {
    expect(PRIVACY_DOC_URL).toBe('/docs/PRIVACY');
  });

  it('keeps the telemetry disclosure in user-facing product language', () => {
    const telemetryDescription = EN_MESSAGES['settings.general.telemetry.description'];

    expect(telemetryDescription).toContain('coarse deployment and lifecycle buckets');
    expect(telemetryDescription).toContain('aggregate resource and outcome counts');
    expect(telemetryDescription).toContain(
      'content-free Patrol, Assistant, and capability-API usage counters',
    );
    expect(telemetryDescription).toContain('URLs, paths, locale, raw browser events');
    expect(telemetryDescription).not.toContain('Pulse Intelligence loop adoption');
    expect(telemetryDescription).not.toContain('activation loop');
    expect(telemetryDescription).not.toContain('operations loop');
  });

  it('documents telemetry retention and field-level rationale in the privacy doc', () => {
    const privacyDoc = readFileSync(path.join(repoRoot, 'docs', 'PRIVACY.md'), 'utf8');
    const shippedPrivacyDoc = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'PRIVACY.md'),
      'utf8',
    );

    expect(privacyDoc).toContain('## Usage Data');
    expect(privacyDoc).toContain('Pulse has one outbound usage-data scope');
    expect(privacyDoc).toContain('Commercial activation and license-recovery runtime records');
    expect(privacyDoc).not.toContain('Local-only commercial handoff events');
    expect(privacyDoc).not.toContain('Disable local-only commercial events');
    expect(privacyDoc).not.toContain('reduceProUpsellNoise');
    expect(privacyDoc).toContain('Every field is listed below with the reason it exists');
    expect(privacyDoc).toContain('rows older than **90 days** are purged automatically');
    expect(privacyDoc).toContain('uses request IP addresses transiently for abuse/rate limiting');
    expect(privacyDoc).toContain('Reset ID');
    expect(privacyDoc).toContain('active Patrol objective briefs');
    expect(privacyDoc).toContain('encrypted at rest in the local organization data directory');
    expect(privacyDoc).toContain('objective text is not included in Pulse usage telemetry');
    expect(privacyDoc).toContain('Pulse Intelligence Patrol control completed operations loop 30d');
    expect(privacyDoc).toContain(
      'Pulse Intelligence Patrol control paid resolved operations loop 30d',
    );
    expect(privacyDoc).not.toContain('completed-work proof');
    expect(privacyDoc).not.toContain('resolved-work proof');
    expect(privacyDoc).not.toContain('governed-operation proof');
    expect(shippedPrivacyDoc).toBe(privacyDoc);
  });

  it('keeps internal commercial compatibility switches out of public configuration docs', () => {
    const configurationDoc = readFileSync(path.join(repoRoot, 'docs', 'CONFIGURATION.md'), 'utf8');

    expect(configurationDoc).not.toContain('reduceProUpsellNoise');
    expect(configurationDoc).not.toContain('disableLocalUpgradeMetrics');
    expect(configurationDoc).not.toContain('PULSE_DISABLE_LOCAL_UPGRADE_METRICS');
    expect(configurationDoc).toContain('PULSE_TELEMETRY');
  });

  it('keeps Relay security guidance aligned with the Relay tier boundary', () => {
    const securityDoc = readFileSync(path.join(repoRoot, 'SECURITY.md'), 'utf8');
    const publicSecurityDoc = readFileSync(
      path.join(frontendRoot, 'public', 'docs', 'SECURITY.md'),
      'utf8',
    );

    for (const copy of [securityDoc, publicSecurityDoc]) {
      expect(copy).toContain('Relay Security (Relay and Above)');
      expect(copy).toContain(
        'Relay functionality requires a Relay, Pro, legacy Pro+, or Cloud license',
      );
      expect(copy).not.toContain('Relay Security (Pro)');
      expect(copy).not.toContain('Relay functionality requires a Pro or Cloud license');
    }
  });

  it('documents self-hosted AI provider transport and resource-policy redaction in the privacy doc', () => {
    const privacyDoc = readFileSync(path.join(repoRoot, 'docs', 'PRIVACY.md'), 'utf8');

    expect(privacyDoc).toContain(
      'AI prompts from self-managed installs do not transit Pulse infrastructure',
    );
    expect(privacyDoc).toContain(
      'governed resource details use the same resource-policy redaction',
    );
    expect(privacyDoc).toContain('Local providers stay on your network');
  });
});
